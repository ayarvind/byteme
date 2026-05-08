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
	SourceMap    map[int]int
	Filename     string
	NumLocals    int
}

type EmittedInstruction struct {
	Opcode   code.Opcode
	Position int
}

type Compiler struct {
	instructions    code.Instructions
	constants       *[]object.Object
	symbolTable     *SymbolTable
	lastInstruction EmittedInstruction
	sourceMap       map[int]int
	currentNode     ast.Node
	Filename        string
	loops           []*LoopInfo
}

type LoopInfo struct {
	StartPos      int
	BreakJumps    []int
	ContinueJumps []int
}

func New() *Compiler {
	constants := make([]object.Object, 0)
	c := &Compiler{
		instructions: code.Instructions{},
		constants:    &constants,
		symbolTable:  NewSymbolTable(),
		sourceMap:    make(map[int]int),
	}

	for name, index := range builtins {
		c.symbolTable.DefineBuiltin(index, name)
	}

	c.loops = []*LoopInfo{}

	return c
}

func NewWithState(s *SymbolTable, constants *[]object.Object) *Compiler {
	return &Compiler{
		instructions: code.Instructions{},
		constants:    constants,
		symbolTable:  s,
		sourceMap:    make(map[int]int),
	}
}

func NewEnclosedCompiler(outer *Compiler) *Compiler {
	s := NewEnclosedSymbolTable(outer.symbolTable)
	c := NewWithState(s, outer.constants)
	c.Filename = outer.Filename
	return c
}

func (c *Compiler) Compile(node ast.Node) error {
	if node == nil { return nil }
	oldNode := c.currentNode
	c.currentNode = node
	defer func() { c.currentNode = oldNode }()

	switch n := node.(type) {
	case *ast.Program:
		// Pre-scan pass
		for _, s := range n.Statements {
			c.preScan(s)
		}
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
		if n.Token.Type == token.FROM || n.Name != nil {
			savedStore = make(map[string]Symbol)
			for k, v := range c.symbolTable.store {
				savedStore[k] = v
			}
		}

		err = c.Compile(program)
		if err != nil {
			return err
		}

		if n.Token.Type == token.FROM {
			newStore := make(map[string]Symbol)
			// keep original globals
			for k, v := range savedStore {
				newStore[k] = v
			}

			// keep specifically imported symbols
			for _, imp := range n.Imports {
				if sym, ok := c.symbolTable.store[imp.Value]; ok {
					newStore[imp.Value] = sym
				} else {
					return fmt.Errorf("imported symbol %s not found in module %s", imp.Value, filename)
				}
			}
			c.symbolTable.store = newStore
		} else if n.Name != nil {
			// Module aliasing: import "path" as alias
			// 1. Collect all NEW symbols defined in the module
			newSymbols := []string{}
			for k := range c.symbolTable.store {
				if _, ok := savedStore[k]; !ok {
					newSymbols = append(newSymbols, k)
				}
			}

			// 2. Emit code to create a Map containing these symbols
			for _, name := range newSymbols {
				sym := c.symbolTable.store[name]
				c.emit(code.OpConstant, c.addConstant(&object.String{Value: name}))
				c.emit(code.OpGetGlobal, sym.Index)
			}
			c.emit(code.OpMap, len(newSymbols))

			// 3. Restore previous symbols
			c.symbolTable.store = savedStore

			// 4. Define the alias and assign the map to it
			aliasSym := c.symbolTable.Define(n.Name.Value, "map")
			c.emit(code.OpSetGlobal, aliasSym.Index)
		}

		return nil

	case *ast.ReturnStatement:
		err := c.Compile(n.ReturnValue)
		if err != nil {
			return err
		}
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

			// Otherwise treat as struct field access
			err := c.Compile(n.Left)
			if err != nil {
				return err
			}
			fieldNameIdx := c.addConstant(&object.String{Value: ident.Value})
			c.emit(code.OpGetField, fieldNameIdx)
			return nil
		}

		if n.Operator == "=" {
			// Left side is what we are assigning to
			switch left := n.Left.(type) {
			case *ast.Identifier:
				err := c.Compile(n.Right)
				if err != nil {
					return err
				}

				symbol, ok := c.symbolTable.Resolve(left.Value)
				if !ok {
					return fmt.Errorf("undefined variable: %s", left.Value)
				}
				if symbol.Scope == GlobalScope {
					c.emit(code.OpSetGlobal, symbol.Index)
				} else if symbol.Scope == LocalScope {
					c.emit(code.OpSetLocal, symbol.Index)
				} else {
					c.emit(code.OpSetFree, symbol.Index)
				}
				c.emit(code.OpNull)
				return nil
			case *ast.InfixExpression: // Dot access like `head.next = ...`
				if left.Operator == "." {
					if ident, ok := left.Right.(*ast.Identifier); ok {
						err := c.Compile(left.Left) // Push the struct instance FIRST
						if err != nil {
							return err
						}

						err = c.Compile(n.Right) // Push the value SECOND
						if err != nil {
							return err
						}

						fieldNameIdx := c.addConstant(&object.String{Value: ident.Value})
						c.emit(code.OpSetField, fieldNameIdx)

						// OpSetField pushes the instance back.
						// If we don't emit anything else, the ExpressionStatement will OpPop the instance, leaving stack balanced.
						return nil
					}
				}
				return fmt.Errorf("invalid assignment target")
			case *ast.IndexExpression: // Array access like `arr[0] = ...`
				err := c.Compile(left.Left) // Push the array FIRST
				if err != nil {
					return err
				}

				err = c.Compile(left.Index) // Push the index SECOND
				if err != nil {
					return err
				}

				err = c.Compile(n.Right) // Push the value THIRD
				if err != nil {
					return err
				}

				c.emit(code.OpSetIndex)
				return nil
			default:
				return fmt.Errorf("invalid assignment target")
			}
		}

		// Reordering for LessThan logic
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

		if n.Operator == "<=" {
			// left <= right  -> !(left > right)
			err := c.Compile(n.Left)
			if err != nil {
				return err
			}
			err = c.Compile(n.Right)
			if err != nil {
				return err
			}
			c.emit(code.OpGreaterThan)
			c.emit(code.OpBang)
			return nil
		}

		if n.Operator == ">=" {
			// left >= right -> !(left < right) -> !(right > left)
			err := c.Compile(n.Right)
			if err != nil {
				return err
			}
			err = c.Compile(n.Left)
			if err != nil {
				return err
			}
			c.emit(code.OpGreaterThan)
			c.emit(code.OpBang)
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
		case "&":
			c.emit(code.OpBitAnd)
		case "|":
			c.emit(code.OpBitOr)
		case "^":
			c.emit(code.OpBitXor)
		case "<<":
			c.emit(code.OpLShift)
		case ">>":
			c.emit(code.OpRShift)
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
		case "~":
			c.emit(code.OpBitNot)
		default:
			return fmt.Errorf("unknown operator %s", n.Operator)
		}

	case *ast.IntegerLiteral:
		integer := &object.Integer{Value: n.Value}
		c.emit(code.OpConstant, c.addConstant(integer))

	case *ast.FloatLiteral:
		float := &object.Float{Value: n.Value}
		c.emit(code.OpConstant, c.addConstant(float))

	case *ast.StringLiteral:
		str := &object.String{Value: n.Value}
		c.emit(code.OpConstant, c.addConstant(str))

	case *ast.CharLiteral:
		char := &object.Char{Value: n.Value}
		c.emit(code.OpConstant, c.addConstant(char))

	case *ast.BooleanLiteral:
		if n.Value {
			c.emit(code.OpTrue)
		} else {
			c.emit(code.OpFalse)
		}

	case *ast.NullLiteral:
		c.emit(code.OpNull)

	case *ast.ArrayLiteral:
		for _, el := range n.Elements {
			err := c.Compile(el)
			if err != nil {
				return err
			}
		}
		c.emit(code.OpArray, len(n.Elements))

	case *ast.LetStatement:
		err := c.Compile(n.Value)
		if err != nil { return err }

		if n.Destructuring != nil {
			c.emitUnpack(n.Destructuring)
		} else {
			symbol := c.symbolTable.Define(n.Name.Value, n.Type)
			if symbol.Scope == GlobalScope {
				c.emit(code.OpSetGlobal, symbol.Index)
			} else {
				c.emit(code.OpSetLocal, symbol.Index)
			}
		}

	case *ast.ConstStatement:
		err := c.Compile(n.Value)
		if err != nil { return err }

		if n.Destructuring != nil {
			c.emitUnpack(n.Destructuring)
		} else {
			symbol := c.symbolTable.Define(n.Name.Value, n.Type)
			symbol.IsConst = true
			if symbol.Scope == GlobalScope {
				c.emit(code.OpSetGlobal, symbol.Index)
			} else {
				c.emit(code.OpSetLocal, symbol.Index)
			}
		}

	case *ast.IfExpression:
		err := c.Compile(n.Condition)
		if err != nil {
			return err
		}

		// Emit jump with placeholder
		jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

		err = c.Compile(n.Consequence)
		if err != nil {
			return err
		}
		// IfExpression consequence is always a *BlockStatement, which doesn't push a value.
		// Since IfExpression is an expression, we must push a value (null) at the end of the block.
		c.emit(code.OpNull)

		if n.Alternative == nil {
			afterConsequencePos := len(c.instructions)
			c.changeOperand(jumpNotTruthyPos, afterConsequencePos)
			// We already pushed OpNull for the consequence.
			// But for the 'false' case where there is no alternative, we need another Null.
			c.emit(code.OpNull)
		} else {
			jumpPos := c.emit(code.OpJump, 9999)

			afterConsequencePos := len(c.instructions)
			c.changeOperand(jumpNotTruthyPos, afterConsequencePos)

			err = c.Compile(n.Alternative)
			if err != nil {
				return err
			}
			// Alternative is also a *BlockStatement, push null.
			c.emit(code.OpNull)

			afterAlternativePos := len(c.instructions)
			c.changeOperand(jumpPos, afterAlternativePos)
		}
		return nil

	case *ast.WhileStatement:
		startPos := len(c.instructions)
		err := c.Compile(n.Condition)
		if err != nil {
			return err
		}

		jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

		err = c.Compile(n.Body)
		if err != nil {
			return err
		}

		c.emit(code.OpJump, startPos)

		afterWhilePos := len(c.instructions)
		c.changeOperand(jumpNotTruthyPos, afterWhilePos)
		c.emit(code.OpNull) // Ensure expression statement balanced

	case *ast.ForStatement:
		// for (init; cond; post) body
		err := c.Compile(n.Init)
		if err != nil { return err }

		startPos := len(c.instructions)
		loopInfo := &LoopInfo{StartPos: startPos}
		c.loops = append(c.loops, loopInfo)
		defer func() { c.loops = c.loops[:len(c.loops)-1] }()

		err = c.Compile(n.Condition)
		if err != nil { return err }

		jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

		err = c.Compile(n.Body)
		if err != nil { return err }

		// Continue jumps to post-statement
		postStartPos := len(c.instructions)
		for _, pos := range loopInfo.ContinueJumps {
			c.changeOperand(pos, postStartPos)
		}

		err = c.Compile(n.Post)
		if err != nil { return err }

		c.emit(code.OpJump, startPos)

		afterForPos := len(c.instructions)
		c.changeOperand(jumpNotTruthyPos, afterForPos)
		
		for _, pos := range loopInfo.BreakJumps {
			c.changeOperand(pos, afterForPos)
		}
		c.emit(code.OpNull)

	case *ast.ForEachStatement:
		// for (val in iterable) body
		// Translates to iterator pattern:
		// iter = OpIterInit(iterable)
		// loop:
		//   val, hasNext = OpIterNext(iter)
		//   if !hasNext goto end
		//   body
		//   goto loop
		// end:

		err := c.Compile(n.Iterable)
		if err != nil { return err }
		c.emit(code.OpIterInit)
		
		// The iterator is on stack. We should probably store it in a hidden local?
		// For simplicity, let's assume OpIterNext expects iterator on stack and keeps it there?
		// Or we can use a local.
		iterSym := c.symbolTable.Define("__iter_" + fmt.Sprintf("%d", len(c.instructions)))
		c.emit(code.OpSetLocal, iterSym.Index)
		
		startPos := len(c.instructions)
		loopInfo := &LoopInfo{StartPos: startPos}
		c.loops = append(c.loops, loopInfo)
		defer func() { c.loops = c.loops[:len(c.loops)-1] }()

		c.emit(code.OpGetLocal, iterSym.Index)
		c.emit(code.OpIterNext)
		
		// OpIterNext pushes: [val, hasNext]
		jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)

		// We need to bind val to the loop variable
		valSym := c.symbolTable.Define(n.Value.Value)
		if n.Key != nil {
			// If key/val loop, OpIterNext should push [key, val, hasNext]
		}
		c.emit(code.OpSetLocal, valSym.Index)

		err = c.Compile(n.Body)
		if err != nil { return err }

		// Continue jumps to loop start (which re-fetches next)
		for _, pos := range loopInfo.ContinueJumps {
			c.changeOperand(pos, startPos)
		}

		c.emit(code.OpJump, startPos)

		afterLoopPos := len(c.instructions)
		c.changeOperand(jumpNotTruthyPos, afterLoopPos)
		
		for _, pos := range loopInfo.BreakJumps {
			c.changeOperand(pos, afterLoopPos)
		}
		c.emit(code.OpNull)

	case *ast.BreakStatement:
		if len(c.loops) == 0 {
			return fmt.Errorf("break statement outside of loop")
		}
		pos := c.emit(code.OpJump, 9999)
		info := c.loops[len(c.loops)-1]
		info.BreakJumps = append(info.BreakJumps, pos)

	case *ast.ContinueStatement:
		if len(c.loops) == 0 {
			return fmt.Errorf("continue statement outside of loop")
		}
		pos := c.emit(code.OpJump, 9999)
		info := c.loops[len(c.loops)-1]
		info.ContinueJumps = append(info.ContinueJumps, pos)

	case *ast.YieldStatement:
		err := c.Compile(n.Value)
		if err != nil { return err }
		c.emit(code.OpYield)

	case *ast.BlockStatement:
		// Pre-scan pass
		for _, s := range n.Statements {
			c.preScan(s)
		}
		for _, s := range n.Statements {
			err := c.Compile(s)
			if err != nil {
				return err
			}
		}

	case *ast.ThrowStatement:
		err := c.Compile(n.Value)
		if err != nil {
			return err
		}
		c.emit(code.OpThrow)

	case *ast.TryStatement:
		jumpToCatchPos := c.emit(code.OpTry, 9999)

		err := c.Compile(n.Body)
		if err != nil {
			return err
		}

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
			if err != nil {
				return err
			}
		} else {
			c.emit(code.OpPop)
		}

		endPos := len(c.instructions)
		c.changeOperand(jumpToEndPos, endPos)

		if n.Finally != nil {
			err = c.Compile(n.Finally)
			if err != nil {
				return err
			}
		}

	case *ast.Identifier:
		symbol, ok := c.symbolTable.Resolve(n.Value)
		if !ok {
			return fmt.Errorf("undefined variable %s", n.Value)
		}

		c.loadSymbol(symbol)

	case *ast.StructLiteral:
		if n.Name != nil {
			// Build the StructLiteral object and store it in the constant pool
			fields := make([]*ast.Parameter, len(n.Fields))
			copy(fields, n.Fields)
			structDef := &object.StructLiteral{
				Name:   n.Name.Value,
				Fields: fields,
			}
			if n.Parent != nil {
				structDef.ParentName = n.Parent.Value
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
		if err != nil {
			return err
		}

		if !enclosedCompiler.lastInstructionIs(code.OpReturnValue) && !enclosedCompiler.lastInstructionIs(code.OpReturn) {
			enclosedCompiler.emit(code.OpReturn)
		}

		numParams := len(n.Parameters)
		if n.Receiver != nil {
			numParams++
		}

		var fnName string
		if n.Name != nil {
			fnName = n.Name.Value
		}

		compiledFn := &object.CompiledFunction{
			Instructions:  enclosedCompiler.instructions,
			NumLocals:     enclosedCompiler.symbolTable.numDefinitions,
			NumParameters: numParams,
			IsAsync:       n.IsAsync,
			SourceMap:     enclosedCompiler.sourceMap,
			Name:          fnName,
			Filename:      c.Filename,
		}

		if n.Receiver != nil {
			// It's a method! Attach it to the struct definition in constants.
			receiverType := n.Receiver.Type
			found := false
			for _, constant := range *c.constants {
				if sl, ok := constant.(*object.StructLiteral); ok && sl.Name == receiverType {
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
			freeSymbols := enclosedCompiler.symbolTable.FreeSymbols
			for _, s := range freeSymbols {
				c.loadSymbol(s)
			}
			c.emit(code.OpClosure, c.addConstant(compiledFn), len(freeSymbols))

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
		if err != nil {
			return err
		}

		for _, arg := range n.Arguments {
			err := c.Compile(arg)
			if err != nil {
				return err
			}
		}

		// If the callee is a StructLiteral (resolved from symbol table),
		// emit OpStructNew instead of OpCall so the VM constructs an instance.
		// We detect this by checking if the function expression is an Identifier
		// that resolves to a StructLiteral in constants — the VM handles this distinction.
		c.emit(code.OpCall, len(n.Arguments))


	case *ast.SpawnExpression:
		err := c.Compile(n.Call.Function)
		if err != nil {
			return err
		}

		for _, arg := range n.Call.Arguments {
			err := c.Compile(arg)
			if err != nil {
				return err
			}
		}

		c.emit(code.OpSpawn, len(n.Call.Arguments))

	case *ast.AwaitExpression:
		err := c.Compile(n.Expression)
		if err != nil {
			return err
		}
		c.emit(code.OpAwait)

	case *ast.LambdaExpression:
		enclosedCompiler := NewEnclosedCompiler(c)

		for _, p := range n.Parameters {
			enclosedCompiler.symbolTable.Define(p.Name.Value)
		}

		err := enclosedCompiler.Compile(n.Body)
		if err != nil { return err }

		if !enclosedCompiler.lastInstructionIs(code.OpReturnValue) && !enclosedCompiler.lastInstructionIs(code.OpReturn) {
			if _, ok := n.Body.(ast.Expression); ok {
				enclosedCompiler.emit(code.OpReturnValue)
			} else {
				enclosedCompiler.emit(code.OpReturn)
			}
		}

		compiledFn := &object.CompiledFunction{
			Instructions:  enclosedCompiler.instructions,
			NumLocals:     enclosedCompiler.symbolTable.numDefinitions,
			NumParameters: len(n.Parameters),
			Filename:      c.Filename,
		}

		freeSymbols := enclosedCompiler.symbolTable.FreeSymbols
		for _, s := range freeSymbols {
			c.loadSymbol(s)
		}
		c.emit(code.OpClosure, c.addConstant(compiledFn), len(freeSymbols))

	case *ast.IndexExpression:
		err := c.Compile(n.Left)
		if err != nil {
			return err
		}
		err = c.Compile(n.Index)
		if err != nil {
			return err
		}
		c.emit(code.OpIndex)
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
	"mapHas":        9,
	"fileRead":      10,
	"fileWrite":     11,
	"map":           12,
	"chan":          13,
	"send":          14,
	"recv":          15,
	"envGet":        16,
	"envSet":        17,
	"args":          18,
	"exit":          19,
	"sha256":        20,
	"md5":           21,
	"regexMatch":    22,
	"regexReplace":  23,
	"osMkdir":       24,
	"osRmdir":       25,
	"osRemove":      26,
	"osRename":      27,
	"osListdir":     28,
	"osExists":      29,
	"osIsdir":       30,
	"osIsfile":      31,
	"osGetcwd":      32,
	"osChdir":       33,
	"osGetpid":      34,
	"pathJoin":      35,
	"pathBase":      36,
	"pathDir":       37,
	"fileOpen":      38,
	"fileClose":     39,
	"fRead":         40,
	"fWrite":        41,
	"fileSeek":      42,
	"instanceOf":    43,
	"charAt":        44,
	"toInt":         45,
	"toFloat":       46,
	"mathSin":       47,
	"mathCos":       48,
	"mathTan":       49,
	"mathSqrt":      50,
	"mathPow":       51,
	"mathLog":       52,
	"mathLog10":     53,
	"mathExp":       54,
	"mathAsin":      55,
	"mathAcos":      56,
	"mathAtan":      57,
	"mathAtan2":     58,
	"mathAbs":       59,
	"mathCeil":      60,
	"mathFloor":     61,
	"strToLower":    62,
	"strToUpper":    63,
	"strTrim":       64,
	"strTrimSpace":  65,
	"strSplit":      66,
	"strJoin":       67,
	"strContains":   68,
	"strHasPrefix":  69,
	"strHasSuffix":  70,
	"strIndex":      71,
	"strLastIndex":  72,
	"strReplace":    73,
	"strRepeat":     74,
	"strCount":      75,
	"strFields":     76,
	"strTrimLeft":   77,
	"strTrimRight":  78,
	"strIsAlpha":    79,
	"strIsDigit":    80,
	"strIsSpace":    81,
	"strReverse":    82,
	"ioReadInput":   83,
	"arrayPush":     84,
	"arrayPop":      85,
	"arraySlice":    86,
	"arraySort":     87,
	"mapDelete":     88,
	"mapKeys":       89,
	"mapValues":     90,
	"timeParse":     91,
	"strReplaceAll": 92,
	"timeAdd":       93,
	"timeSub":       94,
	"timeDiff":      95,
	"timeInLocation": 96,
	"typeof":         97,
	"toChar":         98,
	"toString":       99,
	"generator":      100,
	"fileAppend":     101,
	"fileExists":     102,
	"mathRand":       103,
	"httpHandle":     104,
	"httpServe":      105,
	"httpGet":        106,
	"httpPost":       107,
	"httpResponse":   108,
	"int":           45,
	"float":         46,
	"string":        99,
	"char":          98,
}

func (c *Compiler) Bytecode() *Bytecode {
	return &Bytecode{
		Instructions: c.instructions,
		Constants:    *c.constants,
		SourceMap:    c.sourceMap,
		Filename:     c.Filename,
		NumLocals:    c.symbolTable.numDefinitions,
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

	if c.currentNode != nil {
		// Try to extract line number from node. Many ast nodes have a Token field.
		// We use reflection or just check known types.
		line := c.extractLine(c.currentNode)
		if line > 0 {
			c.sourceMap[pos] = line
		}
	}

	return pos
}

func (c *Compiler) extractLine(node ast.Node) int {
	switch n := node.(type) {
	case *ast.LetStatement: return n.Token.Line
	case *ast.ConstStatement: return n.Token.Line
	case *ast.ReturnStatement: return n.Token.Line
	case *ast.ExpressionStatement: return n.Token.Line
	case *ast.InfixExpression: return n.Token.Line
	case *ast.PrefixExpression: return n.Token.Line
	case *ast.IntegerLiteral: return n.Token.Line
	case *ast.FloatLiteral: return n.Token.Line
	case *ast.StringLiteral: return n.Token.Line
	case *ast.CharLiteral: return n.Token.Line
	case *ast.BooleanLiteral: return n.Token.Line
	case *ast.ArrayLiteral: return n.Token.Line
	case *ast.Identifier: return n.Token.Line
	case *ast.IfExpression: return n.Token.Line
	case *ast.WhileStatement: return n.Token.Line
	case *ast.CallExpression: return n.Token.Line
	case *ast.FunctionLiteral: return n.Token.Line
	case *ast.StructLiteral: return n.Token.Line
	case *ast.TryStatement: return n.Token.Line
	case *ast.ThrowStatement: return n.Token.Line
	case *ast.SpawnExpression: return n.Token.Line
	case *ast.AwaitExpression: return n.Token.Line
	case *ast.IndexExpression: return n.Token.Line
	}
	return 0
}

func (c *Compiler) preScan(stmt ast.Statement) {
	switch s := stmt.(type) {
	case *ast.ExpressionStatement:
		switch expr := s.Expression.(type) {
		case *ast.FunctionLiteral:
			if expr.Name != nil {
				c.symbolTable.Define(expr.Name.Value)
			}
		case *ast.StructLiteral:
			if expr.Name != nil {
				c.symbolTable.Define(expr.Name.Value)
			}
		}
	case *ast.InterfaceStatement:
		c.symbolTable.Define(s.Name.Value)
	case *ast.EnumStatement:
		c.symbolTable.Define(s.Name.Value)
	}
}

func (c *Compiler) lastInstructionIs(op code.Opcode) bool {
	if len(c.instructions) == 0 {
		return false
	}
	return c.lastInstruction.Opcode == op
}

func (c *Compiler) loadSymbol(s Symbol) {
	switch s.Scope {
	case GlobalScope:
		c.emit(code.OpGetGlobal, s.Index)
	case LocalScope:
		c.emit(code.OpGetLocal, s.Index)
	case BuiltinScope:
		c.emit(code.OpGetBuiltin, s.Index)
	case FreeScope:
		c.emit(code.OpGetFree, s.Index)
	}
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
func (c *Compiler) emitUnpack(pattern ast.Expression) {
	switch p := pattern.(type) {
	case *ast.Identifier:
		symbol := c.symbolTable.Define(p.Value, "any")
		if symbol.Scope == GlobalScope {
			c.emit(code.OpSetGlobal, symbol.Index)
		} else {
			c.emit(code.OpSetLocal, symbol.Index)
		}
	case *ast.ArrayLiteral:
		for i, el := range p.Elements {
			c.emit(code.OpDup)
			c.emit(code.OpConstant, c.addConstant(&object.Integer{Value: int64(i)}))
			c.emit(code.OpIndex)
			c.emitUnpack(el)
		}
		c.emit(code.OpPop) // pop the duplicated array
	}
}
