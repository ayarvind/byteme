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
}

func New(bytecode *compiler.Bytecode) *VM {
	mainFrame := NewFrame(bytecode.Instructions, 0)
	frames := make([]*Frame, MaxFrames)
	frames[0] = mainFrame

	return &VM{
		constants:   bytecode.Constants,
		globals:     make([]object.Object, GlobalsSize),
		stack:       make([]object.Object, StackSize),
		sp:          0,
		frames:      frames,
		framesIndex: 1,
	}
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

		case code.OpAdd, code.OpSub, code.OpMul, code.OpDiv:
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
			if builtin, ok := fn.(*object.Builtin); ok {
				args := vm.stack[vm.sp-numArgs : vm.sp]
				result := builtin.Fn(args...)
				vm.sp = vm.sp - numArgs - 1
				if result != nil {
					vm.push(result)
				} else {
					vm.push(object.NULL)
				}
			}

		case code.OpSpawn:
			numArgs := int(ins[ip+1])
			vm.currentFrame().ip += 1
			
			fn := vm.stack[vm.sp-1-numArgs]
			if builtin, ok := fn.(*object.Builtin); ok {
				args := make([]object.Object, numArgs)
				copy(args, vm.stack[vm.sp-numArgs : vm.sp])
				vm.sp = vm.sp - numArgs - 1
				go func() {
					builtin.Fn(args...)
				}()
				err := vm.push(object.NULL)
				if err != nil { return err }
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
	leftValue := left.(*object.Integer).Value
	rightValue := right.(*object.Integer).Value
	var result int64
	switch op {
	case code.OpAdd: result = leftValue + rightValue
	case code.OpSub: result = leftValue - rightValue
	case code.OpMul: result = leftValue * rightValue
	case code.OpDiv: result = leftValue / rightValue
	}
	return vm.push(&object.Integer{Value: result})
}

func (vm *VM) executeComparison(op code.Opcode) error {
	right := vm.pop()
	left := vm.pop()
	if left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ {
		return vm.executeIntegerComparison(op, left, right)
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
	if operand.Type() != object.INTEGER_OBJ {
		return fmt.Errorf("unsupported type for prefix minus: %s", operand.Type())
	}
	value := operand.(*object.Integer).Value
	return vm.push(&object.Integer{Value: -value})
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
