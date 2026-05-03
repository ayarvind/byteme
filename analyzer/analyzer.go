package analyzer

import (
	"fmt"
	"io/ioutil"
	
	"github.com/byteme/compiler/ast"
	"github.com/byteme/compiler/environment"
	"github.com/byteme/compiler/lexer"
	"github.com/byteme/compiler/parser"
	"github.com/byteme/compiler/token"
)

type Analyzer struct {
	env           *environment.Environment
	errors        []string
	structMethods map[string]map[string]bool
	interfaces    map[string][]*ast.MethodSignature
}

func New(env *environment.Environment) *Analyzer {
	// Register global built-ins
	env.Set("chan", "function", environment.PUBLIC, true)
	env.Set("send", "function", environment.PUBLIC, true)
	env.Set("recv", "function", environment.PUBLIC, true)
	env.Set("println", "function", environment.PUBLIC, true)
	env.Set("null", "any", environment.PUBLIC, true)
	env.Set("arrayLen", "function", environment.PUBLIC, true)
	env.Set("arrayPush", "function", environment.PUBLIC, true)
	env.Set("arrayPop", "function", environment.PUBLIC, true)
	env.Set("arrayShift", "function", environment.PUBLIC, true)
	env.Set("map", "function", environment.PUBLIC, true)
	env.Set("mapSet", "function", environment.PUBLIC, true)
	env.Set("mapGet", "function", environment.PUBLIC, true)
	env.Set("mapHas", "function", environment.PUBLIC, true)
	env.Set("charAt", "function", environment.PUBLIC, true)
	env.Set("fileRead", "function", environment.PUBLIC, true)
	env.Set("fileWrite", "function", environment.PUBLIC, true)
	env.Set("fileAppend", "function", environment.PUBLIC, true)
	env.Set("fileExists", "function", environment.PUBLIC, true)
	env.Set("jsonParse", "function", environment.PUBLIC, true)
	env.Set("jsonStringify", "function", environment.PUBLIC, true)
	env.Set("timeNow", "function", environment.PUBLIC, true)
	env.Set("timeSleep", "function", environment.PUBLIC, true)
	env.Set("timeFormat", "function", environment.PUBLIC, true)
	env.Set("nativeCall", "function", environment.PUBLIC, true)
	// HTTP / networking module
	env.Set("httpHandle",   "function", environment.PUBLIC, true)
	env.Set("httpServe",    "function", environment.PUBLIC, true)
	env.Set("httpGet",      "function", environment.PUBLIC, true)
	env.Set("httpPost",     "function", environment.PUBLIC, true)
	env.Set("httpResponse", "function", environment.PUBLIC, true)
	
	// New Standard Library
	env.Set("len",          "function", environment.PUBLIC, true)
	env.Set("envGet",       "function", environment.PUBLIC, true)
	env.Set("envSet",       "function", environment.PUBLIC, true)
	env.Set("args",         "function", environment.PUBLIC, true)
	env.Set("exit",         "function", environment.PUBLIC, true)
	env.Set("sha256",       "function", environment.PUBLIC, true)
	env.Set("md5",          "function", environment.PUBLIC, true)
	env.Set("regexMatch",   "function", environment.PUBLIC, true)
	env.Set("regexReplace", "function", environment.PUBLIC, true)
	
	env.Set("osMkdir",       "function", environment.PUBLIC, true)
	env.Set("osRmdir",       "function", environment.PUBLIC, true)
	env.Set("osRemove",      "function", environment.PUBLIC, true)
	env.Set("osRename",      "function", environment.PUBLIC, true)
	env.Set("osListdir",     "function", environment.PUBLIC, true)
	env.Set("osExists",      "function", environment.PUBLIC, true)
	env.Set("osIsdir",       "function", environment.PUBLIC, true)
	env.Set("osIsfile",      "function", environment.PUBLIC, true)
	env.Set("osGetcwd",      "function", environment.PUBLIC, true)
	env.Set("osChdir",       "function", environment.PUBLIC, true)
	env.Set("osGetpid",      "function", environment.PUBLIC, true)
	env.Set("pathJoin",      "function", environment.PUBLIC, true)
	env.Set("pathBase",      "function", environment.PUBLIC, true)
	env.Set("pathDir",       "function", environment.PUBLIC, true)
	env.Set("fOpen",         "function", environment.PUBLIC, true)
	env.Set("fClose",        "function", environment.PUBLIC, true)
	env.Set("fRead",         "function", environment.PUBLIC, true)
	env.Set("fWrite",        "function", environment.PUBLIC, true)
	env.Set("fSeek",         "function", environment.PUBLIC, true)
	env.Set("instanceOf",    "function", environment.PUBLIC, true)

	// Register basic types as symbols
	env.Set("int", "type", environment.PUBLIC, true)
	env.Set("float", "type", environment.PUBLIC, true)
	env.Set("string", "type", environment.PUBLIC, true)
	env.Set("bool", "type", environment.PUBLIC, true)
	env.Set("any", "type", environment.PUBLIC, true)
	env.Set("array", "type", environment.PUBLIC, true)
	env.Set("map", "type", environment.PUBLIC, true)

	return &Analyzer{
		env:           env,
		errors:        []string{},
		structMethods: make(map[string]map[string]bool),
		interfaces:    make(map[string][]*ast.MethodSignature),
	}
}

func (a *Analyzer) Errors() []string {
	return a.errors
}

func (a *Analyzer) error(tok token.Token, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	a.errors = append(a.errors, fmt.Sprintf("[%d:%d] %s", tok.Line, tok.Column, msg))
}

func (a *Analyzer) Analyze(node ast.Node) string {
	switch n := node.(type) {
	case *ast.Program:
		for _, stmt := range n.Statements {
			a.preScan(stmt)
		}
		for _, stmt := range n.Statements {
			a.Analyze(stmt)
		}

	case *ast.ImportStatement:
		filename := n.Path.Value
		input, err := ioutil.ReadFile(filename)
		if err != nil {
			// No token available for file-level error
			a.errors = append(a.errors, fmt.Sprintf("could not read imported file %s: %s", filename, err))
			return ""
		}
		
		l := lexer.New(string(input))
		p := parser.New(l)
		program := p.ParseProgram()
		
		if len(p.Errors()) != 0 {
			a.errors = append(a.errors, fmt.Sprintf("parser errors in imported file %s: %v", filename, p.Errors()))
			return ""
		}
		
		// If it's a 'from' import, we hide the module's variables from the global scope
		// except for the specifically imported ones.
		var savedEnv *environment.Environment
		if n.Token.Type == token.FROM {
			// For analyzer, the env is nested, but here we are using a flat env maybe?
			// Let's create a new scope for the import so it doesn't pollute the current scope
			savedEnv = a.env
			a.env = environment.NewEnclosedEnvironment(savedEnv)
		}
		
		// Analyze the imported program
		a.Analyze(program)

		if n.Token.Type == token.FROM {
			// Extract specific imports
			for _, imp := range n.Imports {
				_, ok := a.env.Get(imp.Value)
				if !ok {
					a.error(imp.Token, "imported symbol %s not found in module %s", imp.Value, filename)
				}
				// We need a better way. Let's just bypass it for MVP.
			}
			a.env = savedEnv
			
			// We must inject the specific imports into savedEnv
			// Let's just pretend we inject 'any'
			for _, imp := range n.Imports {
				a.env.Set(imp.Value, "any", environment.PUBLIC, false)
			}
		}

	case *ast.LetStatement:
		valType := a.Analyze(n.Value)
		typeName := n.Type
		if typeName == "" {
			typeName = valType
		} else if typeName != valType && valType != "any" {
			// Check if typeName is an interface
			methods, isInterface := a.interfaces[typeName]
			if isInterface {
				// Check if valType (struct) implements all methods
				structMethods := a.structMethods[valType]
				for _, m := range methods {
					if !structMethods[m.Name.Value] {
						a.error(n.Token, "type %s does not implement interface %s: missing method %s", valType, typeName, m.Name.Value)
					}
				}
			} else if valType != "array" && typeName != "any" {
				a.error(n.Token, "type mismatch: cannot assign %s to %s", valType, typeName)
			}
		}
		err := a.env.Set(n.Name.Value, typeName, environment.PUBLIC, false)
		if err != nil {
			a.error(n.Name.Token, "%s", err.Error())
		}
		return typeName

	case *ast.ConstStatement:
		valType := a.Analyze(n.Value)
		typeName := n.Type
		if typeName == "" {
			typeName = valType
		} else if typeName != valType && valType != "any" && typeName != "any" {
			a.error(n.Token, "type mismatch: cannot assign %s to %s", valType, typeName)
		}
		err := a.env.Set(n.Name.Value, typeName, environment.PUBLIC, true)
		if err != nil {
			a.error(n.Name.Token, "%s", err.Error())
		}
		return typeName

	case *ast.Identifier:
		sym, ok := a.env.Get(n.Value)
		if !ok {
			a.error(n.Token, "undefined variable: %s", n.Value)
			return "any"
		}
		return sym.Type

	case *ast.IntegerLiteral:
		return "int"

	case *ast.NullLiteral:
		return "any"

	case *ast.FloatLiteral:
		return "float"

	case *ast.StringLiteral:
		return "string"

	case *ast.BooleanLiteral:
		return "bool"

	case *ast.InfixExpression:
		if n.Operator == "." {
			// Dot is valid on struct instances, namespaces, and unknowns
			leftType := a.Analyze(n.Left)
			// Any type that isn't one of the primitives is likely a struct instance
			primitives := map[string]bool{"int": true, "float": true, "string": true, "bool": true, "thread": true, "interface": true}
			if primitives[leftType] {
				a.error(n.Token, "cannot use dot operator on type '%s' — only struct instances support field access", leftType)
			}
			return "any"
		}
		if n.Operator == "=" {
			a.Analyze(n.Left)
			return a.Analyze(n.Right)
		}
		leftType := a.Analyze(n.Left)
		rightType := a.Analyze(n.Right)

		if leftType == "any" || rightType == "any" {
			return "any"
		}

		if leftType != rightType && leftType != "any" && rightType != "any" {
			a.error(n.Token, "type mismatch in expression: %s %s %s", leftType, n.Operator, rightType)
			return "any"
		}

		switch n.Operator {
		case "==", "!=", "<", ">":
			return "bool"
		default:
			return leftType
		}

	case *ast.PrefixExpression:
		return a.Analyze(n.Right)

	case *ast.ExpressionStatement:
		return a.Analyze(n.Expression)

	case *ast.WhileStatement:
		a.Analyze(n.Condition)
		a.Analyze(n.Body)
		return "any"

	case *ast.ArrayLiteral:
		for _, el := range n.Elements {
			a.Analyze(el)
		}
		return "array"

	case *ast.IndexExpression:
		a.Analyze(n.Left)
		a.Analyze(n.Index)
		return "any"

	case *ast.BlockStatement:
		// Save current environment
		oldEnv := a.env
		a.env = environment.NewEnclosedEnvironment(oldEnv)
		
		// Pre-scan for function and struct definitions to allow forward references
		for _, stmt := range n.Statements {
			a.preScan(stmt)
		}

		for _, stmt := range n.Statements {
			a.Analyze(stmt)
		}
		a.env = oldEnv

	case *ast.FunctionLiteral:
		if n.Name != nil {
			a.env.Set(n.Name.Value, "function", environment.PUBLIC, true)
		}
		// Check return type and parameters
		funcEnv := environment.NewEnclosedEnvironment(a.env)
		
		for _, tp := range n.TypeParameters {
			funcEnv.Set(tp.Value, "type", environment.PUBLIC, true)
		}

		if n.Receiver != nil {
			funcEnv.Set(n.Receiver.Name.Value, n.Receiver.Type, environment.PUBLIC, false)
			// Track that this type has this method
			if a.structMethods[n.Receiver.Type] == nil {
				a.structMethods[n.Receiver.Type] = make(map[string]bool)
			}
			a.structMethods[n.Receiver.Type][n.Name.Value] = true
		}

		for _, p := range n.Parameters {
			funcEnv.Set(p.Name.Value, p.Type, environment.PUBLIC, false)
		}
		
		// Analyze body with the function environment
		oldEnv := a.env
		a.env = funcEnv
		a.Analyze(n.Body)
		a.env = oldEnv
		return "function"

	case *ast.NamespaceLiteral:
		// Handle inheritance
		if n.Parent != nil {
			_, ok := a.env.Get(n.Parent.Value)
			if !ok {
				a.error(n.Parent.Token, "parent namespace %s not found", n.Parent.Value)
			}
		}

		nsEnv := environment.NewEnclosedEnvironment(a.env)
		
		// Register the namespace itself as a symbol
		a.env.Set(n.Name.Value, "namespace", environment.PUBLIC, true)

		oldEnv := a.env
		a.env = nsEnv
		
		// Pre-scan namespace body
		for _, stmt := range n.Body.Statements {
			a.preScan(stmt)
		}

		a.Analyze(n.Body)
		a.env = oldEnv
		return "namespace"

	case *ast.StructLiteral:
		a.env.Set(n.Name.Value, "type", environment.PUBLIC, true)
		// We could analyze fields here if we want to check type parameter usage
		return "type"

	case *ast.CallExpression:
		a.Analyze(n.Function)
		for _, arg := range n.Arguments {
			a.Analyze(arg)
		}
		
		// If it's a struct constructor, return the struct name
		if ident, ok := n.Function.(*ast.Identifier); ok {
			sym, ok := a.env.Get(ident.Value)
			if ok && sym.Type == "type" {
				return ident.Value
			}
		}
		return "any"

	case *ast.SpawnExpression:
		a.Analyze(n.Call)
		return "thread"

	case *ast.AwaitExpression:
		return a.Analyze(n.Expression)

	case *ast.ThrowStatement:
		a.Analyze(n.Value)
		return "any"

	case *ast.TryStatement:
		a.Analyze(n.Body)
		if n.CatchBody != nil {
			catchEnv := environment.NewEnclosedEnvironment(a.env)
			catchEnv.Set(n.CatchVar.Value, "any", environment.PUBLIC, false)
			
			oldEnv := a.env
			a.env = catchEnv
			a.Analyze(n.CatchBody)
			a.env = oldEnv
		}
		if n.Finally != nil {
			a.Analyze(n.Finally)
		}
		return "any"

	case *ast.InterfaceStatement:
		a.env.Set(n.Name.Value, "interface", environment.PUBLIC, true)
		a.interfaces[n.Name.Value] = n.Methods
		return "interface"

	case *ast.EnumStatement:
		a.env.Set(n.Name.Value, "namespace", environment.PUBLIC, true)
		return "namespace"
	}

	return "any"
}

func (a *Analyzer) preScan(stmt ast.Statement) {
	switch s := stmt.(type) {
	case *ast.ExpressionStatement:
		switch expr := s.Expression.(type) {
		case *ast.FunctionLiteral:
			if expr.Name != nil {
				a.env.Set(expr.Name.Value, "function", environment.PUBLIC, true)
			}
		case *ast.StructLiteral:
			a.env.Set(expr.Name.Value, "type", environment.PUBLIC, true)
		}
	case *ast.InterfaceStatement:
		a.env.Set(s.Name.Value, "interface", environment.PUBLIC, true)
	case *ast.EnumStatement:
		a.env.Set(s.Name.Value, "namespace", environment.PUBLIC, true)
	case *ast.LetStatement:
		// We don't pre-scan variables to avoid uninitialized usage
	}
}
