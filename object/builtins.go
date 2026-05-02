package object

import (
	"fmt"
	"encoding/json"
	"time"
	"os"
)

var Builtins = []*Builtin{
	{ // 0: println
		Fn: func(args ...Object) Object {
			for _, arg := range args {
				fmt.Print(arg.Inspect())
				fmt.Print(" ")
			}
			fmt.Println()
			return NULL
		},
	},
	{ // 1: len
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return &Error{Message: fmt.Sprintf("wrong number of arguments. got=%d, want=1", len(args))}
			}
			switch arg := args[0].(type) {
			case *Array:
				return &Integer{Value: int64(len(arg.Elements))}
			case *String:
				return &Integer{Value: int64(len(arg.Value))}
			default:
				return &Error{Message: fmt.Sprintf("argument to `len` not supported, got %s", args[0].Type())}
			}
		},
	},
	{ // 2: timeNow
		Fn: func(args ...Object) Object {
			return &Integer{Value: time.Now().UnixMilli()}
		},
	},
	{ // 3: jsonParse
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			input, ok := args[0].(*String)
			if !ok { return NULL }
			var data interface{}
			if err := json.Unmarshal([]byte(input.Value), &data); err != nil {
				return &Error{Message: err.Error()}
			}
			return ConvertToByteMeObject(data)
		},
	},
	{ // 4: jsonStringify
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			data := ConvertToNative(args[0])
			res, err := json.Marshal(data)
			if err != nil {
				return &Error{Message: err.Error()}
			}
			return &String{Value: string(res)}
		},
	},
	{ // 5: timeSleep
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			ms, ok := args[0].(*Integer)
			if !ok { return NULL }
			time.Sleep(time.Duration(ms.Value) * time.Millisecond)
			return NULL
		},
	},
	{ // 6: timeFormat
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			ts, ok := args[0].(*Integer)
			layout, ok2 := args[1].(*String)
			if !ok || !ok2 { return NULL }
			t := time.UnixMilli(ts.Value)
			return &String{Value: t.Format(layout.Value)}
		},
	},
	{ // 7: mapSet
		Fn: func(args ...Object) Object {
			if len(args) != 3 { return NULL }
			m, ok := args[0].(*Map)
			key, ok2 := args[1].(*String)
			if !ok || !ok2 { return NULL }
			m.Pairs[key.Value] = args[2]
			return NULL
		},
	},
	{ // 8: mapGet
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			m, ok := args[0].(*Map)
			key, ok2 := args[1].(*String)
			if !ok || !ok2 { return NULL }
			val, ok := m.Pairs[key.Value]
			if !ok { return NULL }
			return val
		},
	},
	{ // 9: fileRead
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			path, ok := args[0].(*String)
			if !ok { return NULL }
			content, err := os.ReadFile(path.Value)
			if err != nil { return &Error{Message: err.Error()} }
			return &String{Value: string(content)}
		},
	},
	{ // 10: fileWrite
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			path, ok := args[0].(*String)
			content, ok2 := args[1].(*String)
			if !ok || !ok2 { return NULL }
			err := os.WriteFile(path.Value, []byte(content.Value), 0644)
			if err != nil { return &Error{Message: err.Error()} }
			return NULL
		},
	},
	{ // 11: map
		Fn: func(args ...Object) Object {
			return &Map{Pairs: make(map[string]Object)}
		},
	},
	{ // 12: chan
		Fn: func(args ...Object) Object {
			return &Channel{Value: make(chan Object)}
		},
	},
	{ // 13: send
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return &Error{Message: "send requires 2 arguments"} }
			ch, ok := args[0].(*Channel)
			if !ok { return &Error{Message: "first argument to send must be a channel"} }
			ch.Value <- args[1]
			return NULL
		},
	},
	{ // 14: recv
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return &Error{Message: "recv requires 1 argument"} }
			ch, ok := args[0].(*Channel)
			if !ok { return &Error{Message: "first argument to recv must be a channel"} }
			return <-ch.Value
		},
	},
}

func ConvertToByteMeObject(val interface{}) Object {
	switch v := val.(type) {
	case bool:
		if v { return TRUE }
		return FALSE
	case float64:
		return &Float{Value: v}
	case string:
		return &String{Value: v}
	case []interface{}:
		elements := make([]Object, len(v))
		for i, e := range v {
			elements[i] = ConvertToByteMeObject(e)
		}
		return &Array{Elements: elements}
	case map[string]interface{}:
		pairs := make(map[string]Object)
		for k, val := range v {
			pairs[k] = ConvertToByteMeObject(val)
		}
		return &Map{Pairs: pairs}
	default:
		return NULL
	}
}

func ConvertToNative(obj Object) interface{} {
	switch o := obj.(type) {
	case *Integer: return o.Value
	case *Float:   return o.Value
	case *Boolean: return o.Value
	case *String:  return o.Value
	case *Array:
		res := make([]interface{}, len(o.Elements))
		for i, e := range o.Elements {
			res[i] = ConvertToNative(e)
		}
		return res
	case *Map:
		res := make(map[string]interface{})
		for k, v := range o.Pairs {
			res[k] = ConvertToNative(v)
		}
		return res
	default:
		return nil
	}
}
