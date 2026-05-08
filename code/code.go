package code

import (
	"bytes"
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
	OpMod
	OpBitAnd
	OpBitOr
	OpBitXor
	OpLShift
	OpRShift
	OpPop
	OpTrue
	OpFalse
	OpEqual
	OpNotEqual
	OpGreaterThan
	OpMinus
	OpBang
	OpBitNot
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
	OpAwait
	OpIndex
	OpSetIndex
	OpGetFree
	OpClosure
	OpSetFree
	OpIterInit
	OpIterNext
	OpYield
	OpMap        // operand: 2-byte number of key-value pairs
	OpDup        // duplicate the top value on the stack
	OpDup2       // duplicate the top two values on the stack
	OpSwap       // swap the top two values on the stack
	OpRot        // rotate the top three values on the stack (A, B, C -> C, A, B)
	OpPick       // operand: 1-byte index from top (0 is top). Push stack[sp-1-n]
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
	OpMod:         {"OpMod", []int{}},
	OpBitAnd:      {"OpBitAnd", []int{}},
	OpBitOr:       {"OpBitOr", []int{}},
	OpBitXor:      {"OpBitXor", []int{}},
	OpLShift:      {"OpLShift", []int{}},
	OpRShift:      {"OpRShift", []int{}},
	OpPop:         {"OpPop", []int{}},
	OpTrue:        {"OpTrue", []int{}},
	OpFalse:       {"OpFalse", []int{}},
	OpEqual:       {"OpEqual", []int{}},
	OpNotEqual:    {"OpNotEqual", []int{}},
	OpGreaterThan: {"OpGreaterThan", []int{}},
	OpMinus:       {"OpMinus", []int{}},
	OpBang:        {"OpBang", []int{}},
	OpBitNot:      {"OpBitNot", []int{}},
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
	OpAwait:         {"OpAwait", []int{}},
	OpIndex:         {"OpIndex", []int{}},
	OpSetIndex:      {"OpSetIndex", []int{}},
	OpGetFree:       {"OpGetFree", []int{1}},
	OpClosure:       {"OpClosure", []int{2, 1}}, // 2rd operand is number of upvalues
	OpSetFree:       {"OpSetFree", []int{1}},
	OpIterInit:      {"OpIterInit", []int{}},
	OpIterNext:      {"OpIterNext", []int{}},
	OpYield:         {"OpYield", []int{}},
	OpMap:           {"OpMap", []int{2}},
	OpDup:           {"OpDup", []int{}},
	OpDup2:          {"OpDup2", []int{}},
	OpSwap:          {"OpSwap", []int{}},
	OpRot:           {"OpRot", []int{}},
	OpPick:          {"OpPick", []int{1}},
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
func (ins Instructions) String() string {
	var out bytes.Buffer

	i := 0
	for i < len(ins) {
		def, err := Lookup(ins[i])
		if err != nil {
			fmt.Fprintf(&out, "ERROR: %s\n", err)
			continue
		}

		operands, read := ReadOperands(def, ins[i+1:])

		fmt.Fprintf(&out, "%04d %s\n", i, ins.fmtInstruction(def, operands))

		i += 1 + read
	}

	return out.String()
}

func (ins Instructions) fmtInstruction(def *Definition, operands []int) string {
	operandCount := len(def.OperandWidths)

	if len(operands) != operandCount {
		return fmt.Sprintf("ERROR: operand len %d does not match defined %d\n",
			len(operands), operandCount)
	}

	switch operandCount {
	case 0:
		return def.Name
	case 1:
		return fmt.Sprintf("%s %d", def.Name, operands[0])
	case 2:
		return fmt.Sprintf("%s %d %d", def.Name, operands[0], operands[1])
	}

	return fmt.Sprintf("ERROR: unhandled operandCount for %s\n", def.Name)
}

func ReadOperands(def *Definition, ins []byte) ([]int, int) {
	operands := make([]int, len(def.OperandWidths))
	offset := 0

	for i, width := range def.OperandWidths {
		switch width {
		case 2:
			operands[i] = int(binary.BigEndian.Uint16(ins[offset:]))
		case 1:
			operands[i] = int(ins[offset])
		}

		offset += width
	}

	return operands, offset
}
