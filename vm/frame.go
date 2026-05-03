package vm

import (
	"github.com/byteme/compiler/code"
	"github.com/byteme/compiler/object"
)

type Frame struct {
	cl           *object.Closure
	ip           int
	basePointer  int
}

func NewFrame(cl *object.Closure, basePointer int) *Frame {
	return &Frame{
		cl:           cl,
		ip:           -1,
		basePointer:  basePointer,
	}
}

func (f *Frame) Instructions() code.Instructions {
	return f.cl.Fn.Instructions
}
