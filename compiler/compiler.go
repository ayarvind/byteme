package compiler

import (
	"fmt"
	"io/ioutil"
	
	"github.com/byteme/compiler/ast"
	"github.com/byteme/compiler/code"
	"github.com/byteme/compiler/lexer"
	"github.com/byteme/compiler/object"
	"github.com/byteme/compiler/parser"
	"github.com/byteme/compiler/token"
)

type Bytecode struct {
	Instructions code.Instructions
	Constants    []object.Object
}

type EmittedInstruction struct {
	Opcode   code.Opcode
	Position int
}

type Compiler struct {
	instructions     code.Instructions
	constants        *[]object.Object
	symbolTable      *SymbolTable
	lastInstruction  EmittedInstruction
}

func New() *Compiler {
	constants := make([]object.Object, 0)
	return &Compiler{
		instructions: code.Instructions{},
		constants:    &constants,
		symbolTable:  NewSymbolTable(),
	}
}

func NewWithState(s *SymbolTable, constants *[]object.Object) *Compiler {
	return &Compiler{
		instructions: code.Instructions{},
		constants:    constants,
		symbolTable:  s,
	}
}

func NewEnclosedCompiler(outer *Compiler) *Compiler {
	s := NewEnclosedSymbolTable(outer.symbolTable)
	return NewWithState(s, outer.constants)
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

	case *ast.ImportStatement:
		filename := n.Path.Value
		input, err := ioutil.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("could not read imported file %s: %s", filename, err)
		}
		
		l := lexer.New(string(input))
		p := parser.New(l)
		program := p.ParseProgram()
		
		if len(p.Errors()) != 0 {
			return fmt.Errorf("parser errors in imported file %s: %v", filename, p.Errors())
		}
		
		// If it's a 'from' import, we hide the module's variables from the global scope,
		// except for the specifically imported ones.
		var savedStore map[string]Symbol
		if n.Token.Type == token.FROM {
			savedStore = make(map[string]Symbol)
			for k, v := range c.symbolTable.store { savedStore[k] = v }
		}

		err = c.Compile(program)
		if err != nil { return err }

		if n.Token.Type == token.FROM {
			newStore := make(map[string]Symbol)
			// keep original globals
			for k, v := range savedStore { newStore[k] = v }
			
			// keep specifically imported symbols
			for _, imp := range n.Imports {
				if sym, ok := c.symbolTable.store[imp.Value]; ok {
					newStore[imp.Value] = sym
				} else {
					return fmt.Errorf("imported symbol %s not found in module %s", imp.Value, filename)
				}
			}
			c.symbolTable.store = newStore
		}

		return nil

	case *ast.ReturnStatement:
		err := c.Compile(n.ReturnValue)
		if err != nil { return err }
		c.emit(code.OpReturnValue)

	case *ast.ExpressionStatement:
		err := c.Compile(n.Expression)
		if err != nil {
			return err
		}
		c.emit(code.OpPop)

	case *ast.InfixExpression:
		// ── Dot operator ──────────────────────────────────────────────────
		if n.Operator == "." {
			ident, ok := n.Right.(*ast.Identifier)
			if !ok {
				return fmt.Errorf("right side of '.' must be a field/method name")
			}

			// Check if left side is a namespace identifier (flat compound symbol)
			if leftIdent, ok := n.Left.(*ast.Identifier); ok {
				compoundKey := leftIdent.Value + "." + ident.Value
				if sym, ok := c.symbolTable.Resolve(compoundKey); ok {
					// It's a namespace method — emit a direct variable get
					if sym.Scope == GlobalScope {
						c.emit(code.OpGetGlobal, sym.Index)
					} else {
						c.emit(code.OpGetLocal, sym.Index)
					}
					return nil
				}
			}

			// Otherwise treat as struct field access
			err := c.Compile(n.Left)
			if err != nil { return err }
			fieldNameIdx := c.addConstant(&object.String{Value: ident.Value})
			c.emit(code.OpGetField, fieldNameIdx)
			return nil
		}

		// Reordering for LessThan logic
		if n.Operator == "<" {
			err := c.Compile(n.Right)
			if err != nil { return err }
			err = c.Compile(n.Left)
			if err != nil { return err }
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
		case "%":
			c.emit(code.OpMod)
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

	case *ast.StringLiteral:
		str := &object.String{Value: n.Value}
		c.emit(code.OpConstant, c.addConstant(str))

	case *ast.BooleanLiteral:
		if n.Value {
			c.emit(code.OpTrue)
		} else {
			c.emit(code.OpFalse)
		}

	case *ast.ArrayLiteral:
		for _, el := range n.Elements {
			err := c.Compile(el)
			if err != nil { return err }
		}
		c.emit(code.OpArray, len(n.Elements))

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

	case *ast.ThrowStatement:
		err := c.Compile(n.Value)
		if err != nil { return err }
		c.emit(code.OpThrow)

	case *ast.TryStatement:
		jumpToCatchPos := c.emit(code.OpTry, 9999)
		
		err := c.Compile(n.Body)
		if err != nil { return err }
		
		c.emit(code.OpEndTry)
		jumpToEndPos := c.emit(code.OpJump, 9999)
		
		catchPos := len(c.instructions)
		c.changeOperand(jumpToCatchPos, catchPos)
		
		if n.CatchBody != nil {
			// Catch variable is pushed onto the stack by OpThrow in VM
			symbol := c.symbolTable.Define(n.CatchVar.Value)
			if symbol.Scope == GlobalScope {
				c.emit(code.OpSetGlobal, symbol.Index)
			} else {
				c.emit(code.OpSetLocal, symbol.Index)
			}
			
			err = c.Compile(n.CatchBody)
			if err != nil { return err }
		} else {
			c.emit(code.OpPop)
		}
		
		endPos := len(c.instructions)
		c.changeOperand(jumpToEndPos, endPos)
		
		if n.Finally != nil {
			err = c.Compile(n.Finally)
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

	case *ast.StructLiteral:
		if n.Name != nil {
			// Build the StructLiteral object and store it in the constant pool
			fields := make([]*ast.Parameter, len(n.Fields))
			copy(fields, n.Fields)
			structDef := &object.StructLiteral{
				Name:   n.Name.Value,
				Fields: fields,
			}
			constIdx := c.addConstant(structDef)
			c.emit(code.OpStructDef, constIdx)

			// Bind the constructor to a global/local symbol
			symbol := c.symbolTable.Define(n.Name.Value)
			if symbol.Scope == GlobalScope {
				c.emit(code.OpSetGlobal, symbol.Index)
			} else {
				c.emit(code.OpSetLocal, symbol.Index)
			}
			c.emit(code.OpNull) // balance stack for ExpressionStatement OpPop
		} else {
			c.emit(code.OpNull)
		}

	case *ast.InterfaceStatement:
		if n.Name != nil {
			symbol := c.symbolTable.Define(n.Name.Value)
			c.emit(code.OpNull)
			if symbol.Scope == GlobalScope {
				c.emit(code.OpSetGlobal, symbol.Index)
			} else {
				c.emit(code.OpSetLocal, symbol.Index)
			}
		}
		c.emit(code.OpNull)

	case *ast.EnumStatement:
		if n.Name != nil {
			symbol := c.symbolTable.Define(n.Name.Value)
			c.emit(code.OpNull)
			if symbol.Scope == GlobalScope {
				c.emit(code.OpSetGlobal, symbol.Index)
			} else {
				c.emit(code.OpSetLocal, symbol.Index)
			}
		}
		c.emit(code.OpNull)

	case *ast.FunctionLiteral:
		if n.Name != nil {
			// Define the name before compiling body to allow recursion
			c.symbolTable.Define(n.Name.Value)
		}

		enclosedCompiler := NewEnclosedCompiler(c)

		if n.Receiver != nil {
			// Prepend receiver as first parameter
			enclosedCompiler.symbolTable.Define(n.Receiver.Name.Value)
		}

		for _, p := range n.Parameters {
			enclosedCompiler.symbolTable.Define(p.Name.Value)
		}

		err := enclosedCompiler.Compile(n.Body)
		if err != nil { return err }

		if !enclosedCompiler.lastInstructionIs(code.OpReturnValue) && !enclosedCompiler.lastInstructionIs(code.OpReturn) {
			enclosedCompiler.emit(code.OpReturn)
		}

		numParams := len(n.Parameters)
		if n.Receiver != nil { numParams++ }

		compiledFn := &object.CompiledFunction{
			Instructions:  enclosedCompiler.instructions,
			NumLocals:     enclosedCompiler.symbolTable.numDefinitions,
			NumParameters: numParams,
			IsAsync:       n.IsAsync,
		}

		if n.Receiver != nil {
			// It's a method! Attach it to the struct definition in constants.
			found := false
			for _, constant := range *c.constants {
				if sl, ok := constant.(*object.StructLiteral); ok && sl.Name == n.Receiver.Type {
					if sl.Methods == nil {
						sl.Methods = make(map[string]*object.CompiledFunction)
					}
					sl.Methods[n.Name.Value] = compiledFn
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("method defined for unknown type: %s", n.Receiver.Type)
			}
			// Emit OpNull so that the wrapping expression statement is balanced
			c.emit(code.OpNull)
		} else {
			c.emit(code.OpConstant, c.addConstant(compiledFn))

			if n.Name != nil {
				symbol, _ := c.symbolTable.Resolve(n.Name.Value)
				if symbol.Scope == GlobalScope {
					c.emit(code.OpSetGlobal, symbol.Index)
				} else {
					c.emit(code.OpSetLocal, symbol.Index)
				}
				// Since OpSetGlobal/Local pops the stack, we push a null
				// so that the wrapping ExpressionStatement's OpPop doesn't panic.
				c.emit(code.OpNull)
			}
		}

	case *ast.CallExpression:
		err := c.Compile(n.Function)
		if err != nil { return err }

		for _, arg := range n.Arguments {
			err := c.Compile(arg)
			if err != nil { return err }
		}

		// If the callee is a StructLiteral (resolved from symbol table),
		// emit OpStructNew instead of OpCall so the VM constructs an instance.
		// We detect this by checking if the function expression is an Identifier
		// that resolves to a StructLiteral in constants — the VM handles this distinction.
		c.emit(code.OpCall, len(n.Arguments))

	case *ast.NamespaceLiteral:
		nsName := n.Name.Value
		// Register the namespace name itself as a symbol (resolves to NULL; its methods live under compound keys)
		symbol := c.symbolTable.Define(nsName)
		c.emit(code.OpNull)
		if symbol.Scope == GlobalScope {
			c.emit(code.OpSetGlobal, symbol.Index)
		} else {
			c.emit(code.OpSetLocal, symbol.Index)
		}

		// Compile each function/var in the body under "NamespaceName.memberName"
		for _, stmt := range n.Body.Statements {
			switch s := stmt.(type) {
			case *ast.ExpressionStatement:
				if fnLit, ok := s.Expression.(*ast.FunctionLiteral); ok && fnLit.Name != nil {
					compoundName := nsName + "." + fnLit.Name.Value
					// Compile the function body into a CompiledFunction
					enclosedCompiler := NewEnclosedCompiler(c)
					for _, p := range fnLit.Parameters {
						enclosedCompiler.symbolTable.Define(p.Name.Value)
					}
					if err := enclosedCompiler.Compile(fnLit.Body); err != nil { return err }
					if !enclosedCompiler.lastInstructionIs(code.OpReturnValue) && !enclosedCompiler.lastInstructionIs(code.OpReturn) {
						enclosedCompiler.emit(code.OpReturn)
					}
					compiledFn := &object.CompiledFunction{
						Instructions:  enclosedCompiler.instructions,
						NumLocals:     enclosedCompiler.symbolTable.numDefinitions,
						NumParameters: len(fnLit.Parameters),
						IsAsync:       fnLit.IsAsync,
					}
					c.emit(code.OpConstant, c.addConstant(compiledFn))
					sym := c.symbolTable.Define(compoundName)
					if sym.Scope == GlobalScope {
						c.emit(code.OpSetGlobal, sym.Index)
					} else {
						c.emit(code.OpSetLocal, sym.Index)
					}
				}
			}
		}
		c.emit(code.OpNull) // balance for ExpressionStatement OpPop

	case *ast.SpawnExpression:
		err := c.Compile(n.Call.Function)
		if err != nil { return err }

		for _, arg := range n.Call.Arguments {
			err := c.Compile(arg)
			if err != nil { return err }
		}

		c.emit(code.OpSpawn, len(n.Call.Arguments))

	case *ast.AwaitExpression:
		err := c.Compile(n.Expression)
		if err != nil { return err }
		c.emit(code.OpAwait)
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
	"map":           11,
	"chan":          12,
	"send":          13,
	"recv":          14,
	"envGet":        15,
	"envSet":        16,
	"args":          17,
	"exit":          18,
	"sha256":        19,
	"md5":           20,
	"regexMatch":    21,
	"regexReplace":  22,
	"osMkdir":       23,
	"osRmdir":       24,
	"osRemove":      25,
	"osRename":      26,
	"osListdir":     27,
	"osExists":      28,
	"osIsdir":       29,
	"osIsfile":      30,
	"osGetcwd":      31,
	"osChdir":       32,
	"osGetpid":      33,
	"pathJoin":      34,
	"pathBase":      35,
	"pathDir":       36,
	"fOpen":         37,
	"fClose":        38,
	"fRead":         39,
	"fWrite":        40,
	"fSeek":         41,
	// HTTP / networking
	"httpHandle":   42,
	"httpServe":    43,
	"httpGet":      44,
	"httpPost":     45,
	"httpResponse": 46,
}

func (c *Compiler) Bytecode() *Bytecode {
	return &Bytecode{
		Instructions: c.instructions,
		Constants:    *c.constants,
	}
}

func (c *Compiler) addConstant(obj object.Object) int {
	*c.constants = append(*c.constants, obj)
	return len(*c.constants) - 1
}

func (c *Compiler) emit(op code.Opcode, operands ...int) int {
	ins := code.Make(op, operands...)
	pos := c.addInstruction(ins)

	c.lastInstruction = EmittedInstruction{Opcode: op, Position: pos}

	return pos
}

func (c *Compiler) lastInstructionIs(op code.Opcode) bool {
	if len(c.instructions) == 0 {
		return false
	}
	return c.lastInstruction.Opcode == op
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
