package vm

import (
	"github.com/byteme/compiler/code"
)

type Frame struct {
	instructions code.Instructions
	ip           int
	basePointer  int
}

func NewFrame(ins code.Instructions, basePointer int) *Frame {
	return &Frame{
		instructions: ins,
		ip:           -1,
		basePointer:  basePointer,
	}
}
