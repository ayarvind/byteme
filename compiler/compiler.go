package compiler

import (
	"fmt"
	"github.com/byteme/compiler/ast"
	"github.com/byteme/compiler/code"
	"github.com/byteme/compiler/object"
)

type Bytecode struct {
	Instructions code.Instructions
	Constants    []object.Object
}

type Compiler struct {
	instructions code.Instructions
	constants    []object.Object
	symbolTable  *SymbolTable
}

func New() *Compiler {
	return &Compiler{
		instructions: code.Instructions{},
		constants:    []object.Object{},
		symbolTable:  NewSymbolTable(),
	}
}

func (c *Compiler) Compile(node ast.Node) error {
	switch n := node.(type) {
	case *ast.Program:
		for _, s := range n.Statements {
			err := c.Compile(s)
			if err != nil {
				return err
			}
		}

	case *ast.ExpressionStatement:
		err := c.Compile(n.Expression)
		if err != nil {
			return err
		}
		c.emit(code.OpPop)

	case *ast.InfixExpression:
		// Reordering for GreaterThan logic
		if n.Operator == "<" {
			err := c.Compile(n.Right)
			if err != nil {
				return err
			}
			err = c.Compile(n.Left)
			if err != nil {
				return err
			}
			c.emit(code.OpGreaterThan)
			return nil
		}

		err := c.Compile(n.Left)
		if err != nil {
			return err
		}

		err = c.Compile(n.Right)
		if err != nil {
			return err
		}

		switch n.Operator {
		case "+":
			c.emit(code.OpAdd)
		case "-":
			c.emit(code.OpSub)
		case "*":
			c.emit(code.OpMul)
		case "/":
			c.emit(code.OpDiv)
		case ">":
			c.emit(code.OpGreaterThan)
		case "==":
			c.emit(code.OpEqual)
		case "!=":
			c.emit(code.OpNotEqual)
		default:
			return fmt.Errorf("unknown operator %s", n.Operator)
		}

	case *ast.PrefixExpression:
		err := c.Compile(n.Right)
		if err != nil {
			return err
		}
		switch n.Operator {
		case "!":
			c.emit(code.OpBang)
		case "-":
			c.emit(code.OpMinus)
		default:
			return fmt.Errorf("unknown operator %s", n.Operator)
		}

	case *ast.IntegerLiteral:
		integer := &object.Integer{Value: n.Value}
		c.emit(code.OpConstant, c.addConstant(integer))

	case *ast.BooleanLiteral:
		if n.Value {
			c.emit(code.OpTrue)
		} else {
			c.emit(code.OpFalse)
		}

	case *ast.LetStatement:
		err := c.Compile(n.Value)
		if err != nil { return err }
		symbol := c.symbolTable.Define(n.Name.Value)
		if symbol.Scope == GlobalScope {
			c.emit(code.OpSetGlobal, symbol.Index)
		} else {
			c.emit(code.OpSetLocal, symbol.Index)
		}
		return nil

	case *ast.IfExpression:
		err := c.Compile(n.Condition)
		if err != nil { return err }

		// Emit jump with placeholder
		jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

		err = c.Compile(n.Consequence)
		if err != nil { return err }

		// Remove the pop if it was an expression statement? No, the VM needs to be consistent.
		// For now, if consequence is empty, we might have issues.
		
		if n.Alternative == nil {
			afterConsequencePos := len(c.instructions)
			c.changeOperand(jumpNotTruthyPos, afterConsequencePos)
			// Push NULL so the OpPop of the ExpressionStatement doesn't panic
			c.emit(code.OpNull) // Placeholder for 'if' result
		} else {
			jumpPos := c.emit(code.OpJump, 9999)
			
			afterConsequencePos := len(c.instructions)
			c.changeOperand(jumpNotTruthyPos, afterConsequencePos)
			
			err = c.Compile(n.Alternative)
			if err != nil { return err }
			
			afterAlternativePos := len(c.instructions)
			c.changeOperand(jumpPos, afterAlternativePos)
		}
		return nil

	case *ast.BlockStatement:
		for _, s := range n.Statements {
			err := c.Compile(s)
			if err != nil { return err }
		}

	case *ast.Identifier:
		// Check for built-ins
		if index, ok := builtins[n.Value]; ok {
			c.emit(code.OpGetBuiltin, index)
			return nil
		}
		
		symbol, ok := c.symbolTable.Resolve(n.Value)
		if !ok {
			return fmt.Errorf("undefined variable: %s", n.Value)
		}
		
		if symbol.Scope == GlobalScope {
			c.emit(code.OpGetGlobal, symbol.Index)
		} else {
			c.emit(code.OpGetLocal, symbol.Index)
		}

	case *ast.CallExpression:
		err := c.Compile(n.Function)
		if err != nil { return err }

		for _, arg := range n.Arguments {
			err := c.Compile(arg)
			if err != nil { return err }
		}

		c.emit(code.OpCall, len(n.Arguments))
	}

	return nil
}

var builtins = map[string]int{
	"println":       0,
	"len":           1,
	"timeNow":       2,
	"jsonParse":     3,
	"jsonStringify": 4,
	"timeSleep":     5,
	"timeFormat":    6,
	"mapSet":        7,
	"mapGet":        8,
	"fileRead":      9,
	"fileWrite":     10,
}

func (c *Compiler) Bytecode() *Bytecode {
	return &Bytecode{
		Instructions: c.instructions,
		Constants:    c.constants,
	}
}

func (c *Compiler) addConstant(obj object.Object) int {
	c.constants = append(c.constants, obj)
	return len(c.constants) - 1
}

func (c *Compiler) emit(op code.Opcode, operands ...int) int {
	ins := code.Make(op, operands...)
	pos := c.addInstruction(ins)
	return pos
}

func (c *Compiler) addInstruction(ins []byte) int {
	posNewInstruction := len(c.instructions)
	c.instructions = append(c.instructions, ins...)
	return posNewInstruction
}

func (c *Compiler) changeOperand(opPos int, operand int) {
	op := code.Opcode(c.instructions[opPos])
	newInstruction := code.Make(op, operand)
	c.replaceInstruction(opPos, newInstruction)
}

func (c *Compiler) replaceInstruction(pos int, newInstruction []byte) {
	for i := 0; i < len(newInstruction); i++ {
		c.instructions[pos+i] = newInstruction[i]
	}
}
