package evaluator

import (
	"fmt"
	"github.com/byteme/compiler/ast"
	"github.com/byteme/compiler/environment"
	"github.com/byteme/compiler/object"
)

var (
	NULL  = &object.Null{}
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
)

var builtins = map[string]*object.Builtin{
	"chan": {
		Fn: func(args ...object.Object) object.Object {
			return &object.Channel{Internal: make(chan object.Object, 100)}
		},
	},
	"send": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 {
				return NULL
			}
			ch, ok := args[0].(*object.Channel)
			if !ok {
				return NULL
			}
			ch.Internal <- args[1]
			return NULL
		},
	},
	"recv": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 {
				return NULL
			}
			ch, ok := args[0].(*object.Channel)
			if !ok {
				return NULL
			}
			return <-ch.Internal
		},
	},
	"println": {
		Fn: func(args ...object.Object) object.Object {
			for _, arg := range args {
				fmt.Print(arg.Inspect(), " ")
			}
			fmt.Println()
			return NULL
		},
	},
	"arrayLen": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 { return &object.Integer{Value: 0} }
			if arr, ok := args[0].(*object.Array); ok {
				return &object.Integer{Value: int64(len(arr.Elements))}
			}
			if s, ok := args[0].(*object.String); ok {
				return &object.Integer{Value: int64(len(s.Value))}
			}
			return &object.Integer{Value: 0}
		},
	},
	"arrayPush": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 { return NULL }
			arr, ok := args[0].(*object.Array)
			if !ok { return NULL }
			arr.Elements = append(arr.Elements, args[1])
			return NULL
		},
	},
	"arrayPop": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 { return NULL }
			arr, ok := args[0].(*object.Array)
			if !ok || len(arr.Elements) == 0 { return NULL }
			last := arr.Elements[len(arr.Elements)-1]
			arr.Elements = arr.Elements[:len(arr.Elements)-1]
			return last
		},
	},
	"arrayShift": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 { return NULL }
			arr, ok := args[0].(*object.Array)
			if !ok || len(arr.Elements) == 0 { return NULL }
			first := arr.Elements[0]
			arr.Elements = arr.Elements[1:]
			return first
		},
	},
	"map": {
		Fn: func(args ...object.Object) object.Object {
			return &object.Map{Pairs: make(map[string]object.Object)}
		},
	},
	"mapSet": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 3 { return NULL }
			m, ok := args[0].(*object.Map)
			if !ok { return NULL }
			key, ok := args[1].(*object.String)
			if !ok { return NULL }
			m.Pairs[key.Value] = args[2]
			return NULL
		},
	},
	"mapGet": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 { return NULL }
			m, ok := args[0].(*object.Map)
			if !ok { return NULL }
			key, ok := args[1].(*object.String)
			if !ok { return NULL }
			val, ok := m.Pairs[key.Value]
			if !ok { return NULL }
			return val
		},
	},
	"mapHas": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 { return FALSE }
			m, ok := args[0].(*object.Map)
			if !ok { return FALSE }
			key, ok := args[1].(*object.String)
			if !ok { return FALSE }
			_, ok = m.Pairs[key.Value]
			return nativeBoolToBooleanObject(ok)
		},
	},
	"charAt": { // Useful for Trie
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 { return NULL }
			s, ok := args[0].(*object.String)
			if !ok { return NULL }
			idx, ok := args[1].(*object.Integer)
			if !ok { return NULL }
			if idx.Value < 0 || idx.Value >= int64(len(s.Value)) { return NULL }
			return &object.String{Value: string(s.Value[idx.Value])}
		},
	},
}

func Eval(node ast.Node, env *environment.Environment) object.Object {
	switch n := node.(type) {
	case *ast.Program:
		return evalProgram(n, env)

	case *ast.ExpressionStatement:
		return Eval(n.Expression, env)

	case *ast.IntegerLiteral:
		return &object.Integer{Value: n.Value}

	case *ast.FloatLiteral:
		return &object.Float{Value: n.Value}

	case *ast.BooleanLiteral:
		return nativeBoolToBooleanObject(n.Value)

	case *ast.StringLiteral:
		return &object.String{Value: n.Value}

	case *ast.PrefixExpression:
		right := Eval(n.Right, env)
		return evalPrefixExpression(n.Operator, right)

	case *ast.InfixExpression:
		if n.Operator == "." {
			left := Eval(n.Left, env)
			rightIdent, ok := n.Right.(*ast.Identifier)
			if !ok { return NULL }
			return evalAccessExpression(left, rightIdent)
		}
		if n.Operator == "=" {
			val := Eval(n.Right, env)
			evalAssignment(n.Left, val, env)
			return val
		}
		left := Eval(n.Left, env)
		right := Eval(n.Right, env)
		return evalInfixExpression(n.Operator, left, right)

	case *ast.WhileStatement:
		return evalWhileStatement(n, env)

	case *ast.ArrayLiteral:
		elements := evalExpressions(n.Elements, env)
		return &object.Array{Elements: elements}

	case *ast.IndexExpression:
		left := Eval(n.Left, env)
		index := Eval(n.Index, env)
		return evalIndexExpression(left, index)

	case *ast.BlockStatement:
		return evalBlockStatement(n, env)

	case *ast.IfExpression:
		return evalIfExpression(n, env)

	case *ast.ReturnStatement:
		val := Eval(n.ReturnValue, env)
		return &object.ReturnValue{Value: val}

	case *ast.LetStatement:
		val := Eval(n.Value, env)
		env.SetVal(n.Name.Value, val)
		return val

	case *ast.Identifier:
		return evalIdentifier(n, env)

	case *ast.FunctionLiteral:
		params := n.Parameters
		body := n.Body
		fn := &object.Function{Parameters: params, Env: env, Body: body, IsAsync: n.IsAsync}
		if n.Name != nil {
			env.SetVal(n.Name.Value, fn)
		}
		return fn

	case *ast.CallExpression:
		function := Eval(n.Function, env)
		args := evalExpressions(n.Arguments, env)
		return applyFunction(function, args)

	case *ast.NamespaceLiteral:
		nsEnv := environment.NewEnclosedEnvironment(env)
		Eval(n.Body, nsEnv)
		ns := &object.Namespace{Name: n.Name.Value, Env: nsEnv}
		env.SetVal(n.Name.Value, ns)
		return ns

	case *ast.StructLiteral:
		structLit := &object.StructLiteral{Name: n.Name.Value, Fields: n.Fields}
		// Register constructor: struct name is a function
		constructor := &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				instance := &object.StructInstance{
					Definition: structLit,
					Fields:     make(map[string]object.Object),
				}
				for i, field := range structLit.Fields {
					if i < len(args) {
						instance.Fields[field.Name.Value] = args[i]
					} else {
						instance.Fields[field.Name.Value] = NULL
					}
				}
				return instance
			},
		}
		env.SetVal(n.Name.Value, constructor)
		return structLit

	case *ast.SpawnExpression:
		go func() {
			Eval(n.Call, env)
		}()
		return NULL

	case *ast.AwaitExpression:
		val := Eval(n.Expression, env)
		if future, ok := val.(*object.Future); ok {
			return future.Get()
		}
		return val
	}

	return nil
}

func evalProgram(program *ast.Program, env *environment.Environment) object.Object {
	var result object.Object

	for _, statement := range program.Statements {
		result = Eval(statement, env)

		if returnValue, ok := result.(*object.ReturnValue); ok {
			return returnValue.Value
		}
	}

	return result
}

func evalBlockStatement(block *ast.BlockStatement, env *environment.Environment) object.Object {
	var result object.Object

	for _, statement := range block.Statements {
		result = Eval(statement, env)

		if result != nil {
			rt := result.Type()
			if rt == object.RETURN_VALUE_OBJ {
				return result
			}
		}
	}

	return result
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

func evalPrefixExpression(operator string, right object.Object) object.Object {
	switch operator {
	case "!":
		return evalBangOperatorExpression(right)
	case "-":
		return evalMinusPrefixOperatorExpression(right)
	default:
		return NULL
	}
}

func evalBangOperatorExpression(right object.Object) object.Object {
	switch right {
	case TRUE:
		return FALSE
	case FALSE:
		return TRUE
	case NULL:
		return TRUE
	default:
		return FALSE
	}
}

func evalMinusPrefixOperatorExpression(right object.Object) object.Object {
	if right.Type() == object.INTEGER_OBJ {
		value := right.(*object.Integer).Value
		return &object.Integer{Value: -value}
	}
	if right.Type() == object.FLOAT_OBJ {
		value := right.(*object.Float).Value
		return &object.Float{Value: -value}
	}
	return NULL
}

func evalInfixExpression(operator string, left, right object.Object) object.Object {
	switch {
	case left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ:
		return evalIntegerInfixExpression(operator, left, right)
	case left.Type() == object.FLOAT_OBJ && right.Type() == object.FLOAT_OBJ:
		return evalFloatInfixExpression(operator, left, right)
	case left.Type() == object.STRING_OBJ && right.Type() == object.STRING_OBJ:
		return evalStringInfixExpression(operator, left, right)
	case operator == "==":
		return nativeBoolToBooleanObject(left == right)
	case operator == "!=":
		return nativeBoolToBooleanObject(left != right)
	default:
		return NULL
	}
}

func evalIntegerInfixExpression(operator string, left, right object.Object) object.Object {
	leftVal := left.(*object.Integer).Value
	rightVal := right.(*object.Integer).Value

	switch operator {
	case "+":
		return &object.Integer{Value: leftVal + rightVal}
	case "-":
		return &object.Integer{Value: leftVal - rightVal}
	case "*":
		return &object.Integer{Value: leftVal * rightVal}
	case "/":
		return &object.Integer{Value: leftVal / rightVal}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return NULL
	}
}

func evalFloatInfixExpression(operator string, left, right object.Object) object.Object {
	leftVal := left.(*object.Float).Value
	rightVal := right.(*object.Float).Value

	switch operator {
	case "+":
		return &object.Float{Value: leftVal + rightVal}
	case "-":
		return &object.Float{Value: leftVal - rightVal}
	case "*":
		return &object.Float{Value: leftVal * rightVal}
	case "/":
		return &object.Float{Value: leftVal / rightVal}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	default:
		return NULL
	}
}

func evalStringInfixExpression(operator string, left, right object.Object) object.Object {
	if operator != "+" {
		return NULL
	}

	leftVal := left.(*object.String).Value
	rightVal := right.(*object.String).Value
	return &object.String{Value: leftVal + rightVal}
}

func evalAccessExpression(left object.Object, ident *ast.Identifier) object.Object {
	if left.Type() == object.NAMESPACE_OBJ {
		ns := left.(*object.Namespace)
		val, ok := ns.Env.GetVal(ident.Value)
		if ok {
			return val.(object.Object)
		}
	}
	if left.Type() == object.STRUCT_OBJ {
		instance := left.(*object.StructInstance)
		val, ok := instance.Fields[ident.Value]
		if ok {
			return val
		}
	}
	return NULL
}

func evalWhileStatement(ws *ast.WhileStatement, env *environment.Environment) object.Object {
	var result object.Object = NULL

	for isTruthy(Eval(ws.Condition, env)) {
		result = Eval(ws.Body, env)
		if result != nil && result.Type() == object.RETURN_VALUE_OBJ {
			return result
		}
	}

	return result
}

func evalIndexExpression(left, index object.Object) object.Object {
	switch {
	case left.Type() == object.ARRAY_OBJ && index.Type() == object.INTEGER_OBJ:
		return evalArrayArrayIndexExpression(left, index)
	default:
		return NULL
	}
}

func evalArrayArrayIndexExpression(array, index object.Object) object.Object {
	arrayObject := array.(*object.Array)
	idx := index.(*object.Integer).Value
	max := int64(len(arrayObject.Elements) - 1)

	if idx < 0 || idx > max {
		return NULL
	}

	return arrayObject.Elements[idx]
}

func evalAssignment(left ast.Expression, val object.Object, env *environment.Environment) {
	switch l := left.(type) {
	case *ast.Identifier:
		env.SetValInScope(l.Value, val)
	case *ast.IndexExpression:
		arrayObj := Eval(l.Left, env)
		if arrayObj.Type() == object.ARRAY_OBJ {
			array := arrayObj.(*object.Array)
			idx := Eval(l.Index, env).(*object.Integer).Value
			if idx >= 0 && idx < int64(len(array.Elements)) {
				array.Elements[idx] = val
			}
		}
	case *ast.InfixExpression:
		if l.Operator == "." {
			leftObj := Eval(l.Left, env)
			if leftObj.Type() == object.STRUCT_OBJ {
				instance := leftObj.(*object.StructInstance)
				rightIdent := l.Right.(*ast.Identifier)
				instance.Fields[rightIdent.Value] = val
			}
		}
	}
}

func evalIfExpression(ie *ast.IfExpression, env *environment.Environment) object.Object {
	condition := Eval(ie.Condition, env)

	if isTruthy(condition) {
		return Eval(ie.Consequence, env)
	} else if ie.Alternative != nil {
		return Eval(ie.Alternative, env)
	} else {
		return NULL
	}
}

func isTruthy(obj object.Object) bool {
	switch obj {
	case NULL:
		return false
	case TRUE:
		return true
	case FALSE:
		return false
	default:
		return true
	}
}

func evalIdentifier(node *ast.Identifier, env *environment.Environment) object.Object {
	if val, ok := env.GetVal(node.Value); ok {
		return val.(object.Object)
	}
	if builtin, ok := builtins[node.Value]; ok {
		return builtin
	}
	return NULL
}

func evalExpressions(exps []ast.Expression, env *environment.Environment) []object.Object {
	var result []object.Object

	for _, e := range exps {
		evaluated := Eval(e, env)
		result = append(result, evaluated)
	}

	return result
}

func applyFunction(fn object.Object, args []object.Object) object.Object {
	switch function := fn.(type) {
	case *object.Function:
		extendEnv := func(f *object.Function, args []object.Object) *environment.Environment {
			env := environment.NewEnclosedEnvironment(f.Env)
			for paramIdx, param := range f.Parameters {
				env.SetVal(param.Name.Value, args[paramIdx])
			}
			return env
		}

		env := extendEnv(function, args)

		if function.IsAsync {
			future := &object.Future{ValueChan: make(chan object.Object, 1)}
			go func() {
				evaluated := Eval(function.Body, env)
				future.ValueChan <- unwrapReturnValue(evaluated)
			}()
			return future
		}

		evaluated := Eval(function.Body, env)
		return unwrapReturnValue(evaluated)

	case *object.Builtin:
		return function.Fn(args...)

	default:
		return NULL
	}
}

func unwrapReturnValue(obj object.Object) object.Object {
	if returnValue, ok := obj.(*object.ReturnValue); ok {
		return returnValue.Value
	}
	return obj
}
