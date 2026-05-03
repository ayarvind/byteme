package code

import (
	"encoding/binary"
	"fmt"
)

type Instructions []byte

type Opcode byte

const (
	OpConstant Opcode = iota
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpPop
	OpTrue
	OpFalse
	OpEqual
	OpNotEqual
	OpGreaterThan
	OpMinus
	OpBang
	OpGetBuiltin
	OpCall
	OpReturnValue
	OpReturn
	OpJump
	OpJumpNotTruthy
	OpSetGlobal
	OpGetGlobal
	OpNull
	OpGetLocal
	OpSetLocal
	OpArray
	OpSpawn
	OpSend
	OpReceive
	OpStructDef  // push a StructLiteral (type definition) onto stack
	OpStructNew  // pop args + StructLiteral, construct StructInstance
	OpGetField   // pop instance, push field value   (operand: const-idx of field name string)
	OpSetField   // pop value + instance, set field  (operand: const-idx of field name string)
	OpThrow
	OpTry        // operand: 2-byte jump destination for catch block
	OpEndTry
)

type Definition struct {
	Name          string
	OperandWidths []int
}

var definitions = map[Opcode]*Definition{
	OpConstant:    {"OpConstant", []int{2}},
	OpAdd:         {"OpAdd", []int{}},
	OpSub:         {"OpSub", []int{}},
	OpMul:         {"OpMul", []int{}},
	OpDiv:         {"OpDiv", []int{}},
	OpPop:         {"OpPop", []int{}},
	OpTrue:        {"OpTrue", []int{}},
	OpFalse:       {"OpFalse", []int{}},
	OpEqual:       {"OpEqual", []int{}},
	OpNotEqual:    {"OpNotEqual", []int{}},
	OpGreaterThan: {"OpGreaterThan", []int{}},
	OpMinus:       {"OpMinus", []int{}},
	OpBang:        {"OpBang", []int{}},
	OpGetBuiltin:  {"OpGetBuiltin", []int{1}}, // 1-byte index to builtin table
	OpCall:        {"OpCall", []int{1}},       // 1-byte number of arguments
	OpReturnValue: {"OpReturnValue", []int{}},
	OpReturn:      {"OpReturn", []int{}},
	OpJump:          {"OpJump", []int{2}},          // 2-byte jump destination
	OpJumpNotTruthy: {"OpJumpNotTruthy", []int{2}}, // 2-byte jump destination
	OpSetGlobal:     {"OpSetGlobal", []int{2}},
	OpGetGlobal:     {"OpGetGlobal", []int{2}},
	OpNull:          {"OpNull", []int{}},
	OpGetLocal:      {"OpGetLocal", []int{1}},
	OpSetLocal:      {"OpSetLocal", []int{1}},
	OpArray:         {"OpArray", []int{2}}, // 2-byte number of elements
	OpSpawn:         {"OpSpawn", []int{1}}, // 1-byte number of arguments
	OpSend:          {"OpSend", []int{}},
	OpReceive:       {"OpReceive", []int{}},
	OpStructDef:     {"OpStructDef", []int{2}},      // operand: constant-pool index of StructLiteral
	OpStructNew:     {"OpStructNew", []int{1}},      // operand: number of field args passed positionally
	OpGetField:      {"OpGetField", []int{2}},       // operand: constant-pool index of field-name string
	OpSetField:      {"OpSetField", []int{2}},       // operand: constant-pool index of field-name string
	OpThrow:         {"OpThrow", []int{}},
	OpTry:           {"OpTry", []int{2}},
	OpEndTry:        {"OpEndTry", []int{}},
}

func Lookup(op byte) (*Definition, error) {
	def, ok := definitions[Opcode(op)]
	if !ok {
		return nil, fmt.Errorf("opcode %d undefined", op)
	}
	return def, nil
}

func Make(op Opcode, operands ...int) []byte {
	def, ok := definitions[op]
	if !ok {
		return []byte{}
	}

	instructionLen := 1
	for _, w := range def.OperandWidths {
		instructionLen += w
	}

	instruction := make([]byte, instructionLen)
	instruction[0] = byte(op)

	offset := 1
	for i, o := range operands {
		width := def.OperandWidths[i]
		switch width {
		case 2:
			binary.BigEndian.PutUint16(instruction[offset:], uint16(o))
		case 1:
			instruction[offset] = byte(o)
		}
		offset += width
	}

	return instruction
}
