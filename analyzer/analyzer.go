package analyzer

import (
	"fmt"
	"io/ioutil"
	
	"github.com/byteme/compiler/ast"
	"github.com/byteme/compiler/environment"
	"github.com/byteme/compiler/lexer"
	"github.com/byteme/compiler/parser"
	"github.com/byteme/compiler/token"
	"strings"
)
 
type FunctionSignature struct {
	Params []string
	Return string
}

type Analyzer struct {
	env               *environment.Environment
	errors            []string
	structFields      map[string]map[string]string
	structMethods     map[string]map[string]FunctionSignature
	structParents     map[string]string
	interfaces        map[string][]*ast.MethodSignature
	currentReturnType string
	funcSignatures    map[string]FunctionSignature
	source            string
	filename          string
	lines             []string
}

func New(env *environment.Environment, source string, filename string) *Analyzer {
	// Register basic types as symbols
	env.Set("int", "type", environment.PUBLIC, true)
	env.Set("float", "type", environment.PUBLIC, true)
	env.Set("string", "type", environment.PUBLIC, true)
	env.Set("char", "type", environment.PUBLIC, true)
	env.Set("bool", "type", environment.PUBLIC, true)
	env.Set("any", "type", environment.PUBLIC, true)
	env.Set("array", "type", environment.PUBLIC, true)
	env.Set("map", "type", environment.PUBLIC, true)

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
	// env.Set("map", "function", environment.PUBLIC, true) // Collides with map type
	env.Set("mapSet", "function", environment.PUBLIC, true)
	env.Set("mapGet", "function", environment.PUBLIC, true)
	env.Set("mapHas", "function", environment.PUBLIC, true)
	env.Set("charAt", "function", environment.PUBLIC, true)
	env.Set("arraySlice", "function", environment.PUBLIC, true)
	env.Set("arraySort", "function", environment.PUBLIC, true)
	env.Set("mapDelete", "function", environment.PUBLIC, true)
	env.Set("mapKeys", "function", environment.PUBLIC, true)
	env.Set("mapValues", "function", environment.PUBLIC, true)
	env.Set("timeParse", "function", environment.PUBLIC, true)
	env.Set("strReplaceAll", "function", environment.PUBLIC, true)
	env.Set("timeAdd", "function", environment.PUBLIC, true)
	env.Set("timeSub", "function", environment.PUBLIC, true)
	env.Set("timeDiff", "function", environment.PUBLIC, true)
	env.Set("timeInLocation", "function", environment.PUBLIC, true)
	env.Set("ioReadInput", "function", environment.PUBLIC, true)
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
	
	env.Set("toInt",        "function", environment.PUBLIC, true)
	env.Set("toFloat",      "function", environment.PUBLIC, true)
	env.Set("toChar",       "function", environment.PUBLIC, true)
	env.Set("toString",     "function", environment.PUBLIC, true)
	env.Set("typeof",       "function", environment.PUBLIC, true)
	env.Set("len",          "function", environment.PUBLIC, true)
	env.Set("envGet",       "function", environment.PUBLIC, true)
	env.Set("envSet",       "function", environment.PUBLIC, true)
	env.Set("args",         "function", environment.PUBLIC, true)
	env.Set("exit",         "function", environment.PUBLIC, true)
	env.Set("sha256",       "function", environment.PUBLIC, true)
	env.Set("md5",          "function", environment.PUBLIC, true)
	env.Set("regexMatch",   "function", environment.PUBLIC, true)
	env.Set("regexReplace", "function", environment.PUBLIC, true)
	
	// Math Built-ins
	env.Set("mathSin", "function", environment.PUBLIC, true)
	env.Set("mathCos", "function", environment.PUBLIC, true)
	env.Set("mathTan", "function", environment.PUBLIC, true)
	env.Set("mathSqrt", "function", environment.PUBLIC, true)
	env.Set("mathPow", "function", environment.PUBLIC, true)
	env.Set("mathLog", "function", environment.PUBLIC, true)
	env.Set("mathLog10", "function", environment.PUBLIC, true)
	env.Set("mathExp", "function", environment.PUBLIC, true)
	env.Set("mathAsin", "function", environment.PUBLIC, true)
	env.Set("mathAcos", "function", environment.PUBLIC, true)
	env.Set("mathAtan", "function", environment.PUBLIC, true)
	env.Set("mathAtan2", "function", environment.PUBLIC, true)
	env.Set("mathAbs", "function", environment.PUBLIC, true)
	env.Set("mathCeil", "function", environment.PUBLIC, true)
	env.Set("mathFloor", "function", environment.PUBLIC, true)

	// String Built-ins
	env.Set("strToLower", "function", environment.PUBLIC, true)
	env.Set("strToUpper", "function", environment.PUBLIC, true)
	env.Set("strTrim", "function", environment.PUBLIC, true)
	env.Set("strTrimSpace", "function", environment.PUBLIC, true)
	env.Set("strSplit", "function", environment.PUBLIC, true)
	env.Set("strJoin", "function", environment.PUBLIC, true)
	env.Set("strContains", "function", environment.PUBLIC, true)
	env.Set("strHasPrefix", "function", environment.PUBLIC, true)
	env.Set("strHasSuffix", "function", environment.PUBLIC, true)
	env.Set("strIndex", "function", environment.PUBLIC, true)
	env.Set("strLastIndex", "function", environment.PUBLIC, true)
	env.Set("strReplace", "function", environment.PUBLIC, true)
	env.Set("strRepeat", "function", environment.PUBLIC, true)
	env.Set("strCount", "function", environment.PUBLIC, true)
	env.Set("strFields", "function", environment.PUBLIC, true)
	env.Set("strTrimLeft", "function", environment.PUBLIC, true)
	env.Set("strTrimRight", "function", environment.PUBLIC, true)
	env.Set("strIsAlpha", "function", environment.PUBLIC, true)
	env.Set("strIsDigit", "function", environment.PUBLIC, true)
	env.Set("strIsSpace", "function", environment.PUBLIC, true)
	env.Set("strReverse", "function", environment.PUBLIC, true)
	
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
	env.Set("generator",     "function", environment.PUBLIC, true)

	a := &Analyzer{
		env:               env,
		errors:            []string{},
		structFields:      make(map[string]map[string]string),
		structMethods:     make(map[string]map[string]FunctionSignature),
		structParents:     make(map[string]string),
		interfaces:        make(map[string][]*ast.MethodSignature),
		currentReturnType: "",
		funcSignatures:    make(map[string]FunctionSignature),
		source:            source,
		filename:          filename,
		lines:             strings.Split(source, "\n"),
	}
	
	// Register built-in signatures
	a.funcSignatures["len"] = FunctionSignature{Params: []string{"any"}, Return: "int"}
	a.funcSignatures["println"] = FunctionSignature{Params: []string{"any"}, Return: "void"}
	a.funcSignatures["mapSet"] = FunctionSignature{Params: []string{"map", "any", "any"}, Return: "void"}
	a.funcSignatures["mapGet"] = FunctionSignature{Params: []string{"map", "any"}, Return: "any"}
	a.funcSignatures["mapHas"] = FunctionSignature{Params: []string{"map", "any"}, Return: "bool"}
	a.funcSignatures["arrayLen"] = FunctionSignature{Params: []string{"array"}, Return: "int"}
	a.funcSignatures["arrayPush"] = FunctionSignature{Params: []string{"array", "any"}, Return: "void"}
	a.funcSignatures["generator"] = FunctionSignature{Params: []string{}, Return: "Iterator"}
	
	return a
}

func (a *Analyzer) Errors() []string {
	return a.errors
}

func (a *Analyzer) error(tok token.Token, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	
	loc := fmt.Sprintf("%s:%d:%d", a.filename, tok.Line, tok.Column)
	fullMsg := fmt.Sprintf("[%s] %s", loc, msg)

	// Add code snippet
	if tok.Line > 0 && tok.Line <= len(a.lines) {
		line := a.lines[tok.Line-1]
		
		// Create highlight pointer
		pointer := ""
		for i := 1; i < tok.Column; i++ {
			if i-1 < len(line) && line[i-1] == '\t' {
				pointer += "\t"
			} else {
				pointer += " "
			}
		}
		pointer += "^"

		snippet := fmt.Sprintf("\n  %d | %s\n      | %s", tok.Line, line, pointer)
		fullMsg += snippet
	}

	a.errors = append(a.errors, fullMsg)
}

func (a *Analyzer) isAssignable(target, source string) bool {
	if target == "any" || source == "any" || target == source {
		return true
	}

	// Union types (target)
	if strings.Contains(target, "|") {
		targets := strings.Split(target, "|")
		for _, t := range targets {
			if a.isAssignable(strings.TrimSpace(t), source) {
				return true
			}
		}
		return false
	}

	// Union types (source)
	if strings.Contains(source, "|") {
		sources := strings.Split(source, "|")
		for _, s := range sources {
			if !a.isAssignable(target, strings.TrimSpace(s)) {
				return false
			}
		}
		return true
	}

	// Inheritance
	curr := source
	for curr != "" {
		parent, ok := a.structParents[curr]
		if !ok {
			break
		}
		if parent == target {
			return true
		}
		curr = parent
	}

	// Implicit conversions
	if target == "float" && source == "int" {
		return true
	}
	// Interface implementation
	methods, isInterface := a.interfaces[target]
	if isInterface {
		structMethods := a.structMethods[source]
		for _, m := range methods {
			if _, ok := structMethods[m.Name.Value]; !ok {
				return false
			}
		}
		return true
	}
	return false
}

func (a *Analyzer) isTerminated(stmt ast.Statement) bool {
	switch s := stmt.(type) {
	case *ast.ReturnStatement, *ast.ThrowStatement:
		return true
	case *ast.BlockStatement:
		var terminated bool
		for _, stmt := range s.Statements {
			if terminated {
				a.error(stmt.GetToken(), "unreachable code detected")
				break
			}
			a.Analyze(stmt)
			if a.isTerminated(stmt) {
				terminated = true
			}
		}
		return terminated
	case *ast.ExpressionStatement:
		if ifExp, ok := s.Expression.(*ast.IfExpression); ok {
			if ifExp.Alternative == nil {
				return false
			}
			return a.isTerminated(ifExp.Consequence) && a.isTerminated(ifExp.Alternative)
		}
	}
	return false
}

func (a *Analyzer) lookupField(typeName, fieldName string) (string, bool) {
	// Handle unions: field must exist in ALL types
	if strings.Contains(typeName, "|") {
		types := strings.Split(typeName, "|")
		var commonType string
		for i, t := range types {
			t = strings.TrimSpace(t)
			fType, ok := a.lookupField(t, fieldName)
			if !ok {
				return "", false
			}
			if i == 0 {
				commonType = fType
			} else if commonType != fType {
				commonType = "any" // Mixed types in union field
			}
		}
		return commonType, true
	}

	fields, ok := a.structFields[typeName]
	if ok {
		if fType, ok := fields[fieldName]; ok {
			return fType, true
		}
	}

	parent, ok := a.structParents[typeName]
	if ok {
		return a.lookupField(parent, fieldName)
	}

	return "", false
}

func (a *Analyzer) lookupMethod(typeName, methodName string) (FunctionSignature, bool) {
	// Handle unions: method must exist in ALL types
	if strings.Contains(typeName, "|") {
		types := strings.Split(typeName, "|")
		var commonSig FunctionSignature
		for i, t := range types {
			t = strings.TrimSpace(t)
			sig, ok := a.lookupMethod(t, methodName)
			if !ok {
				return FunctionSignature{}, false
			}
			if i == 0 {
				commonSig = sig
			}
			// We could check sig compatibility here, but let's keep it simple
		}
		return commonSig, true
	}

	methods, ok := a.structMethods[typeName]
	if ok {
		if sig, ok := methods[methodName]; ok {
			return sig, true
		}
	}

	parent, ok := a.structParents[typeName]
	if ok {
		return a.lookupMethod(parent, methodName)
	}

	return FunctionSignature{}, false
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
		typeName := n.Type
		if typeName == "" {
			if n.Value == nil {
				a.error(n.Token, "variable declaration without type requires immediate initialization")
				return "any"
			}
			typeName = a.Analyze(n.Value)
		} else if n.Value != nil {
			valType := a.Analyze(n.Value)
			if !a.isAssignable(typeName, valType) {
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
		} else if !a.isAssignable(typeName, valType) {
			a.error(n.Token, "type mismatch: cannot assign %s to constant of type %s", valType, typeName)
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

	case *ast.CharLiteral:
		return "char"

	case *ast.BooleanLiteral:
		return "bool"

	case *ast.InfixExpression:
		if n.Operator == "." {
			leftType := a.Analyze(n.Left)
			
			rightIdent, isIdent := n.Right.(*ast.Identifier)
			if isIdent {
				// Check fields
				if fieldType, ok := a.lookupField(leftType, rightIdent.Value); ok {
					return fieldType
				}
				// Check methods
				if _, ok := a.lookupMethod(leftType, rightIdent.Value); ok {
					return "function"
				}

				if leftType != "any" && !strings.Contains(leftType, "|") {
					a.error(n.Token, "type %s has no field or method %s", leftType, rightIdent.Value)
				} else if strings.Contains(leftType, "|") {
					a.error(n.Token, "field or method %s is not common to all types in union %s", rightIdent.Value, leftType)
				}
				return "any"
			}
			return "any"
		}
		if n.Operator == "=" {
			leftType := a.Analyze(n.Left)
			rightType := a.Analyze(n.Right)
			if !a.isAssignable(leftType, rightType) {
				a.error(n.Token, "type mismatch in assignment: cannot assign %s to %s", rightType, leftType)
			}

			// Check for constant reassignment
			if ident, ok := n.Left.(*ast.Identifier); ok {
				if sym, ok := a.env.Get(ident.Value); ok {
					if sym.IsConst {
						a.error(n.Token, "cannot reassign to constant variable: %s", ident.Value)
					}
				}
			}
			return rightType
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
		case "==", "!=", "<", ">", "<=", ">=":
			return "bool"
		default:
			return leftType
		}

	case *ast.PrefixExpression:
		return a.Analyze(n.Right)

	case *ast.ExpressionStatement:
		return a.Analyze(n.Expression)

	case *ast.WhileStatement:
		condType := a.Analyze(n.Condition)
		if condType != "bool" && condType != "any" {
			a.error(n.Token, "while condition must be bool, got %s", condType)
		}
		a.Analyze(n.Body)
		return "any"

	case *ast.IfExpression:
		condType := a.Analyze(n.Condition)
		if condType != "bool" && condType != "any" {
			a.error(n.Token, "if condition must be bool, got %s", condType)
		}
		a.Analyze(n.Consequence)
		if n.Alternative != nil {
			a.Analyze(n.Alternative)
		}
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

		var terminated bool
		for _, stmt := range n.Statements {
			if terminated {
				a.error(stmt.GetToken(), "unreachable code detected")
				break
			}
			a.Analyze(stmt)
			if a.isTerminated(stmt) {
				terminated = true
			}
		}
		a.env = oldEnv
		return "any"

	case *ast.FunctionLiteral:
		if n.Name != nil {
			a.env.Set(n.Name.Value, "function", environment.PUBLIC, true)
			
			sig := FunctionSignature{Return: n.ReturnType}
			for _, p := range n.Parameters {
				sig.Params = append(sig.Params, p.Type)
			}
			a.funcSignatures[n.Name.Value] = sig
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
				a.structMethods[n.Receiver.Type] = make(map[string]FunctionSignature)
			}
			sig := FunctionSignature{Return: n.ReturnType}
			for _, p := range n.Parameters {
				sig.Params = append(sig.Params, p.Type)
			}
			a.structMethods[n.Receiver.Type][n.Name.Value] = sig
		}

		for _, p := range n.Parameters {
			funcEnv.Set(p.Name.Value, p.Type, environment.PUBLIC, false)
		}
		
		// Analyze body with the function environment
		oldEnv := a.env
		a.env = funcEnv
		
		oldRet := a.currentReturnType
		a.currentReturnType = n.ReturnType
		
		a.Analyze(n.Body)

		// Exhaustive return check
		if n.ReturnType != "any" && n.ReturnType != "" && n.ReturnType != "void" {
			if !a.isTerminated(n.Body) {
				a.error(n.Token, "missing return at end of function: expected %s", n.ReturnType)
			}
		}
		
		a.currentReturnType = oldRet
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
		if n.Parent != nil {
			a.structParents[n.Name.Value] = n.Parent.Value
		}
		fields := make(map[string]string)
		for _, f := range n.Fields {
			fields[f.Name.Value] = f.Type
		}
		a.structFields[n.Name.Value] = fields
		return "type"

	case *ast.CallExpression:
		a.Analyze(n.Function)
		for _, arg := range n.Arguments {
			a.Analyze(arg)
		}
		
		if ident, ok := n.Function.(*ast.Identifier); ok {
			// Check signature
			if sig, ok := a.funcSignatures[ident.Value]; ok {
				if len(n.Arguments) != len(sig.Params) && ident.Value != "println" && ident.Value != "generator" {
					a.error(n.Token, "wrong number of arguments for %s: expected %d, got %d", ident.Value, len(sig.Params), len(n.Arguments))
				} else {
					// Check argument types
					for i, arg := range n.Arguments {
						if i >= len(sig.Params) { break }
						argType := a.Analyze(arg)
						expectedType := sig.Params[i]
						if expectedType != "any" && argType != "any" && expectedType != argType {
							a.error(n.Token, "type mismatch for argument %d of %s: expected %s, got %s", i+1, ident.Value, expectedType, argType)
						}
					}
				}
			}

			// Built-in return type deduction
			switch ident.Value {
			case "map":
				return "map"
			case "array":
				return "array"
			case "generator":
				return "Iterator"
			case "len", "arrayLen", "toInt", "strIndex", "strLastIndex", "strCount":
				return "int"
			case "toFloat":
				return "float"
			case "toString", "typeof", "strToLower", "strToUpper", "strTrim", "strTrimSpace", "strJoin", "strReplace", "strRepeat", "strTrimLeft", "strTrimRight", "strReverse", "jsonStringify":
				return "string"
			case "toChar", "charAt":
				return "char"
			case "strContains", "strHasPrefix", "strHasSuffix", "strIsAlpha", "strIsDigit", "strIsSpace", "mapHas", "osExists", "osIsdir", "osIsfile", "regexMatch":
				return "bool"
			}

			sym, ok := a.env.Get(ident.Value)
			if ok && sym.Type == "type" {
				return ident.Value
			}
		} else if dot, ok := n.Function.(*ast.InfixExpression); ok && dot.Operator == "." {
			// Method call: u.greet()
			leftType := a.Analyze(dot.Left)
			if methods, ok := a.structMethods[leftType]; ok {
				if rightIdent, ok := dot.Right.(*ast.Identifier); ok {
					if sig, ok := methods[rightIdent.Value]; ok {
						if len(n.Arguments) != len(sig.Params) {
							a.error(n.Token, "wrong number of arguments for method %s: expected %d, got %d", rightIdent.Value, len(sig.Params), len(n.Arguments))
						} else {
							for i, arg := range n.Arguments {
								argType := a.Analyze(arg)
								expectedType := sig.Params[i]
								if expectedType != "any" && argType != "any" && expectedType != argType {
									a.error(n.Token, "type mismatch for argument %d of method %s: expected %s, got %s", i+1, rightIdent.Value, expectedType, argType)
								}
							}
						}
						return sig.Return
					}
				}
			}
		}
		return "any"

	case *ast.SpawnExpression:
		a.Analyze(n.Call)
		return "thread"

	case *ast.AwaitExpression:
		return a.Analyze(n.Expression)

	case *ast.ReturnStatement:
		var retType string
		if n.ReturnValue != nil {
			retType = a.Analyze(n.ReturnValue)
		} else {
			retType = "any" // Default for empty return
		}
		
		if a.currentReturnType != "" && a.currentReturnType != "any" && retType != "any" {
			if retType != a.currentReturnType {
				a.error(n.Token, "return type mismatch: expected %s, got %s", a.currentReturnType, retType)
			}
		}
		return retType

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

	case *ast.ForStatement:
		// Save current environment
		oldEnv := a.env
		a.env = environment.NewEnclosedEnvironment(oldEnv)
		
		a.Analyze(n.Init)
		condType := a.Analyze(n.Condition)
		if condType != "bool" && condType != "any" {
			a.error(n.Token, "for condition must be bool, got %s", condType)
		}
		a.Analyze(n.Post)
		a.Analyze(n.Body)
		
		a.env = oldEnv
		return "any"

	case *ast.ForEachStatement:
		iterableType := a.Analyze(n.Iterable)
		if iterableType != "array" && iterableType != "map" && iterableType != "string" && iterableType != "Iterator" && iterableType != "any" {
			a.error(n.Token, "cannot iterate over type %s", iterableType)
		}

		// Save current environment
		oldEnv := a.env
		a.env = environment.NewEnclosedEnvironment(oldEnv)
		
		if n.Key != nil {
			keyType := "any"
			if iterableType == "array" || iterableType == "string" { keyType = "int" }
			if iterableType == "map" { keyType = "string" }
			a.env.Set(n.Key.Value, keyType, environment.PUBLIC, false)
		}
		a.env.Set(n.Value.Value, "any", environment.PUBLIC, false)
		
		a.Analyze(n.Body)
		
		a.env = oldEnv
		return "any"

	case *ast.BreakStatement, *ast.ContinueStatement:
		// We should check if we are inside a loop, but for now let's keep it simple
		return "any"

	case *ast.YieldStatement:
		a.Analyze(n.Value)
		return "any"
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
				sig := FunctionSignature{Return: expr.ReturnType}
				for _, p := range expr.Parameters {
					sig.Params = append(sig.Params, p.Type)
				}

				if expr.Receiver != nil {
					if a.structMethods[expr.Receiver.Type] == nil {
						a.structMethods[expr.Receiver.Type] = make(map[string]FunctionSignature)
					}
					a.structMethods[expr.Receiver.Type][expr.Name.Value] = sig
				} else {
					a.funcSignatures[expr.Name.Value] = sig
				}
			}
		case *ast.StructLiteral:
			a.env.Set(expr.Name.Value, "type", environment.PUBLIC, true)
			fields := make(map[string]string)
			for _, f := range expr.Fields {
				fields[f.Name.Value] = f.Type
			}
			a.structFields[expr.Name.Value] = fields
		}
	case *ast.InterfaceStatement:
		a.env.Set(s.Name.Value, "interface", environment.PUBLIC, true)
	case *ast.EnumStatement:
		a.env.Set(s.Name.Value, "namespace", environment.PUBLIC, true)
	case *ast.LetStatement:
		// We don't pre-scan variables to avoid uninitialized usage
	}
}
