package analyzer

import (
	"fmt"
	"github.com/byteme/compiler/ast"
	"github.com/byteme/compiler/environment"
)

type Analyzer struct {
	env    *environment.Environment
	errors []string
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

	return &Analyzer{
		env:    env,
		errors: []string{},
	}
}

func (a *Analyzer) Errors() []string {
	return a.errors
}

func (a *Analyzer) error(format string, args ...interface{}) {
	a.errors = append(a.errors, fmt.Sprintf(format, args...))
}

func (a *Analyzer) Analyze(node ast.Node) string {
	switch n := node.(type) {
	case *ast.Program:
		for _, stmt := range n.Statements {
			a.Analyze(stmt)
		}

	case *ast.LetStatement:
		valType := a.Analyze(n.Value)
		typeName := n.Type
		if typeName == "" {
			typeName = valType
		} else if typeName != valType && valType != "any" && valType != "array" && typeName != "any" {
			a.error("type mismatch: cannot assign %s to %s", valType, typeName)
		}
		err := a.env.Set(n.Name.Value, typeName, environment.PUBLIC, false)
		if err != nil {
			a.error("%s", err.Error())
		}
		return typeName

	case *ast.ConstStatement:
		valType := a.Analyze(n.Value)
		typeName := n.Type
		if typeName == "" {
			typeName = valType
		} else if typeName != valType && valType != "any" && typeName != "any" {
			a.error("type mismatch: cannot assign %s to %s", valType, typeName)
		}
		err := a.env.Set(n.Name.Value, typeName, environment.PUBLIC, true)
		if err != nil {
			a.error("%s", err.Error())
		}
		return typeName

	case *ast.Identifier:
		sym, ok := a.env.Get(n.Value)
		if !ok {
			a.error("undefined variable: %s", n.Value)
			return "any"
		}
		return sym.Type

	case *ast.IntegerLiteral:
		return "int"

	case *ast.FloatLiteral:
		return "float"

	case *ast.StringLiteral:
		return "string"

	case *ast.BooleanLiteral:
		return "bool"

	case *ast.InfixExpression:
		if n.Operator == "." {
			leftType := a.Analyze(n.Left)
			if leftType != "namespace" && leftType != "any" && leftType != "type" {
				a.error("cannot use dot operator on type %s", leftType)
				return "any"
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

		if leftType != rightType {
			a.error("type mismatch in expression: %s %s %s", leftType, n.Operator, rightType)
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
				a.error("parent namespace %s not found", n.Parent.Value)
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
		return "type"

	case *ast.CallExpression:
		a.Analyze(n.Function)
		for _, arg := range n.Arguments {
			a.Analyze(arg)
		}
		return "any" // In a full implementation, we'd look up the return type

	case *ast.SpawnExpression:
		a.Analyze(n.Call)
		return "thread"

	case *ast.AwaitExpression:
		return a.Analyze(n.Expression)
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
	case *ast.LetStatement:
		// We don't pre-scan variables to avoid uninitialized usage
	}
}
