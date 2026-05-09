package analyzer

import (
	"fmt"
	"io/ioutil"
	
	"github.com/byteme/compiler/ast"
	"github.com/byteme/compiler/environment"
	"github.com/byteme/compiler/lexer"
	"github.com/byteme/compiler/parser"
	"github.com/byteme/compiler/token"
	"path/filepath"
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
	builtinSignatures map[string]FunctionSignature
	source            string
	filename          string
	lines             []string
	ResolvedSymbols   map[string]environment.Symbol // key: "filename:line:col"
}

func New(env *environment.Environment, source string, filename string) *Analyzer {
	// Register basic types as symbols
	env.Set("int", "type", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("float", "type", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("string", "type", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("char", "type", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("bool", "type", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("any", "type", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("array", "type", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("map", "type", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("void", "type", environment.PUBLIC, true, "", 0, 0, "")

	// Register global built-ins
	env.Set("chan", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("send", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("recv", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("println", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("null", "any", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("arrayLen", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("arrayPush", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("arrayPop", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("arrayShift", "function", environment.PUBLIC, true, "", 0, 0, "")
	// env.Set("map", "function", environment.PUBLIC, true, "", 0, 0, "") // Collides with map type
	env.Set("mapSet", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("mapGet", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("mapHas", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("charAt", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("arraySlice", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("arraySort", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("mapDelete", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("mapKeys", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("mapValues", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("timeParse", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("strReplaceAll", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("timeAdd", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("timeSub", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("timeDiff", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("timeInLocation", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("ioReadInput", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("fileRead", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("fileWrite", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("fileAppend", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("fileExists", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("jsonParse", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("jsonStringify", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("timeNow", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("timeSleep", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("timeFormat", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("nativeCall", "function", environment.PUBLIC, true, "", 0, 0, "")
	// HTTP / networking module
	env.Set("httpHandle",   "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("httpServe",    "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("httpGet",      "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("httpPost",     "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("httpResponse", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("httpDo",       "function", environment.PUBLIC, true, "", 0, 0, "")
	
	env.Set("toInt",        "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("toFloat",      "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("toChar",       "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("toString",     "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("typeof",       "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("len",          "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("envGet",       "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("envSet",       "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("args",         "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("exit",         "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("sha256",       "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("md5",          "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("regexMatch",   "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("regexReplace", "function", environment.PUBLIC, true, "", 0, 0, "")
	
	// Math Built-ins
	env.Set("mathSin", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("mathCos", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("mathTan", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("mathSqrt", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("mathPow", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("mathLog", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("mathLog10", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("mathExp", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("mathAsin", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("mathAcos", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("mathAtan", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("mathAtan2", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("mathAbs", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("mathCeil", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("mathFloor", "function", environment.PUBLIC, true, "", 0, 0, "")

	// String Built-ins
	env.Set("strToLower", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("strToUpper", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("strTrim", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("strTrimSpace", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("strSplit", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("strJoin", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("strContains", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("strHasPrefix", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("strHasSuffix", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("strIndex", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("strLastIndex", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("strReplace", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("strRepeat", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("strCount", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("strFields", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("strTrimLeft", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("strTrimRight", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("strIsAlpha", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("strIsDigit", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("strIsSpace", "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("strReverse", "function", environment.PUBLIC, true, "", 0, 0, "")
	
	env.Set("osMkdir",       "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("osRmdir",       "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("osRemove",      "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("osRename",      "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("osListdir",     "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("osExists",      "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("osIsdir",       "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("osIsfile",      "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("osGetcwd",      "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("osChdir",       "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("osGetpid",      "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("pathJoin",      "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("pathBase",      "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("pathDir",       "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("fOpen",         "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("fClose",        "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("fRead",         "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("fWrite",        "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("fSeek",         "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("instanceOf",    "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("generator",     "function", environment.PUBLIC, true, "", 0, 0, "")
	env.Set("mathRand",      "function", environment.PUBLIC, true, "", 0, 0, "")

	a := &Analyzer{
		env:               env,
		errors:            []string{},
		structFields:      make(map[string]map[string]string),
		structMethods:     make(map[string]map[string]FunctionSignature),
		structParents:     make(map[string]string),
		interfaces:        make(map[string][]*ast.MethodSignature),
		currentReturnType: "",
		funcSignatures:    make(map[string]FunctionSignature),
		builtinSignatures: make(map[string]FunctionSignature),
		source:            source,
	}
	
	absPath, err := filepath.Abs(filename)
	if err == nil {
		a.filename = strings.ToLower(strings.ReplaceAll(absPath, "\\", "/"))
	} else {
		a.filename = strings.ToLower(strings.ReplaceAll(filename, "\\", "/"))
	}

	a.lines = strings.Split(source, "\n")
	a.ResolvedSymbols = make(map[string]environment.Symbol)
	
	// Register built-in signatures
	a.builtinSignatures["len"] = FunctionSignature{Params: []string{"any"}, Return: "int"}
	a.builtinSignatures["println"] = FunctionSignature{Params: []string{"any"}, Return: "void"}
	a.builtinSignatures["httpGet"] = FunctionSignature{Params: []string{"string"}, Return: "map"}
	a.builtinSignatures["httpPost"] = FunctionSignature{Params: []string{"string", "string"}, Return: "map"}
	a.builtinSignatures["httpDo"] = FunctionSignature{Params: []string{"string", "string", "string", "map"}, Return: "map"}
	a.builtinSignatures["httpResponse"] = FunctionSignature{Params: []string{"int", "string"}, Return: "map"}
	a.builtinSignatures["typeof"] = FunctionSignature{Params: []string{"any"}, Return: "string"}
	a.builtinSignatures["toString"] = FunctionSignature{Params: []string{"any"}, Return: "string"}
	a.builtinSignatures["toInt"] = FunctionSignature{Params: []string{"any"}, Return: "int"}
	a.builtinSignatures["toFloat"] = FunctionSignature{Params: []string{"any"}, Return: "float"}
	a.builtinSignatures["jsonParse"] = FunctionSignature{Params: []string{"string"}, Return: "any"}
	a.builtinSignatures["jsonStringify"] = FunctionSignature{Params: []string{"any"}, Return: "string"}
	a.builtinSignatures["mapSet"] = FunctionSignature{Params: []string{"map", "any", "any"}, Return: "void"}
	a.builtinSignatures["mapGet"] = FunctionSignature{Params: []string{"map", "any"}, Return: "any"}
	a.builtinSignatures["mapHas"] = FunctionSignature{Params: []string{"map", "any"}, Return: "bool"}
	a.builtinSignatures["arrayLen"] = FunctionSignature{Params: []string{"array"}, Return: "int"}
	a.builtinSignatures["arrayPush"] = FunctionSignature{Params: []string{"array", "any"}, Return: "void"}
	a.builtinSignatures["generator"] = FunctionSignature{Params: []string{}, Return: "Iterator"}
	a.builtinSignatures["httpHandle"] = FunctionSignature{Params: []string{"string", "function"}, Return: "void"}
	a.builtinSignatures["httpServe"] = FunctionSignature{Params: []string{"int"}, Return: "void"}
	a.builtinSignatures["mapKeys"] = FunctionSignature{Params: []string{"map"}, Return: "array"}
	a.builtinSignatures["osExists"] = FunctionSignature{Params: []string{"string"}, Return: "bool"}
	a.builtinSignatures["fileRead"] = FunctionSignature{Params: []string{"string"}, Return: "string"}
	a.builtinSignatures["fileWrite"] = FunctionSignature{Params: []string{"string", "string"}, Return: "void"}
	a.builtinSignatures["strSplit"] = FunctionSignature{Params: []string{"string", "string"}, Return: "array"}
	a.builtinSignatures["strReplace"] = FunctionSignature{Params: []string{"string", "string", "string", "int"}, Return: "string"}
	
	// Populate signatures in environment
	for name, sig := range a.builtinSignatures {
		a.env.SetSignature(name, sig.Params, sig.Return)
	}
	
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
	if target == "float" && source == "int" { return true }
	if target == "string" && (source == "int" || source == "float" || source == "bool" || source == "char") { return true }

	if strings.Contains(source, ".") && !strings.Contains(target, ".") && target != "any" && target != "int" && target != "float" && target != "string" && target != "bool" && target != "char" && target != "array" && target != "map" {
		if strings.HasSuffix(source, "."+target) {
			return true
		}
	}
	// Case where both are base names but match one of the full names? 
	// (Too complex, let's stick to suffix match)

	if target == source { return true }

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
	if typeName == "map" || typeName == "any" {
		return "any", true
	}
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
	if !ok {
		// Try to resolve base name to full name
		for full, f := range a.structFields {
			if strings.HasSuffix(full, "."+typeName) {
				fields = f
				ok = true
				typeName = full
				break
			}
		}
	}

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
	if !ok {
		// Try to resolve base name to full name
		for full, m := range a.structMethods {
			if strings.HasSuffix(full, "."+typeName) {
				methods = m
				ok = true
				typeName = full
				break
			}
		}
	}

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
		
		// Analyze the imported program with its own filename context
		oldFilename := a.filename
		
		absPath, err := filepath.Abs(filename)
		var normalizedPath string
		if err == nil {
			normalizedPath = strings.ToLower(strings.ReplaceAll(absPath, "\\", "/"))
		} else {
			normalizedPath = strings.ToLower(strings.ReplaceAll(filename, "\\", "/"))
		}

		a.filename = normalizedPath
		a.Analyze(program)
		a.filename = oldFilename

		// Record the import path itself for navigation
		ikey := fmt.Sprintf("%s:%d:%d", a.filename, n.Path.Token.Line, n.Path.Token.Column)
		a.ResolvedSymbols[ikey] = environment.Symbol{
			Name:     filename,
			Type:     "module",
			Filename: normalizedPath,
			Line:     1,
			Column:   1,
		}

		if n.Token.Type == token.FROM {
			// Extract specific imports
			for _, imp := range n.Imports {
				// Record usage of the imported symbol in the 'from' list
				// Its definition is whatever was just analyzed in the module
				if sym, ok := a.env.Get(imp.Value); ok {
					ukey := fmt.Sprintf("%s:%d:%d", a.filename, imp.Token.Line, imp.Token.Column)
					a.ResolvedSymbols[ukey] = sym
				}
				a.env.Set(imp.Value, "any", environment.PUBLIC, false, a.filename, imp.Token.Line, imp.Token.Column, "")
			}
		} else if n.Name != nil {
			a.env.Set(n.Name.Value, "map", environment.PUBLIC, false, a.filename, n.Name.Token.Line, n.Name.Token.Column, "")
			// Record the alias definition
			akey := fmt.Sprintf("%s:%d:%d", a.filename, n.Name.Token.Line, n.Name.Token.Column)
			a.ResolvedSymbols[akey] = environment.Symbol{
				Name:     n.Name.Value,
				Type:     "module",
				Filename: a.filename,
				Line:     n.Name.Token.Line,
				Column:   n.Name.Token.Column,
				Docstring: "",
			}
		}

	case *ast.LetStatement:
		if n.Destructuring != nil {
			if n.Value == nil {
				a.error(n.Token, "destructuring declaration requires immediate initialization")
				return "any"
			}
			a.Analyze(n.Value)
			a.analyzeDestructuring(n.Destructuring, false)
			return "any"
		}

		typeName := n.Type
		if typeName == "" {
			if n.Value == nil {
				a.error(n.Token, "variable declaration without type requires immediate initialization")
				return "any"
			}
			typeName = a.Analyze(n.Value)
		}
		// Resolve base name if possible
		if _, ok := a.structFields[typeName]; !ok && typeName != "any" && typeName != "int" && typeName != "float" && typeName != "string" && typeName != "bool" && typeName != "char" && typeName != "array" && typeName != "map" {
			for full := range a.structFields {
				if strings.HasSuffix(full, "."+typeName) {
					typeName = full
					break
				}
			}
		}
		if n.Value != nil {
			valType := a.Analyze(n.Value)
			if !a.isAssignable(typeName, valType) {
				a.error(n.Token, "type mismatch: cannot assign %s to %s", valType, typeName)
			}
		}
		err := a.env.Set(n.Name.Value, typeName, environment.PUBLIC, false, a.filename, n.Name.Token.Line, n.Name.Token.Column, "")
		if err != nil {
			a.error(n.Name.Token, "%s", err.Error())
		}
		// Record definition
		key := fmt.Sprintf("%s:%d:%d", a.filename, n.Name.Token.Line, n.Name.Token.Column)
		a.ResolvedSymbols[key] = environment.Symbol{Name: n.Name.Value, Type: typeName, Filename: a.filename, Line: n.Name.Token.Line, Column: n.Name.Token.Column, Docstring: ""}
		return typeName

	case *ast.ConstStatement:
		if n.Destructuring != nil {
			if n.Value == nil {
				a.error(n.Token, "destructuring declaration requires immediate initialization")
				return "any"
			}
			a.Analyze(n.Value)
			a.analyzeDestructuring(n.Destructuring, true)
			return "any"
		}

		valType := a.Analyze(n.Value)
		typeName := n.Type
		if typeName == "" {
			typeName = valType
		} else if !a.isAssignable(typeName, valType) {
			a.error(n.Token, "type mismatch: cannot assign %s to constant of type %s", valType, typeName)
		}
		err := a.env.Set(n.Name.Value, typeName, environment.PUBLIC, true, a.filename, n.Name.Token.Line, n.Name.Token.Column, "")
		if err != nil {
			a.error(n.Name.Token, "%s", err.Error())
		}
		// Record definition
		key := fmt.Sprintf("%s:%d:%d", a.filename, n.Name.Token.Line, n.Name.Token.Column)
		a.ResolvedSymbols[key] = environment.Symbol{Name: n.Name.Value, Type: typeName, Filename: a.filename, Line: n.Name.Token.Line, Column: n.Name.Token.Column, IsConst: true}
		return typeName

	case *ast.Identifier:
		sym, ok := a.env.Get(n.Value)
		if !ok {
			// Try registered functions
			for full := range a.funcSignatures {
				if strings.HasSuffix(full, "."+n.Value) {
					if s, ok := a.env.Get(full); ok {
						// Record usage
						ukey := fmt.Sprintf("%s:%d:%d", a.filename, n.Token.Line, n.Token.Column)
						a.ResolvedSymbols[ukey] = s
						return s.Type
					}
				}
			}

			a.error(n.Token, "undefined variable: %s", n.Value)
			return "any"
		}
		// Record usage
		ukey := fmt.Sprintf("%s:%d:%d", a.filename, n.Token.Line, n.Token.Column)
		a.ResolvedSymbols[ukey] = sym
		return sym.Type

	case *ast.IntegerLiteral:
		return "int"

	case *ast.NullLiteral:
		return "any"

	case *ast.FloatLiteral:
		return "float"

	case *ast.StringLiteral:
		return "string"

	case *ast.TemplateStringLiteral:
		for _, part := range n.Parts {
			a.Analyze(part)
		}
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
				// Handle namespaces (enums)
				if leftType == "namespace" {
					if leftIdent, ok := n.Left.(*ast.Identifier); ok {
						compoundKey := leftIdent.Value + "." + rightIdent.Value
						if sym, ok := a.env.Get(compoundKey); ok {
							return sym.Type
						}
					}
				}

				// Check fields
				if fieldType, ok := a.lookupField(leftType, rightIdent.Value); ok {
					// Record field usage? We don't have a symbol for fields easily, 
					// but we can try to find the struct definition.
					return fieldType
				}
				// Check methods
				if _, ok := a.lookupMethod(leftType, rightIdent.Value); ok {
					return "function"
				}

				// Handle module symbols (if left is an alias)
				if leftType == "module" || leftType == "map" {
					if leftIdent, ok := n.Left.(*ast.Identifier); ok {
						compoundKey := leftIdent.Value + "." + rightIdent.Value
						if sym, ok := a.env.Get(compoundKey); ok {
							ukey := fmt.Sprintf("%s:%d:%d", a.filename, rightIdent.Token.Line, rightIdent.Token.Column)
							a.ResolvedSymbols[ukey] = sym
							return sym.Type
						}
					}
					// Fallback: maybe it's just a global name imported from that module
					if sym, ok := a.env.Get(rightIdent.Value); ok {
						ukey := fmt.Sprintf("%s:%d:%d", a.filename, rightIdent.Token.Line, rightIdent.Token.Column)
						a.ResolvedSymbols[ukey] = sym
						return sym.Type
					}
				}

				if leftType != "any" && !strings.Contains(leftType, "|") && leftType != "module" && leftType != "map" {
					a.error(n.Token, "type %s has no field or method %s", leftType, rightIdent.Value)
				}
			}
			return "any"
		}
		if n.Operator == "&&" || n.Operator == "||" {
			leftType := a.Analyze(n.Left)
			rightType := a.Analyze(n.Right)
			if leftType == rightType {
				return leftType
			}
			return leftType + "|" + rightType
		}

		if n.Operator == "=" || n.Operator == "+=" || n.Operator == "-=" || n.Operator == "*=" || n.Operator == "/=" {
			leftType := a.Analyze(n.Left)
			rightType := a.Analyze(n.Right)

			if !a.isAssignableNode(n.Left) {
				a.error(n.Token, "operator %s must be applied to an assignable expression", n.Operator)
			}

			if !a.isAssignable(leftType, rightType) {
				if !(n.Operator == "+=" && (leftType == "string" || rightType == "string")) {
					a.error(n.Token, "type mismatch in assignment: cannot assign %s to %s", rightType, leftType)
				}
			}

			// Check for constant reassignment
			if ident, ok := n.Left.(*ast.Identifier); ok {
				if sym, ok := a.env.Get(ident.Value); ok {
					if sym.IsConst {
						a.error(n.Token, "cannot reassign to constant variable: %s", ident.Value)
					}
				}
			}
			return leftType
		}
		leftType := a.Analyze(n.Left)
		rightType := a.Analyze(n.Right)

		if leftType == "any" || rightType == "any" {
			return "any"
		}

		if n.Operator == "+" && (leftType == "string" || rightType == "string") {
			return "string"
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
		rightType := a.Analyze(n.Right)
		if n.Operator == "++" || n.Operator == "--" {
			if rightType != "int" && rightType != "float" && rightType != "any" {
				a.error(n.Token, "operator %s can only be applied to numeric types, got %s", n.Operator, rightType)
			}
			if !a.isAssignableNode(n.Right) {
				a.error(n.Token, "operator %s must be applied to an assignable expression", n.Operator)
			}
			// Check for constant
			if ident, ok := n.Right.(*ast.Identifier); ok {
				if sym, ok := a.env.Get(ident.Value); ok && sym.IsConst {
					a.error(n.Token, "cannot increment/decrement constant variable: %s", ident.Value)
				}
			}
		}
		return rightType

	case *ast.PostfixExpression:
		leftType := a.Analyze(n.Left)
		if n.Operator == "++" || n.Operator == "--" {
			if leftType != "int" && leftType != "float" && leftType != "any" {
				a.error(n.Token, "operator %s can only be applied to numeric types, got %s", n.Operator, leftType)
			}
			if !a.isAssignableNode(n.Left) {
				a.error(n.Token, "operator %s must be applied to an assignable expression", n.Operator)
			}
			// Check for constant
			if ident, ok := n.Left.(*ast.Identifier); ok {
				if sym, ok := a.env.Get(ident.Value); ok && sym.IsConst {
					a.error(n.Token, "cannot increment/decrement constant variable: %s", ident.Value)
				}
			}
		}
		return leftType

	case *ast.ExpressionStatement:
		return a.Analyze(n.Expression)

	case *ast.LambdaExpression:
		return a.AnalyzeLambdaExpression(n)

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

		// Simple type narrowing: if (typeof(x) == "Type")
		var narrowedEnv *environment.Environment
		if bin, ok := n.Condition.(*ast.InfixExpression); ok && bin.Operator == "==" {
			if call, ok := bin.Left.(*ast.CallExpression); ok {
				if ident, ok := call.Function.(*ast.Identifier); ok && ident.Value == "typeof" {
					if targetIdent, ok := call.Arguments[0].(*ast.Identifier); ok {
						if typeLit, ok := bin.Right.(*ast.StringLiteral); ok {
							narrowedEnv = environment.NewEnclosedEnvironment(a.env)
							typeName := typeLit.Value
							// Try to resolve base name to full name if needed
							if _, ok := a.structFields[typeName]; !ok {
								// Check all registered structs for a matching base name
								for full := range a.structFields {
									if strings.HasSuffix(full, "."+typeName) || full == typeName {
										typeName = full
										break
									}
								}
							}
							narrowedEnv.Set(targetIdent.Value, typeName, environment.PUBLIC, false, a.filename, targetIdent.Token.Line, targetIdent.Token.Column, "")
						}
					}
				}
			}
		}

		if narrowedEnv != nil {
			oldEnv := a.env
			a.env = narrowedEnv
			a.Analyze(n.Consequence)
			a.env = oldEnv
		} else {
			a.Analyze(n.Consequence)
		}

		if n.Alternative != nil {
			a.Analyze(n.Alternative)
		}
		return "any"

	case *ast.ArrayLiteral:
		for _, el := range n.Elements {
			a.Analyze(el)
		}
		return "array"

	case *ast.FunctionLiteral:
		// Save current state
		oldEnv := a.env
		oldRet := a.currentReturnType
		
		a.env = environment.NewEnclosedEnvironment(oldEnv)
		a.currentReturnType = strings.ReplaceAll(n.ReturnType, "::", ".")

		// Register parameters
		for _, p := range n.Parameters {
			pType := strings.ReplaceAll(p.Type, "::", ".")
			a.env.Set(p.Name.Value, pType, environment.PUBLIC, false, a.filename, p.Name.Token.Line, p.Name.Token.Column, "")
			// Record definition
			key := fmt.Sprintf("%s:%d:%d", a.filename, p.Name.Token.Line, p.Name.Token.Column)
			a.ResolvedSymbols[key] = environment.Symbol{Name: p.Name.Value, Type: pType, Filename: a.filename, Line: p.Name.Token.Line, Column: p.Name.Token.Column, Docstring: ""}
		}

		// Handle receiver for methods
		if n.Receiver != nil {
			recType := strings.ReplaceAll(n.Receiver.Type, "::", ".")
			a.env.Set(n.Receiver.Name.Value, recType, environment.PUBLIC, false, a.filename, n.Receiver.Name.Token.Line, n.Receiver.Name.Token.Column, "")
		}

		a.Analyze(n.Body)

		// Restore state
		a.env = oldEnv
		a.currentReturnType = oldRet
		return "function"

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


	case *ast.StructLiteral:
		a.env.Set(n.Name.Value, "struct", environment.PUBLIC, true, a.filename, n.Name.Token.Line, n.Name.Token.Column, "")
		if n.Parent != nil {
			a.structParents[n.Name.Value] = n.Parent.Value
		}
		fields := make(map[string]string)
		for _, f := range n.Fields {
			fields[f.Name.Value] = f.Type
		}
		a.structFields[n.Name.Value] = fields
		return "struct"

	case *ast.TernaryExpression:
		a.Analyze(n.Condition)
		consequenceType := a.Analyze(n.Consequence)
		alternativeType := a.Analyze(n.Alternative)
		if consequenceType == alternativeType {
			return consequenceType
		}
		return consequenceType + "|" + alternativeType

	case *ast.CallExpression:
		a.Analyze(n.Function)
		
		argTypes := make([]string, len(n.Arguments))
		for i, arg := range n.Arguments {
			argTypes[i] = a.Analyze(arg)
		}
		
		if ident, ok := n.Function.(*ast.Identifier); ok {
			// Check registered built-in signatures first
			if sig, ok := a.builtinSignatures[ident.Value]; ok {
				return sig.Return
			}

			// Check user-defined function signatures
			if sig, ok := a.funcSignatures[ident.Value]; ok {
				if len(n.Arguments) != len(sig.Params) && ident.Value != "println" && ident.Value != "generator" {
					a.error(n.Token, "wrong number of arguments for %s: expected %d, got %d", ident.Value, len(sig.Params), len(n.Arguments))
				} else {
					for i := range n.Arguments {
						if i >= len(sig.Params) { break }
						argType := argTypes[i]
						expectedType := strings.ReplaceAll(sig.Params[i], "::", ".")
						if expectedType != "any" && argType != "any" && !a.isAssignable(expectedType, argType) {
							a.error(n.Token, "type mismatch for argument %d of %s: expected %s, got %s", i+1, ident.Value, expectedType, argType)
						}
					}
				}
				return sig.Return
			}

			// Built-in return type deduction (Legacy fallback)
			switch ident.Value {
			case "map": return "map"
			case "array": return "array"
			case "generator": return "Iterator"
			case "len", "arrayLen", "toInt", "strIndex", "strLastIndex", "strCount": return "int"
			case "toFloat": return "float"
			case "toString", "typeof", "strToLower", "strToUpper", "strTrim", "strTrimSpace", "strJoin", "strReplace", "strRepeat", "strTrimLeft", "strTrimRight", "strReverse", "jsonStringify": return "string"
			case "toChar", "charAt": return "char"
			case "strContains", "strHasPrefix", "strHasSuffix", "strIsAlpha", "strIsDigit", "strIsSpace", "mapHas", "osExists", "osIsdir", "osIsfile", "regexMatch": return "bool"
			}

			sym, ok := a.env.Get(ident.Value)
			if ok && (sym.Type == "struct" || sym.Type == "type") {
				return ident.Value
			}
		} else if dot, ok := n.Function.(*ast.InfixExpression); ok && (dot.Operator == "." || dot.Operator == "::") {
			// Check if it's a namespace call like SocialSystem::User(...)
			if leftIdent, ok := dot.Left.(*ast.Identifier); ok {
				if rightIdent, ok := dot.Right.(*ast.Identifier); ok {
					compoundKey := leftIdent.Value + "." + rightIdent.Value
					if sym, ok := a.env.Get(compoundKey); ok {
						if sym.Type == "struct" || sym.Type == "type" {
							return compoundKey
						}
						if sym.Type == "function" {
							if sig, ok := a.funcSignatures[compoundKey]; ok {
								return sig.Return
							}
							return "any"
						}
					}
				}
			}

			// Method call: u.greet()
			leftType := a.Analyze(dot.Left)
			if methods, ok := a.structMethods[leftType]; ok {
				if rightIdent, ok := dot.Right.(*ast.Identifier); ok {
					if sig, ok := methods[rightIdent.Value]; ok {
						if len(n.Arguments) != len(sig.Params) {
							a.error(n.Token, "wrong number of arguments for method %s: expected %d, got %d", rightIdent.Value, len(sig.Params), len(n.Arguments))
						} else {
							for i := range n.Arguments {
								argType := argTypes[i]
								expectedType := strings.ReplaceAll(sig.Params[i], "::", ".")
								if expectedType != "any" && argType != "any" && !a.isAssignable(expectedType, argType) {
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
			retType = "void" // Default for empty return
		}
		
		if a.currentReturnType != "" && a.currentReturnType != "any" && retType != "any" {
			if !a.isAssignable(a.currentReturnType, retType) {
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
			catchEnv.Set(n.CatchVar.Value, "any", environment.PUBLIC, false, a.filename, n.CatchVar.Token.Line, n.CatchVar.Token.Column, "")
			
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
		a.env.Set(n.Name.Value, "interface", environment.PUBLIC, true, a.filename, n.Name.Token.Line, n.Name.Token.Column, "")
		a.interfaces[n.Name.Value] = n.Methods
		return "interface"

	case *ast.EnumStatement:
		a.env.Set(n.Name.Value, "namespace", environment.PUBLIC, true, a.filename, n.Name.Token.Line, n.Name.Token.Column, "")
		for _, v := range n.Variants {
			variantKey := n.Name.Value + "." + v.Name.Value
			if len(v.Types) == 0 {
				a.env.Set(variantKey, "any", environment.PUBLIC, true, a.filename, v.Name.Token.Line, v.Name.Token.Column, "")
			} else {
				a.env.Set(variantKey, "function", environment.PUBLIC, true, a.filename, v.Name.Token.Line, v.Name.Token.Column, "")
				sig := FunctionSignature{Params: v.Types, Return: n.Name.Value}
				a.funcSignatures[variantKey] = sig
			}
		}
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
			a.env.Set(n.Key.Value, keyType, environment.PUBLIC, false, a.filename, n.Key.Token.Line, n.Key.Token.Column, "")
		}
		a.env.Set(n.Value.Value, "any", environment.PUBLIC, false, a.filename, n.Value.Token.Line, n.Value.Token.Column, "")
		
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
				a.env.Set(expr.Name.Value, "function", environment.PUBLIC, true, a.filename, expr.Name.Token.Line, expr.Name.Token.Column, expr.Docstring)
				// Record definition
				key := fmt.Sprintf("%s:%d:%d", a.filename, expr.Name.Token.Line, expr.Name.Token.Column)
				sig := FunctionSignature{Return: expr.ReturnType}
				for _, p := range expr.Parameters {
					sig.Params = append(sig.Params, p.Type)
				}
				
				a.env.SetSignature(expr.Name.Value, sig.Params, sig.Return)

				a.ResolvedSymbols[key] = environment.Symbol{
					Name:     expr.Name.Value,
					Type:     "function",
					Filename: a.filename,
					Line:     expr.Name.Token.Line,
					Column:   expr.Name.Token.Column,
					Docstring: expr.Docstring,
					Params:    sig.Params,
					ReturnType: sig.Return,
				}

				if expr.Receiver != nil {
					receiverType := strings.ReplaceAll(expr.Receiver.Type, "::", ".")
					if a.structMethods[receiverType] == nil {
						a.structMethods[receiverType] = make(map[string]FunctionSignature)
					}
					a.structMethods[receiverType][expr.Name.Value] = sig
				} else {
					a.funcSignatures[expr.Name.Value] = sig
				}
			}
		case *ast.StructLiteral:
			a.env.Set(expr.Name.Value, "struct", environment.PUBLIC, true, a.filename, expr.Name.Token.Line, expr.Name.Token.Column, "")
			if expr.Parent != nil {
				a.structParents[expr.Name.Value] = expr.Parent.Value
			}
			fields := make(map[string]string)
			fieldList := []string{}
			for _, f := range expr.Fields {
				fields[f.Name.Value] = f.Type
				fieldList = append(fieldList, f.Name.Value+": "+f.Type)
			}
			a.structFields[expr.Name.Value] = fields

			// Record struct definition for hover
			key := fmt.Sprintf("%s:%d:%d", a.filename, expr.Name.Token.Line, expr.Name.Token.Column)
			fieldDoc := "Fields: " + strings.Join(fieldList, ", ")
			if expr.Parent != nil {
				fieldDoc = "extends " + expr.Parent.Value + " | " + fieldDoc
			}
			a.ResolvedSymbols[key] = environment.Symbol{
				Name:     expr.Name.Value,
				Type:     "struct",
				Filename: a.filename,
				Line:     expr.Name.Token.Line,
				Column:   expr.Name.Token.Column,
				Docstring: fieldDoc,
			}
		}
	case *ast.InterfaceStatement:
		a.env.Set(s.Name.Value, "interface", environment.PUBLIC, true, a.filename, s.Name.Token.Line, s.Name.Token.Column, "")
	case *ast.EnumStatement:
		a.env.Set(s.Name.Value, "namespace", environment.PUBLIC, true, a.filename, s.Name.Token.Line, s.Name.Token.Column, "")
		for _, v := range s.Variants {
			variantKey := s.Name.Value + "." + v.Name.Value
			if len(v.Types) == 0 {
				a.env.Set(variantKey, "any", environment.PUBLIC, true, a.filename, v.Name.Token.Line, v.Name.Token.Column, "")
			} else {
				a.env.Set(variantKey, "function", environment.PUBLIC, true, a.filename, v.Name.Token.Line, v.Name.Token.Column, "")
				sig := FunctionSignature{Params: v.Types, Return: s.Name.Value}
				a.funcSignatures[variantKey] = sig
			}
		}
	case *ast.LetStatement:
		// We don't pre-scan variables to avoid uninitialized usage
	}
}
func (a *Analyzer) analyzeDestructuring(pattern ast.Expression, isConst bool) {
	switch p := pattern.(type) {
	case *ast.Identifier:
		a.env.Set(p.Value, "any", environment.PUBLIC, isConst, a.filename, p.Token.Line, p.Token.Column, "")
	case *ast.ArrayLiteral:
		for _, el := range p.Elements {
			a.analyzeDestructuring(el, isConst)
		}
	}
}

func (a *Analyzer) AnalyzeLambdaExpression(n *ast.LambdaExpression) string {
	prevEnv := a.env
	a.env = environment.NewEnclosedEnvironment(prevEnv)
	defer func() { a.env = prevEnv }()

	for _, p := range n.Parameters {
		a.env.Set(p.Name.Value, p.Type, environment.PUBLIC, false, a.filename, p.Name.Token.Line, p.Name.Token.Column, "")
	}

	return a.Analyze(n.Body)
}

func (a *Analyzer) isAssignableNode(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.Identifier:
		return true
	case *ast.IndexExpression:
		return true
	case *ast.InfixExpression:
		return n.Operator == "."
	}
	return false
}
