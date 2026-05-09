package vm

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/byteme/compiler/code"
	"github.com/byteme/compiler/compiler"
	"github.com/byteme/compiler/object"
)

const StackSize = 8192
const MaxFrames = 2048
const GlobalsSize = 65536
var ErrYield = fmt.Errorf("yield")

type VM struct {
	constants []object.Object
	globals   []object.Object

	stack []object.Object
	sp    int // Stack pointer

	frames      []*Frame
	framesIndex int

	catchHandlers []*CatchHandler
	structs       map[string]*object.StructLiteral
	YieldedValue  object.Object
}

type CatchHandler struct {
	ip          int
	sp          int
	framesIndex int
}

// httpBuiltinStart holds the index where HTTP builtins begin in the Builtins slice.
var httpBuiltinStart int

func init() {
	httpBuiltinStart = object.RegisterHTTPBuiltins()
	object.GeneratorNext = func(g *object.Generator) (object.Object, bool) {
		if v, ok := g.VMState.(*VM); ok {
			return v.Next()
		}
		return object.NULL, false
	}
	object.MakeGenerator = func(fn *object.Closure, args []object.Object) object.Object {
		consts, globs := object.GetVMContext()
		child := NewWithGlobalStore(*consts, *globs)
		
		child.push(fn)
		for _, arg := range args {
			child.push(arg)
		}

		frame := NewFrame(fn, child.sp-len(args))
		child.pushFrame(frame)
		child.sp = frame.basePointer + fn.Fn.NumLocals
		
		return &object.Generator{VMState: child}
	}
}

func New(bytecode *compiler.Bytecode) *VM {
	mainFn := &object.CompiledFunction{
		Instructions: bytecode.Instructions,
		SourceMap:    bytecode.SourceMap,
		Filename:     bytecode.Filename,
		Name:         "<main>",
		NumLocals:    bytecode.NumLocals,
	}
	mainClosure := &object.Closure{Fn: mainFn}
	mainFrame := NewFrame(mainClosure, 0)
	frames := make([]*Frame, MaxFrames)
	frames[0] = mainFrame

	vm := &VM{
		constants:     bytecode.Constants,
		globals:       make([]object.Object, GlobalsSize),
		stack:         make([]object.Object, StackSize),
		sp:            0, // Start sp at 0, main frame will handle its locals
		frames:        frames,
		framesIndex:   1,
		catchHandlers: []*CatchHandler{},
		structs:       make(map[string]*object.StructLiteral),
	}
	vm.registerRunner()
	return vm
}

func NewWithGlobalStore(constants []object.Object, globals []object.Object) *VM {
	vm := &VM{
		constants:     constants,
		globals:       globals,
		stack:         make([]object.Object, StackSize),
		sp:            0,
		frames:        make([]*Frame, MaxFrames),
		framesIndex:   0,
		catchHandlers: []*CatchHandler{},
		structs:       make(map[string]*object.StructLiteral),
	}
	return vm
}

// registerRunner sets the package-level object.RunFunction so that HTTP
// (and other) builtins can call back into ByteMe compiled functions.
func (vm *VM) registerRunner() {
	object.RunFunction = func(closure *object.Closure, constants []object.Object, globals []object.Object, args []object.Object) object.Object {
		child := NewWithGlobalStore(constants, globals)

		// Push the function (closure) onto the stack first
		child.push(closure)
		
		// Push arguments onto the stack
		for _, arg := range args {
			child.push(arg)
		}

		frame := NewFrame(closure, child.sp-len(args))
		child.pushFrame(frame)
		child.sp = frame.basePointer + closure.Fn.NumLocals

		if err := child.Run(); err != nil {
			return &object.Error{Message: err.Error()}
		}
		return child.StackTop()
	}

	object.SetVMContext(&vm.constants, &vm.globals)
}

func (vm *VM) currentFrame() *Frame {
	return vm.frames[vm.framesIndex-1]
}

func (vm *VM) pushFrame(f *Frame) {
	vm.frames[vm.framesIndex] = f
	vm.framesIndex++
}

func (vm *VM) popFrame() *Frame {
	vm.framesIndex--
	return vm.frames[vm.framesIndex]
}

func (vm *VM) LastPoppedStackElem() object.Object {
	return vm.stack[vm.sp]
}

func (vm *VM) StackTop() object.Object {
	if vm.sp == 0 {
		return object.NULL
	}
	return vm.stack[vm.sp-1]
}

func (vm *VM) Run() error {
	var ip int
	var ins []byte
	var op code.Opcode

	for vm.framesIndex > 0 && vm.currentFrame().ip < len(vm.currentFrame().Instructions())-1 {
		vm.currentFrame().ip++

		ip = vm.currentFrame().ip
		ins = vm.currentFrame().Instructions()
		op = code.Opcode(ins[ip])

		switch op {
		case code.OpConstant:
			constIndex := binary.BigEndian.Uint16(ins[ip+1:])
			vm.currentFrame().ip += 2
			err := vm.push(vm.constants[constIndex])
			if err != nil { return err }

		case code.OpAdd, code.OpSub, code.OpMul, code.OpDiv, code.OpMod,
			code.OpBitAnd, code.OpBitOr, code.OpBitXor, code.OpLShift, code.OpRShift:
			err := vm.executeBinaryArithmetic(op)
			if err != nil { return err }

		case code.OpPop:
			vm.pop()

		case code.OpTrue:
			err := vm.push(object.TRUE)
			if err != nil { return err }
		case code.OpFalse:
			err := vm.push(object.FALSE)
			if err != nil { return err }

		case code.OpEqual, code.OpNotEqual, code.OpGreaterThan:
			err := vm.executeComparison(op)
			if err != nil { return err }

		case code.OpMinus:
			err := vm.executePrefixMinus()
			if err != nil { return err }

		case code.OpClosure:
			constIndex := binary.BigEndian.Uint16(ins[ip+1:])
			numFree := int(ins[ip+3])
			vm.currentFrame().ip += 3

			err := vm.pushClosure(int(constIndex), numFree)
			if err != nil { return err }

		case code.OpGetFree:
			freeIndex := int(ins[ip+1])
			vm.currentFrame().ip += 1

			err := vm.push(vm.currentFrame().cl.Free[freeIndex])
			if err != nil { return err }

		case code.OpSetFree:
			freeIndex := int(ins[ip+1])
			vm.currentFrame().ip += 1
			vm.currentFrame().cl.Free[freeIndex] = vm.pop()

		case code.OpBang:
			err := vm.executeBangOperator()
			if err != nil { return err }

		case code.OpBitNot:
			err := vm.executePrefixBitNot()
			if err != nil { return err }

		case code.OpJump:
			pos := int(binary.BigEndian.Uint16(ins[ip+1:]))
			vm.currentFrame().ip = pos - 1

		case code.OpJumpNotTruthy:
			pos := int(binary.BigEndian.Uint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			condition := vm.pop()
			if !isTruthy(condition) {
				vm.currentFrame().ip = pos - 1
			}

		case code.OpSetGlobal:
			globalIndex := binary.BigEndian.Uint16(ins[ip+1:])
			vm.currentFrame().ip += 2
			vm.globals[globalIndex] = vm.pop()

		case code.OpGetGlobal:
			globalIndex := binary.BigEndian.Uint16(ins[ip+1:])
			vm.currentFrame().ip += 2
			err := vm.push(vm.globals[globalIndex])
			if err != nil { return err }

		case code.OpSetLocal:
			localIndex := int(ins[ip+1])
			vm.currentFrame().ip += 1
			frame := vm.currentFrame()
			vm.stack[frame.basePointer+localIndex] = vm.pop()

		case code.OpGetLocal:
			localIndex := int(ins[ip+1])
			vm.currentFrame().ip += 1
			frame := vm.currentFrame()
			err := vm.push(vm.stack[frame.basePointer+localIndex])
			if err != nil { return err }

		case code.OpGetBuiltin:
			builtinIndex := ins[ip+1]
			vm.currentFrame().ip += 1
			definition := object.Builtins[builtinIndex]
			err := vm.push(definition)
			if err != nil { return err }

		case code.OpCall:
			numArgs := int(ins[ip+1])
			vm.currentFrame().ip += 1

			fn := vm.stack[vm.sp-1-numArgs]
			err := vm.executeCall(fn, numArgs)
			if err != nil { return err }

		case code.OpReturnValue:
			returnValue := vm.pop()
			frame := vm.popFrame()
			vm.sp = frame.basePointer - 1
			err := vm.push(returnValue)
			if err != nil { return err }

		case code.OpReturn:
			frame := vm.popFrame()
			vm.sp = frame.basePointer - 1
			err := vm.push(object.NULL)
			if err != nil { return err }

		case code.OpSpawn:
			numArgs := int(ins[ip+1])
			vm.currentFrame().ip += 1

			fnObj := vm.stack[vm.sp-1-numArgs]
			future := &object.Future{ValueChan: make(chan object.Object, 1)}

			switch f := fnObj.(type) {
			case *object.Builtin:
				args := make([]object.Object, numArgs)
				copy(args, vm.stack[vm.sp-numArgs:vm.sp])
				vm.sp = vm.sp - numArgs - 1
				go func() {
					res := f.Fn(args...)
					future.ValueChan <- res
				}()
				err := vm.push(future)
				if err != nil { return err }

			case *object.Closure, *object.CompiledFunction:
				var cl *object.Closure
				var fn *object.CompiledFunction
				if c, ok := f.(*object.Closure); ok {
					cl = c
					fn = c.Fn
				} else {
					fn = f.(*object.CompiledFunction)
					cl = &object.Closure{Fn: fn}
				}

				if numArgs != fn.NumParameters {
					return fmt.Errorf("wrong number of arguments for spawn: want=%d, got=%d", fn.NumParameters, numArgs)
				}

				// Spawn a new VM!
				newVM := NewWithGlobalStore(vm.constants, vm.globals)
				
				// Push the closure onto the stack at index 0 (so that OpReturnValue can find it at basePointer-1)
				err := newVM.push(cl)
				if err != nil { return err }

				// Push arguments onto the new VM's stack (starting at index 1)
				for i := vm.sp - numArgs; i < vm.sp; i++ {
					err := newVM.push(vm.stack[i])
					if err != nil { return err }
				}

				vm.sp = vm.sp - numArgs - 1 // Pop args and function from original stack
				
				go func() {
					frame := NewFrame(cl, newVM.sp-numArgs)
					newVM.pushFrame(frame)
					newVM.sp = frame.basePointer + fn.NumLocals
					err := newVM.Run()
					if err != nil {
						future.ValueChan <- &object.Error{Message: err.Error()}
					} else {
						future.ValueChan <- newVM.StackTop()
					}
				}()
				
				err = vm.push(future)
				if err != nil { return err }

			default:
				return fmt.Errorf("spawning non-function: %T", fnObj)
			}

		case code.OpArray:
			numElements := int(binary.BigEndian.Uint16(ins[ip+1:]))
			vm.currentFrame().ip += 2

			elements := make([]object.Object, numElements)
			for i := 0; i < numElements; i++ {
				elements[i] = vm.stack[vm.sp-numElements+i]
			}
			vm.sp = vm.sp - numElements

			err := vm.push(&object.Array{Elements: elements})
			if err != nil { return err }

		case code.OpNull:
			err := vm.push(object.NULL)
			if err != nil { return err }

		case code.OpAwait:
			obj := vm.pop()
			if future, ok := obj.(*object.Future); ok {
				result := future.Get()
				err := vm.push(result)
				if err != nil { return err }
			} else {
				// If it's not a future, just return it (like JS await)
				err := vm.push(obj)
				if err != nil { return err }
			}

		// ── Struct opcodes ────────────────────────────────────────────────
		case code.OpStructDef:
			// Push the StructLiteral (type definition) from the constant pool
			constIdx := int(binary.BigEndian.Uint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			structLit := vm.constants[constIdx].(*object.StructLiteral)
			
			// Link parent if exists
			if structLit.ParentName != "" {
				if parent, ok := vm.structs[structLit.ParentName]; ok {
					structLit.Parent = parent
				}
			}
			
			vm.structs[structLit.Name] = structLit
			err := vm.push(structLit)
			if err != nil { return err }

		case code.OpGetField:
			// Stack: [..., instance]  → push instance.field
			constIdx := int(binary.BigEndian.Uint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			fieldName := vm.constants[constIdx].(*object.String).Value
			instance := vm.pop()
			switch inst := instance.(type) {
			case *object.StructInstance:
				// Search fields in hierarchy
				var val object.Object
				var ok bool
				
				curr := inst
				for curr != nil {
					val, ok = curr.Fields[fieldName]
					if ok { break }
					// To check parent fields, we need to know the parent instance's layout?
					// No, StructInstance currently stores ALL fields in a flat map.
					// Wait, if it's flat, then inst.Fields[fieldName] should just work.
					break
				}
				
				if ok {
					err := vm.push(val)
					if err != nil { return err }
				} else {
					// Check methods recursively
					var method *object.CompiledFunction
					currDef := inst.Definition
					for currDef != nil {
						method, ok = currDef.Methods[fieldName]
						if ok { break }
						currDef = currDef.Parent
					}
					
					if ok {
						err := vm.push(&object.BoundMethod{Receiver: instance, Method: method})
						if err != nil { return err }
					} else {
						err := vm.push(object.NULL)
						if err != nil { return err }
					}
				}
			case *object.Map:
				err := vm.pushMapField(inst, fieldName)
				if err != nil { return err }
			case *object.Namespace:
				val, ok := inst.Env.GetVal(fieldName)
				if ok {
					err := vm.push(val.(object.Object))
					if err != nil { return err }
				} else {
					err := vm.push(object.NULL)
					if err != nil { return err }
				}
			default:
				return fmt.Errorf("cannot access field '%s' on %s", fieldName, instance.Type())
			}

		case code.OpSetField:
			// Stack: [..., instance, value]  → instance.field = value
			constIdx := int(binary.BigEndian.Uint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			fieldName := vm.constants[constIdx].(*object.String).Value
			value := vm.pop()
			instance := vm.pop()
			switch inst := instance.(type) {
			case *object.StructInstance:
				inst.Fields[fieldName] = value
				err := vm.push(instance)
				if err != nil { return err }
			case *object.Map:
				inst.Pairs[fieldName] = object.MapPair{Key: &object.String{Value: fieldName}, Value: value}
				err := vm.push(instance)
				if err != nil { return err }
			default:
				return fmt.Errorf("cannot set field '%s' on %s", fieldName, instance.Type())
			}

		case code.OpThrow:
			errValue := vm.pop()
			if len(vm.catchHandlers) == 0 {
				return fmt.Errorf("uncaught error: %s", errValue.Inspect())
			}
			// Find the nearest handler
			handler := vm.catchHandlers[len(vm.catchHandlers)-1]
			vm.catchHandlers = vm.catchHandlers[:len(vm.catchHandlers)-1]

			// Restore stack and frame state
			vm.sp = handler.sp
			vm.framesIndex = handler.framesIndex
			vm.currentFrame().ip = handler.ip - 1
			err := vm.push(errValue)
			if err != nil { return err }

		case code.OpTry:
			jumpToCatch := int(binary.BigEndian.Uint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			vm.catchHandlers = append(vm.catchHandlers, &CatchHandler{
				ip:          jumpToCatch,
				sp:          vm.sp,
				framesIndex: vm.framesIndex,
			})

		case code.OpEndTry:
			if len(vm.catchHandlers) > 0 {
				vm.catchHandlers = vm.catchHandlers[:len(vm.catchHandlers)-1]
			}

		case code.OpIndex:
			index := vm.pop()
			left := vm.pop()
			err := vm.executeIndexExpression(left, index)
			if err != nil { return err }

		case code.OpSetIndex:
			value := vm.pop()
			index := vm.pop()
			left := vm.pop()
			err := vm.executeSetIndexExpression(left, index, value)
			if err != nil { return err }

		case code.OpIterInit:
			obj := vm.pop()
			switch o := obj.(type) {
			case *object.Array:
				vm.push(&object.ArrayIterator{Array: o, Index: 0})
			case *object.Map:
				keys := make([]string, 0, len(o.Pairs))
				vals := make([]object.Object, 0, len(o.Pairs))
				for k, p := range o.Pairs {
					keys = append(keys, k)
					vals = append(vals, p.Value)
				}
				vm.push(&object.MapIterator{Keys: keys, Values: vals, Index: 0})
			case *object.String:
				// Treat string as array of chars
				elements := make([]object.Object, len(o.Value))
				for i, r := range o.Value {
					elements[i] = &object.String{Value: string(r)}
				}
				vm.push(&object.ArrayIterator{Array: &object.Array{Elements: elements}, Index: 0})
			default:
				if obj.Type() == object.ITERATOR_OBJ {
					vm.push(obj)
					break
				}
				return fmt.Errorf("cannot iterate over %s", obj.Type())
			}

		case code.OpIterNext:
			iterObj := vm.pop()
			var val object.Object
			var hasNext bool

			switch it := iterObj.(type) {
			case *object.ArrayIterator:
				val, hasNext = it.Next()
			case *object.MapIterator:
				val, hasNext = it.Next()
			case *object.Generator:
				val, hasNext = it.Next()
			default:
				return fmt.Errorf("not an iterator: %T", iterObj)
			}

			if hasNext {
				vm.push(val)
				vm.push(object.TRUE)
			} else {
				vm.push(object.NULL)
				vm.push(object.FALSE)
			}

		case code.OpYield:
			vm.YieldedValue = vm.pop()
			return ErrYield

		case code.OpMap:
			numElements := int(binary.BigEndian.Uint16(ins[ip+1:]))
			vm.currentFrame().ip += 2

			pairs := make(map[string]object.MapPair)
			for i := 0; i < numElements; i++ {
				value := vm.pop()
				key := vm.pop()
				
				sKey, ok := key.(*object.String)
				if !ok {
					return fmt.Errorf("map keys must be strings, got %s", key.Type())
				}
				pairs[sKey.Value] = object.MapPair{Key: sKey, Value: value}
			}
			vm.push(&object.Map{Pairs: pairs})

		case code.OpDup:
			vm.push(vm.stack[vm.sp-1])
		case code.OpDup2:
			vm.push(vm.stack[vm.sp-2])
			vm.push(vm.stack[vm.sp-2])
		case code.OpSwap:
			vm.stack[vm.sp-1], vm.stack[vm.sp-2] = vm.stack[vm.sp-2], vm.stack[vm.sp-1]
		case code.OpRot:
			// A, B, C -> C, A, B
			a, b, c := vm.stack[vm.sp-3], vm.stack[vm.sp-2], vm.stack[vm.sp-1]
			vm.stack[vm.sp-3], vm.stack[vm.sp-2], vm.stack[vm.sp-1] = c, a, b
		case code.OpPick:
			n := int(ins[ip+1])
			vm.currentFrame().ip += 1
			vm.push(vm.stack[vm.sp-1-n])
		}
	}
	return nil
}

func (vm *VM) Next() (object.Object, bool) {
	err := vm.Run()
	if err == ErrYield {
		return vm.YieldedValue, true
	}
	return object.NULL, false
}

func (vm *VM) Trace() string {
	var out bytes.Buffer
	for i := vm.framesIndex - 1; i >= 0; i-- {
		frame := vm.frames[i]
		ip := frame.ip
		fn := frame.cl.Fn
		line := 0
		if fn.SourceMap != nil {
			// Find the nearest line number (largest offset <= ip)
			bestOffset := -1
			for offset, l := range fn.SourceMap {
				if offset <= ip && offset > bestOffset {
					bestOffset = offset
					line = l
				}
			}
		}
		name := fn.Name
		if name == "" { name = "<anonymous>" }
		filename := fn.Filename
		if filename == "" { filename = "<unknown>" }
		out.WriteString(fmt.Sprintf("  at %s (%s:%d)\n", name, filename, line))
	}
	return out.String()
}

func (vm *VM) pushMapField(inst *object.Map, fieldName string) error {
	pair, ok := inst.Pairs[fieldName]
	if !ok {
		return vm.push(object.NULL)
	}
	return vm.push(pair.Value)
}

func (vm *VM) executeIndexExpression(left, index object.Object) error {
	switch obj := left.(type) {
	case *object.Array:
		i, ok := index.(*object.Integer)
		if !ok {
			return fmt.Errorf("index must be an integer, got %s", index.Type())
		}
		idx := i.Value
		if idx < 0 || idx >= int64(len(obj.Elements)) {
			return vm.push(object.NULL)
		}
		return vm.push(obj.Elements[idx])
	case *object.Map:
		key := index.Inspect()
		pair, ok := obj.Pairs[key]
		if !ok {
			return vm.push(object.NULL)
		}
		return vm.push(pair.Value)
	case *object.String:
		i, ok := index.(*object.Integer)
		if !ok {
			return fmt.Errorf("index must be an integer, got %s", index.Type())
		}
		idx := i.Value
		if idx < 0 || idx >= int64(len(obj.Value)) {
			return vm.push(object.NULL)
		}
		return vm.push(&object.String{Value: string(obj.Value[idx])})
	default:
		return fmt.Errorf("index operator not supported: %s", left.Type())
	}
}

func (vm *VM) executeSetIndexExpression(left, index, value object.Object) error {
	switch obj := left.(type) {
	case *object.Array:
		i, ok := index.(*object.Integer)
		if !ok {
			return fmt.Errorf("index must be an integer, got %s", index.Type())
		}
		idx := i.Value
		if idx < 0 || idx >= int64(len(obj.Elements)) {
			return fmt.Errorf("index out of range: %d", idx)
		}
		obj.Elements[idx] = value
		return vm.push(obj)
	case *object.Map:
		key := index.Inspect()
		obj.Pairs[key] = object.MapPair{Key: index, Value: value}
		return vm.push(obj)
	default:
		return fmt.Errorf("index assignment not supported: %s", left.Type())
	}
}

func (vm *VM) executeCall(fn object.Object, numArgs int) error {
	switch callee := fn.(type) {
	case *object.Closure:
		return vm.callClosure(callee, numArgs)
	case *object.CompiledFunction:
		return vm.callClosure(&object.Closure{Fn: callee}, numArgs)
	case *object.Builtin:
		return vm.callBuiltin(callee, numArgs)
	case *object.StructLiteral:
		// Struct constructor: Point(x, y) → StructInstance{x: ..., y: ...}
		allFields := callee.GetAllFields()
		if numArgs > len(allFields) {
			return fmt.Errorf("struct %s requires %d fields, got %d", callee.Name, len(allFields), numArgs)
		}
		instance := &object.StructInstance{
			Definition: callee,
			Fields:     make(map[string]object.Object),
		}
		for i, field := range allFields {
			if i < numArgs {
				instance.Fields[field.Name.Value] = vm.stack[vm.sp-numArgs+i]
			} else {
				instance.Fields[field.Name.Value] = object.NULL
			}
		}
		vm.sp = vm.sp - numArgs - 1
		return vm.push(instance)
	case *object.EnumConstructor:
		// Enum constructor: Shape.Circle(1.5) -> EnumInstance
		variantDef := callee.Definition.Variants[callee.Variant]
		if numArgs != len(variantDef.Types) {
			return fmt.Errorf("enum variant %s.%s requires %d values, got %d", callee.Definition.Name, callee.Variant, len(variantDef.Types), numArgs)
		}
		instance := &object.EnumInstance{
			Definition: callee.Definition,
			Variant:    callee.Variant,
			Values:     make([]object.Object, numArgs),
		}
		for i := 0; i < numArgs; i++ {
			instance.Values[i] = vm.stack[vm.sp-numArgs+i]
		}
		vm.sp = vm.sp - numArgs - 1
		return vm.push(instance)
	case *object.BoundMethod:
		if numArgs != callee.Method.NumParameters-1 {
			return fmt.Errorf("wrong number of arguments for method: want=%d, got=%d", callee.Method.NumParameters-1, numArgs)
		}

		if vm.sp >= StackSize {
			return fmt.Errorf("stack overflow")
		}

		// Shift arguments up to make room for receiver
		for i := vm.sp; i > vm.sp-numArgs; i-- {
			vm.stack[i] = vm.stack[i-1]
		}
		vm.stack[vm.sp-numArgs] = callee.Receiver
		vm.sp++

		frame := NewFrame(&object.Closure{Fn: callee.Method}, vm.sp-numArgs-1)
		vm.pushFrame(frame)
		vm.sp = frame.basePointer + callee.Method.NumLocals
		return nil
	default:
		return fmt.Errorf("calling non-function and non-built-in: %T", fn)
	}
}

func (vm *VM) callClosure(cl *object.Closure, numArgs int) error {
	if numArgs != cl.Fn.NumParameters {
		return fmt.Errorf("wrong number of arguments: want=%d, got=%d",
			cl.Fn.NumParameters, numArgs)
	}

	frame := NewFrame(cl, vm.sp-numArgs)
	vm.pushFrame(frame)

	vm.sp = frame.basePointer + cl.Fn.NumLocals

	return nil
}

func (vm *VM) callBuiltin(builtin *object.Builtin, numArgs int) error {
	args := vm.stack[vm.sp-numArgs : vm.sp]
	result := builtin.Fn(args...)
	vm.sp = vm.sp - numArgs - 1
	if result != nil {
		vm.push(result)
	} else {
		vm.push(object.NULL)
	}
	return nil
}

func (vm *VM) pushClosure(constIndex int, numFree int) error {
	constant := vm.constants[constIndex]
	function, ok := constant.(*object.CompiledFunction)
	if !ok {
		return fmt.Errorf("not a function: %+v", constant)
	}

	free := make([]object.Object, numFree)
	for i := 0; i < numFree; i++ {
		free[i] = vm.stack[vm.sp-numFree+i]
	}
	vm.sp -= numFree

	closure := &object.Closure{Fn: function, Free: free}
	return vm.push(closure)
}

func isTruthy(obj object.Object) bool {
	switch obj {
	case object.FALSE, object.NULL:
		return false
	default:
		return true
	}
}

func (vm *VM) executeBinaryArithmetic(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()

	if left.Type() == object.STRING_OBJ || right.Type() == object.STRING_OBJ {
		if op != code.OpAdd {
			return fmt.Errorf("unknown operator %d for strings", op)
		}
		var leftVal, rightVal string
		if left.Type() == object.STRING_OBJ {
			leftVal = left.(*object.String).Value
		} else if left.Type() == object.INTEGER_OBJ {
			leftVal = fmt.Sprintf("%d", left.(*object.Integer).Value)
		} else {
			leftVal = left.Inspect()
		}

		if right.Type() == object.STRING_OBJ {
			rightVal = right.(*object.String).Value
		} else if right.Type() == object.INTEGER_OBJ {
			rightVal = fmt.Sprintf("%d", right.(*object.Integer).Value)
		} else {
			rightVal = right.Inspect()
		}

		return vm.push(&object.String{Value: leftVal + rightVal})
	}

	if left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ {
		leftValue := left.(*object.Integer).Value
		rightValue := right.(*object.Integer).Value
		var result int64
		switch op {
		case code.OpAdd:
			result = leftValue + rightValue
		case code.OpSub:
			result = leftValue - rightValue
		case code.OpMul:
			result = leftValue * rightValue
		case code.OpDiv:
			result = leftValue / rightValue
		case code.OpMod:
			result = leftValue % rightValue
		case code.OpBitAnd:
			result = leftValue & rightValue
		case code.OpBitOr:
			result = leftValue | rightValue
		case code.OpBitXor:
			result = leftValue ^ rightValue
		case code.OpLShift:
			result = leftValue << uint(rightValue)
		case code.OpRShift:
			result = leftValue >> uint(rightValue)
		}
		return vm.push(&object.Integer{Value: result})
	}

	if left.Type() == object.FLOAT_OBJ || right.Type() == object.FLOAT_OBJ {
		var lVal, rVal float64
		if left.Type() == object.FLOAT_OBJ {
			lVal = left.(*object.Float).Value
		} else {
			lVal = float64(left.(*object.Integer).Value)
		}
		if right.Type() == object.FLOAT_OBJ {
			rVal = right.(*object.Float).Value
		} else {
			rVal = float64(right.(*object.Integer).Value)
		}

		var result float64
		switch op {
		case code.OpAdd:
			result = lVal + rVal
		case code.OpSub:
			result = lVal - rVal
		case code.OpMul:
			result = lVal * rVal
		case code.OpDiv:
			result = lVal / rVal
		}
		return vm.push(&object.Float{Value: result})
	}

	return fmt.Errorf("unsupported types for binary operation: %s (%v) and %s (%v) [Op: %d]", left.Type(), left, right.Type(), right, op)
}

func (vm *VM) executeComparison(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()
	if left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ {
		return vm.executeIntegerComparison(op, left, right)
	}
	if left.Type() == object.FLOAT_OBJ || right.Type() == object.FLOAT_OBJ {
		return vm.executeFloatComparison(op, left, right)
	}
	if left.Type() == object.BOOLEAN_OBJ && right.Type() == object.BOOLEAN_OBJ {
		return vm.executeBooleanComparison(op, left, right)
	}
	switch op {
	case code.OpEqual:
		if left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ {
			return vm.push(nativeBoolToBooleanObject(left.(*object.String).Value == right.(*object.String).Value))
		}
		return vm.push(nativeBoolToBooleanObject(left == right))
	case code.OpNotEqual:
		if left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ {
			return vm.push(nativeBoolToBooleanObject(left.(*object.String).Value != right.(*object.String).Value))
		}
		return vm.push(nativeBoolToBooleanObject(left != right))
	case code.OpGreaterThan:
		return vm.push(nativeBoolToBooleanObject(left.Inspect() > right.Inspect()))
	default:
		return fmt.Errorf("unknown operator: %d", op)
	}
}

func (vm *VM) executeIntegerComparison(op code.Opcode, left, right object.Object) error {
	leftVal := left.(*object.Integer).Value
	rightVal := right.(*object.Integer).Value
	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(leftVal == rightVal))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(leftVal != rightVal))
	case code.OpGreaterThan:
		return vm.push(nativeBoolToBooleanObject(leftVal > rightVal))
	default:
		return fmt.Errorf("unknown operator: %d", op)
	}
}

func (vm *VM) executeFloatComparison(op code.Opcode, left, right object.Object) error {
	var lVal, rVal float64
	if left.Type() == object.FLOAT_OBJ {
		lVal = left.(*object.Float).Value
	} else {
		lVal = float64(left.(*object.Integer).Value)
	}
	if right.Type() == object.FLOAT_OBJ {
		rVal = right.(*object.Float).Value
	} else {
		rVal = float64(right.(*object.Integer).Value)
	}

	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(lVal == rVal))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(lVal != rVal))
	case code.OpGreaterThan:
		return vm.push(nativeBoolToBooleanObject(lVal > rVal))
	default:
		return fmt.Errorf("unknown operator: %d", op)
	}
}

func (vm *VM) executeBooleanComparison(op code.Opcode, left, right object.Object) error {
	leftVal := left.(*object.Boolean).Value
	rightVal := right.(*object.Boolean).Value
	lInt, rInt := 0, 0
	if leftVal {
		lInt = 1
	}
	if rightVal {
		rInt = 1
	}
	switch op {
	case code.OpEqual:
		return vm.push(nativeBoolToBooleanObject(leftVal == rightVal))
	case code.OpNotEqual:
		return vm.push(nativeBoolToBooleanObject(leftVal != rightVal))
	case code.OpGreaterThan:
		return vm.push(nativeBoolToBooleanObject(lInt > rInt))
	default:
		return fmt.Errorf("unknown operator: %d", op)
	}
}

func (vm *VM) executePrefixMinus() error {
	operand := vm.pop()
	switch operand.Type() {
	case object.INTEGER_OBJ:
		value := operand.(*object.Integer).Value
		return vm.push(&object.Integer{Value: -value})
	case object.FLOAT_OBJ:
		value := operand.(*object.Float).Value
		return vm.push(&object.Float{Value: -value})
	default:
		return fmt.Errorf("unsupported type for prefix minus: %s", operand.Type())
	}
}

func (vm *VM) executeBangOperator() error {
	operand := vm.pop()
	switch operand {
	case object.TRUE:
		return vm.push(object.FALSE)
	case object.FALSE:
		return vm.push(object.TRUE)
	case object.NULL:
		return vm.push(object.TRUE)
	default:
		return vm.push(object.FALSE)
	}
}

func (vm *VM) executePrefixBitNot() error {
	operand := vm.pop()
	if operand.Type() != object.INTEGER_OBJ {
		return fmt.Errorf("unsupported type for bitwise not: %s", operand.Type())
	}
	value := operand.(*object.Integer).Value
	return vm.push(&object.Integer{Value: ^value})
}

func (vm *VM) push(obj object.Object) error {
	if vm.sp >= StackSize {
		return fmt.Errorf("stack overflow")
	}
	vm.stack[vm.sp] = obj
	vm.sp++
	return nil
}

func (vm *VM) pop() object.Object {
	obj := vm.stack[vm.sp-1]
	vm.sp--
	return obj
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return object.TRUE
	}
	return object.FALSE
}
