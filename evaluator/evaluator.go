package evaluator

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
	"github.com/byteme/compiler/ast"
	"github.com/byteme/compiler/environment"
	"github.com/byteme/compiler/object"
)

var (
	NULL  = object.NULL
	TRUE  = object.TRUE
	FALSE = object.FALSE
)

var builtins = map[string]*object.Builtin{
	"chan": {
		Fn: func(args ...object.Object) object.Object {
			return &object.Channel{Value: make(chan object.Object, 100)}
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
			ch.Value <- args[1]
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
			return <-ch.Value
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
			return &object.Map{Pairs: make(map[string]object.MapPair)}
		},
	},
	"mapSet": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 3 { return NULL }
			m, ok := args[0].(*object.Map)
			if !ok { return NULL }
			key, ok := args[1].(*object.String)
			if !ok { return NULL }
			m.Pairs[key.Value] = object.MapPair{Key: key, Value: args[2]}
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
			val, ok := m.Get(key.Value)
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
			_, ok = m.Get(key.Value)
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
	"fileRead": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 { return NULL }
			path, ok := args[0].(*object.String)
			if !ok { return NULL }
			content, err := os.ReadFile(path.Value)
			if err != nil {
				return &object.Error{Message: err.Error()}
			}
			return &object.String{Value: string(content)}
		},
	},
	"fileWrite": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 { return NULL }
			path, ok := args[0].(*object.String)
			content, ok2 := args[1].(*object.String)
			if !ok || !ok2 { return NULL }
			err := os.WriteFile(path.Value, []byte(content.Value), 0644)
			if err != nil {
				return &object.Error{Message: err.Error()}
			}
			return TRUE
		},
	},
	"fileAppend": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 { return NULL }
			path, ok := args[0].(*object.String)
			content, ok2 := args[1].(*object.String)
			if !ok || !ok2 { return NULL }
			f, err := os.OpenFile(path.Value, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return &object.Error{Message: err.Error()}
			}
			defer f.Close()
			if _, err := f.WriteString(content.Value); err != nil {
				return &object.Error{Message: err.Error()}
			}
			return TRUE
		},
	},
	"fileExists": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 { return FALSE }
			path, ok := args[0].(*object.String)
			if !ok { return FALSE }
			_, err := os.Stat(path.Value)
			return nativeBoolToBooleanObject(!os.IsNotExist(err))
		},
	},
	"jsonParse": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 { return NULL }
			input, ok := args[0].(*object.String)
			if !ok { return NULL }
			
			var data interface{}
			if err := json.Unmarshal([]byte(input.Value), &data); err != nil {
				return &object.Error{Message: err.Error()}
			}
			return convertToByteMeObject(data)
		},
	},
	"jsonStringify": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 { return NULL }
			input := args[0]
			data := convertToNative(input)
			res, err := json.Marshal(data)
			if err != nil {
				return &object.Error{Message: err.Error()}
			}
			return &object.String{Value: string(res)}
		},
	},
	"timeNow": {
		Fn: func(args ...object.Object) object.Object {
			return &object.Integer{Value: time.Now().UnixMilli()}
		},
	},
	"timeSleep": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 { return NULL }
			ms, ok := args[0].(*object.Integer)
			if !ok { return NULL }
			time.Sleep(time.Duration(ms.Value) * time.Millisecond)
			return NULL
		},
	},
	"timeFormat": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 2 { return NULL }
			ts, ok := args[0].(*object.Integer)
			layout, ok2 := args[1].(*object.String)
			if !ok || !ok2 { return NULL }
			t := time.UnixMilli(ts.Value)
			return &object.String{Value: t.Format(layout.Value)}
		},
	},
	"nativeCall": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) < 1 { return NULL }
			funcName, ok := args[0].(*object.String)
			if !ok { return NULL }
			
			nativeFn, exists := NativeRegistry[funcName.Value]
			if !exists {
				return &object.Error{Message: "native function not found: " + funcName.Value}
			}
			
			return nativeFn(args[1:]...)
		},
	},
	"typeof": {
		Fn: func(args ...object.Object) object.Object {
			if len(args) != 1 { return NULL }
			if si, ok := args[0].(*object.StructInstance); ok {
				return &object.String{Value: si.Definition.Name}
			}
			return &object.String{Value: string(args[0].Type())}
		},
	},
}

var NativeRegistry = make(map[string]func(...object.Object) object.Object)

func RegisterNative(name string, fn func(...object.Object) object.Object) {
	NativeRegistry[name] = fn
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
		var val object.Object = NULL
		if n.Value != nil {
			val = Eval(n.Value, env)
		}
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

	case *ast.StructLiteral:
		structLit := &object.StructLiteral{Name: n.Name.Value, Fields: n.Fields}
		
		if n.Parent != nil {
			if parentObj, ok := env.GetVal("@struct_" + n.Parent.Value); ok {
				if parentLit, ok := parentObj.(*object.StructLiteral); ok {
					structLit.Parent = parentLit
				}
			}
		}

		// Register constructor: struct name is a function
		constructor := &object.Builtin{
			Fn: func(args ...object.Object) object.Object {
				instance := &object.StructInstance{
					Definition: structLit,
					Fields:     make(map[string]object.Object),
				}
				
				// Collect all fields from hierarchy
				var getAllFields func(*object.StructLiteral) []*ast.Parameter
				getAllFields = func(s *object.StructLiteral) []*ast.Parameter {
					if s.Parent == nil {
						return s.Fields
					}
					return append(getAllFields(s.Parent), s.Fields...)
				}
				
				allFields := getAllFields(structLit)
				
				if len(args) > len(allFields) {
					return &object.Error{Message: fmt.Sprintf("struct %s requires %d fields, got %d", structLit.Name, len(allFields), len(args))}
				}

				for i, field := range allFields {
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
		env.SetVal("@struct_" + n.Name.Value, structLit)
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

	case *ast.ThrowStatement:
		val := Eval(n.Value, env)
		if isError(val) { return val }
		return &object.Error{Message: val.Inspect()}

	case *ast.TryStatement:
		return evalTryStatement(n, env)
		
	case *ast.EnumStatement:
		return evalEnumStatement(n, env)
	}

	return nil
}

func evalProgram(program *ast.Program, env *environment.Environment) object.Object {
	var result object.Object

	for _, statement := range program.Statements {
		result = Eval(statement, env)

		if result != nil {
			if rt := result.Type(); rt == object.RETURN_VALUE_OBJ || rt == object.ERROR_OBJ {
				return result
			}
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
			if rt == object.RETURN_VALUE_OBJ || rt == object.ERROR_OBJ {
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
	case left.Type() == object.BOOLEAN_OBJ && right.Type() == object.BOOLEAN_OBJ:
		return evalBooleanInfixExpression(operator, left, right)
	case operator == "==":
		return nativeBoolToBooleanObject(left == right)
	case operator == "!=":
		return nativeBoolToBooleanObject(left != right)
	default:
		return NULL
	}
}

func evalBooleanInfixExpression(operator string, left, right object.Object) object.Object {
	leftVal := left.(*object.Boolean).Value
	rightVal := right.(*object.Boolean).Value

	// Convert bool to int for comparison: false=0, true=1
	lInt := 0
	if leftVal { lInt = 1 }
	rInt := 0
	if rightVal { rInt = 1 }

	switch operator {
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	case "<":
		return nativeBoolToBooleanObject(lInt < rInt)
	case ">":
		return nativeBoolToBooleanObject(lInt > rInt)
	case "<=":
		return nativeBoolToBooleanObject(lInt <= rInt)
	case ">=":
		return nativeBoolToBooleanObject(lInt >= rInt)
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
	leftVal := left.(*object.String).Value
	rightVal := right.(*object.String).Value

	switch operator {
	case "+":
		return &object.String{Value: leftVal + rightVal}
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	default:
		return NULL
	}
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

func isError(obj object.Object) bool {
	if obj != nil {
		return obj.Type() == object.ERROR_OBJ
	}
	return false
}

func evalTryStatement(ts *ast.TryStatement, env *environment.Environment) object.Object {
	result := Eval(ts.Body, env)
	
	if isError(result) && ts.CatchBody != nil {
		catchEnv := environment.NewEnclosedEnvironment(env)
		catchEnv.SetVal(ts.CatchVar.Value, result)
		result = Eval(ts.CatchBody, catchEnv)
	}

	if ts.Finally != nil {
		Eval(ts.Finally, env)
	}

	return result
}

func evalEnumStatement(es *ast.EnumStatement, env *environment.Environment) object.Object {
	nsEnv := environment.NewEnclosedEnvironment(env)
	for i, member := range es.Members {
		nsEnv.SetVal(member.Value, &object.Integer{Value: int64(i)})
	}
	ns := &object.Namespace{Name: es.Name.Value, Env: nsEnv}
	env.SetVal(es.Name.Value, ns)
	return ns
}

func convertToByteMeObject(data interface{}) object.Object {
	return object.ConvertToByteMeObject(data)
}

func convertToNative(obj object.Object) interface{} {
	return object.ConvertToNative(obj)
}
