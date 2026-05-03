package vm

import (
	"encoding/binary"
	"fmt"
	"github.com/byteme/compiler/code"
	"github.com/byteme/compiler/compiler"
	"github.com/byteme/compiler/object"
)

const StackSize = 2048
const MaxFrames = 1024
const GlobalsSize = 65536

type VM struct {
	constants []object.Object
	globals   []object.Object

	stack []object.Object
	sp    int // Stack pointer

	frames      []*Frame
	framesIndex int

	catchHandlers []*CatchHandler
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
}

func New(bytecode *compiler.Bytecode) *VM {
	mainFrame := NewFrame(bytecode.Instructions, 0)
	frames := make([]*Frame, MaxFrames)
	frames[0] = mainFrame

	vm := &VM{
		constants:   bytecode.Constants,
		globals:     make([]object.Object, GlobalsSize),
		stack:       make([]object.Object, StackSize),
		sp:          0,
		frames:        frames,
		framesIndex:   1,
		catchHandlers: make([]*CatchHandler, 0),
	}
	vm.registerRunner()
	return vm
}

func NewWithGlobalStore(constants []object.Object, globals []object.Object) *VM {
	frames := make([]*Frame, MaxFrames)
	frames[0] = NewFrame(code.Instructions{}, 0)

	vm := &VM{
		constants:   constants,
		globals:     globals,
		stack:       make([]object.Object, StackSize),
		sp:          0,
		frames:        frames,
		framesIndex:   1,
		catchHandlers: make([]*CatchHandler, 0),
	}
	vm.registerRunner()
	return vm
}

// registerRunner sets the package-level object.RunFunction so that HTTP
// (and other) builtins can call back into ByteMe compiled functions.
func (vm *VM) registerRunner() {
	object.RunFunction = func(fn *object.CompiledFunction, constants []object.Object, globals []object.Object, args []object.Object) object.Object {
		child := NewWithGlobalStore(constants, globals)
		// Push the function then its arguments, then set up the call frame.
		if err := child.push(fn); err != nil {
			return object.NULL
		}
		for _, a := range args {
			if err := child.push(a); err != nil {
				return object.NULL
			}
		}
		frame := NewFrame(fn.Instructions, child.sp-len(args))
		child.pushFrame(frame)
		child.sp = frame.basePointer + fn.NumLocals

		if err := child.Run(); err != nil {
			return &object.Error{Message: err.Error()}
		}
		return child.StackTop()
	}

	// Patch the httpRoutes entries so they carry the VM's constants & globals.
	// We do this lazily when a handler fires, by passing them through RunFunction.
	// The HTTP server reads them from the route entry set via httpHandle.
	// Since httpHandle is called at script evaluation time, the globals are live
	// and the entry can snapshot them now.
	//
	// We expose a post-registration hook so httpHandle can store the current VM refs.
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
	var ins code.Instructions
	var op code.Opcode

	for vm.currentFrame().ip < len(vm.currentFrame().instructions)-1 {
		vm.currentFrame().ip++
		
		ip = vm.currentFrame().ip
		ins = vm.currentFrame().instructions
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
			vm.push(object.TRUE)
		case code.OpFalse:
			vm.push(object.FALSE)

		case code.OpEqual, code.OpNotEqual, code.OpGreaterThan:
			err := vm.executeComparison(op)
			if err != nil { return err }

		case code.OpMinus:
			err := vm.executePrefixMinus()
			if err != nil { return err }

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
			switch fn := fn.(type) {
			case *object.Builtin:
				args := vm.stack[vm.sp-numArgs : vm.sp]
				result := fn.Fn(args...)
				vm.sp = vm.sp - numArgs - 1
				if result != nil {
					vm.push(result)
				} else {
					vm.push(object.NULL)
				}
			case *object.StructLiteral:
				// Struct constructor: Point(x, y) → StructInstance{x: ..., y: ...}
				if numArgs != len(fn.Fields) {
					return fmt.Errorf("struct %s requires %d fields, got %d", fn.Name, len(fn.Fields), numArgs)
				}
				instance := &object.StructInstance{
					Definition: fn,
					Fields:     make(map[string]object.Object),
				}
				for i, field := range fn.Fields {
					instance.Fields[field.Name.Value] = vm.stack[vm.sp-numArgs+i]
				}
				vm.sp = vm.sp - numArgs - 1
				vm.push(instance)
			case *object.BoundMethod:
				if numArgs != fn.Method.NumParameters-1 {
					return fmt.Errorf("wrong number of arguments for method: want=%d, got=%d", fn.Method.NumParameters-1, numArgs)
				}
				// Shift arguments up to make room for receiver
				for i := vm.sp; i > vm.sp-numArgs; i-- {
					vm.stack[i] = vm.stack[i-1]
				}
				vm.stack[vm.sp-numArgs] = fn.Receiver
				vm.sp++

				frame := NewFrame(fn.Method.Instructions, vm.sp-numArgs-1)
				vm.pushFrame(frame)
				vm.sp = frame.basePointer + fn.Method.NumLocals

			case *object.CompiledFunction:
				if numArgs != fn.NumParameters {
					return fmt.Errorf("wrong number of arguments: want=%d, got=%d", fn.NumParameters, numArgs)
				}
				
				if fn.IsAsync {
					// Async call: return a Future immediately
					future := &object.Future{ValueChan: make(chan object.Object, 1)}
					newVM := NewWithGlobalStore(vm.constants, vm.globals)
					
					newVM.push(fn)
					for i := vm.sp - numArgs; i < vm.sp; i++ {
						newVM.push(vm.stack[i])
					}
					
					vm.sp = vm.sp - numArgs - 1
					vm.push(future)

					go func() {
						frame := NewFrame(fn.Instructions, newVM.sp-numArgs)
						newVM.pushFrame(frame)
						newVM.sp = frame.basePointer + fn.NumLocals
						newVM.Run()
						result := newVM.StackTop()
						future.ValueChan <- result
					}()
				} else {
					frame := NewFrame(fn.Instructions, vm.sp-numArgs)
					vm.pushFrame(frame)
					vm.sp = frame.basePointer + fn.NumLocals
				}

			default:
				return fmt.Errorf("calling non-function and non-built-in: %T", fn)
			}

		case code.OpReturnValue:
			returnValue := vm.pop()
			frame := vm.popFrame()
			vm.sp = frame.basePointer - 1
			vm.push(returnValue)

		case code.OpReturn:
			frame := vm.popFrame()
			vm.sp = frame.basePointer - 1
			vm.push(object.NULL)

		case code.OpSpawn:
			numArgs := int(ins[ip+1])
			vm.currentFrame().ip += 1
			
			fn := vm.stack[vm.sp-1-numArgs]
			switch fn := fn.(type) {
			case *object.Builtin:
				args := make([]object.Object, numArgs)
				copy(args, vm.stack[vm.sp-numArgs : vm.sp])
				vm.sp = vm.sp - numArgs - 1
				go func() {
					fn.Fn(args...)
				}()
				err := vm.push(object.NULL)
				if err != nil { return err }
			case *object.CompiledFunction:
				if numArgs != fn.NumParameters {
					return fmt.Errorf("wrong number of arguments: want=%d, got=%d", fn.NumParameters, numArgs)
				}
				// Spawn a new VM!
				newVM := NewWithGlobalStore(vm.constants, vm.globals)
				
				newVM.push(fn)
				for i := vm.sp - numArgs; i < vm.sp; i++ {
					newVM.push(vm.stack[i])
				}
				
				vm.sp = vm.sp - numArgs - 1
				vm.push(object.NULL)

				go func() {
					frame := NewFrame(fn.Instructions, newVM.sp-numArgs)
					newVM.pushFrame(frame)
					newVM.sp = frame.basePointer + fn.NumLocals
					newVM.Run()
				}()
			default:
				return fmt.Errorf("spawning non-function")
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
				vm.push(result)
			} else {
				// If it's not a future, just return it (like JS await)
				vm.push(obj)
			}

		// ── Struct opcodes ────────────────────────────────────────────────
		case code.OpStructDef:
			// Push the StructLiteral (type definition) from the constant pool
			constIdx := int(binary.BigEndian.Uint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			err := vm.push(vm.constants[constIdx])
			if err != nil { return err }

		case code.OpGetField:
			// Stack: [..., instance]  → push instance.field
			constIdx := int(binary.BigEndian.Uint16(ins[ip+1:]))
			vm.currentFrame().ip += 2
			fieldName := vm.constants[constIdx].(*object.String).Value
			instance := vm.pop()
			switch inst := instance.(type) {
			case *object.StructInstance:
				val, ok := inst.Fields[fieldName]
				if ok {
					vm.push(val)
				} else {
					// Check methods
					method, ok := inst.Definition.Methods[fieldName]
					if ok {
						vm.push(&object.BoundMethod{Receiver: instance, Method: method})
					} else {
						vm.push(object.NULL)
					}
				}
			case *object.Map:
				val, ok := inst.Pairs[fieldName]
				if !ok { vm.push(object.NULL) } else { vm.push(val) }
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
				vm.push(instance)
			case *object.Map:
				inst.Pairs[fieldName] = value
				vm.push(instance)
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
			vm.push(errValue)

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
		}
	}
	return nil
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
		case code.OpAdd:      result = leftValue + rightValue
		case code.OpSub:      result = leftValue - rightValue
		case code.OpMul:      result = leftValue * rightValue
		case code.OpDiv:      result = leftValue / rightValue
		case code.OpMod:      result = leftValue % rightValue
		case code.OpBitAnd:   result = leftValue & rightValue
		case code.OpBitOr:    result = leftValue | rightValue
		case code.OpBitXor:   result = leftValue ^ rightValue
		case code.OpLShift:   result = leftValue << uint(rightValue)
		case code.OpRShift:   result = leftValue >> uint(rightValue)
		}
		return vm.push(&object.Integer{Value: result})
	}

	if left.Type() == object.FLOAT_OBJ || right.Type() == object.FLOAT_OBJ {
		var lVal, rVal float64
		if left.Type() == object.FLOAT_OBJ { lVal = left.(*object.Float).Value } else { lVal = float64(left.(*object.Integer).Value) }
		if right.Type() == object.FLOAT_OBJ { rVal = right.(*object.Float).Value } else { rVal = float64(right.(*object.Integer).Value) }
		
		var result float64
		switch op {
		case code.OpAdd: result = lVal + rVal
		case code.OpSub: result = lVal - rVal
		case code.OpMul: result = lVal * rVal
		case code.OpDiv: result = lVal / rVal
		}
		return vm.push(&object.Float{Value: result})
	}

	return fmt.Errorf("unsupported types for binary operation: %s and %s", left.Type(), right.Type())
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
	case code.OpEqual: return vm.push(nativeBoolToBooleanObject(left == right))
	case code.OpNotEqual: return vm.push(nativeBoolToBooleanObject(left != right))
	default: return fmt.Errorf("unknown operator: %d", op)
	}
}

func (vm *VM) executeIntegerComparison(op code.Opcode, left, right object.Object) error {
	leftVal := left.(*object.Integer).Value
	rightVal := right.(*object.Integer).Value
	switch op {
	case code.OpEqual: return vm.push(nativeBoolToBooleanObject(leftVal == rightVal))
	case code.OpNotEqual: return vm.push(nativeBoolToBooleanObject(leftVal != rightVal))
	case code.OpGreaterThan: return vm.push(nativeBoolToBooleanObject(leftVal > rightVal))
	default: return fmt.Errorf("unknown operator: %d", op)
	}
}

func (vm *VM) executeFloatComparison(op code.Opcode, left, right object.Object) error {
	var lVal, rVal float64
	if left.Type() == object.FLOAT_OBJ { lVal = left.(*object.Float).Value } else { lVal = float64(left.(*object.Integer).Value) }
	if right.Type() == object.FLOAT_OBJ { rVal = right.(*object.Float).Value } else { rVal = float64(right.(*object.Integer).Value) }
	
	switch op {
	case code.OpEqual: return vm.push(nativeBoolToBooleanObject(lVal == rVal))
	case code.OpNotEqual: return vm.push(nativeBoolToBooleanObject(lVal != rVal))
	case code.OpGreaterThan: return vm.push(nativeBoolToBooleanObject(lVal > rVal))
	default: return fmt.Errorf("unknown operator: %d", op)
	}
}

func (vm *VM) executeBooleanComparison(op code.Opcode, left, right object.Object) error {
	leftVal := left.(*object.Boolean).Value
	rightVal := right.(*object.Boolean).Value
	lInt, rInt := 0, 0
	if leftVal { lInt = 1 }
	if rightVal { rInt = 1 }
	switch op {
	case code.OpEqual: return vm.push(nativeBoolToBooleanObject(leftVal == rightVal))
	case code.OpNotEqual: return vm.push(nativeBoolToBooleanObject(leftVal != rightVal))
	case code.OpGreaterThan: return vm.push(nativeBoolToBooleanObject(lInt > rInt))
	default: return fmt.Errorf("unknown operator: %d", op)
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
	case object.TRUE: return vm.push(object.FALSE)
	case object.FALSE: return vm.push(object.TRUE)
	case object.NULL: return vm.push(object.TRUE)
	default: return vm.push(object.FALSE)
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
	if vm.sp >= StackSize { return fmt.Errorf("stack overflow") }
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
	if input { return object.TRUE }
	return object.FALSE
}
