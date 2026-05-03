package object

import (
	"fmt"
	"encoding/json"
	"time"
	"os"
	"io"
	"crypto/sha256"
	"crypto/md5"
	"encoding/hex"
	"regexp"
	"path/filepath"
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
	{ // 15: envGet
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			key, ok := args[0].(*String)
			if !ok { return NULL }
			return &String{Value: os.Getenv(key.Value)}
		},
	},
	{ // 16: envSet
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			key, ok1 := args[0].(*String)
			val, ok2 := args[1].(*String)
			if !ok1 || !ok2 { return NULL }
			os.Setenv(key.Value, val.Value)
			return NULL
		},
	},
	{ // 17: args
		Fn: func(args ...Object) Object {
			res := &Array{Elements: make([]Object, len(os.Args))}
			for i, a := range os.Args {
				res.Elements[i] = &String{Value: a}
			}
			return res
		},
	},
	{ // 18: exit
		Fn: func(args ...Object) Object {
			code := 0
			if len(args) == 1 {
				if c, ok := args[0].(*Integer); ok {
					code = int(c.Value)
				}
			}
			os.Exit(code)
			return NULL
		},
	},
	{ // 19: sha256
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			input, ok := args[0].(*String)
			if !ok { return NULL }
			hash := sha256.Sum256([]byte(input.Value))
			return &String{Value: hex.EncodeToString(hash[:])}
		},
	},
	{ // 20: md5
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			input, ok := args[0].(*String)
			if !ok { return NULL }
			hash := md5.Sum([]byte(input.Value))
			return &String{Value: hex.EncodeToString(hash[:])}
		},
	},
	{ // 21: regexMatch
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			pattern, ok1 := args[0].(*String)
			text, ok2 := args[1].(*String)
			if !ok1 || !ok2 { return NULL }
			match, err := regexp.MatchString(pattern.Value, text.Value)
			if err != nil { return &Error{Message: err.Error()} }
			if match { return TRUE }
			return FALSE
		},
	},
	{ // 22: regexReplace
		Fn: func(args ...Object) Object {
			if len(args) != 3 { return NULL }
			pattern, ok1 := args[0].(*String)
			text, ok2 := args[1].(*String)
			repl, ok3 := args[2].(*String)
			if !ok1 || !ok2 || !ok3 { return NULL }
			re, err := regexp.Compile(pattern.Value)
			if err != nil { return &Error{Message: err.Error()} }
			return &String{Value: re.ReplaceAllString(text.Value, repl.Value)}
		},
	},
	{ // 23: osMkdir
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			path, ok := args[0].(*String)
			if !ok { return NULL }
			if err := os.MkdirAll(path.Value, 0755); err != nil { return &Error{Message: err.Error()} }
			return NULL
		},
	},
	{ // 24: osRmdir
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			path, ok := args[0].(*String)
			if !ok { return NULL }
			if err := os.Remove(path.Value); err != nil { return &Error{Message: err.Error()} }
			return NULL
		},
	},
	{ // 25: osRemove
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			path, ok := args[0].(*String)
			if !ok { return NULL }
			if err := os.Remove(path.Value); err != nil { return &Error{Message: err.Error()} }
			return NULL
		},
	},
	{ // 26: osRename
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			old, ok1 := args[0].(*String)
			newPath, ok2 := args[1].(*String)
			if !ok1 || !ok2 { return NULL }
			if err := os.Rename(old.Value, newPath.Value); err != nil { return &Error{Message: err.Error()} }
			return NULL
		},
	},
	{ // 27: osListdir
		Fn: func(args ...Object) Object {
			path := "."
			if len(args) == 1 {
				if p, ok := args[0].(*String); ok { path = p.Value }
			}
			entries, err := os.ReadDir(path)
			if err != nil { return &Error{Message: err.Error()} }
			res := &Array{Elements: make([]Object, len(entries))}
			for i, e := range entries { res.Elements[i] = &String{Value: e.Name()} }
			return res
		},
	},
	{ // 28: osExists
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			path, ok := args[0].(*String)
			if !ok { return NULL }
			if _, err := os.Stat(path.Value); err == nil { return TRUE }
			return FALSE
		},
	},
	{ // 29: osIsdir
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			path, ok := args[0].(*String)
			if !ok { return NULL }
			info, err := os.Stat(path.Value)
			if err == nil && info.IsDir() { return TRUE }
			return FALSE
		},
	},
	{ // 30: osIsfile
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			path, ok := args[0].(*String)
			if !ok { return NULL }
			info, err := os.Stat(path.Value)
			if err == nil && !info.IsDir() { return TRUE }
			return FALSE
		},
	},
	{ // 31: osGetcwd
		Fn: func(args ...Object) Object {
			cwd, err := os.Getwd()
			if err != nil { return &Error{Message: err.Error()} }
			return &String{Value: cwd}
		},
	},
	{ // 32: osChdir
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			path, ok := args[0].(*String)
			if !ok { return NULL }
			if err := os.Chdir(path.Value); err != nil { return &Error{Message: err.Error()} }
			return NULL
		},
	},
	{ // 33: osGetpid
		Fn: func(args ...Object) Object {
			return &Integer{Value: int64(os.Getpid())}
		},
	},
	{ // 34: pathJoin
		Fn: func(args ...Object) Object {
			parts := make([]string, len(args))
			for i, arg := range args {
				if s, ok := arg.(*String); ok { parts[i] = s.Value }
			}
			return &String{Value: filepath.Join(parts...)}
		},
	},
	{ // 35: pathBase
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			path, ok := args[0].(*String)
			if !ok { return NULL }
			return &String{Value: filepath.Base(path.Value)}
		},
	},
	{ // 36: pathDir
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			path, ok := args[0].(*String)
			if !ok { return NULL }
			return &String{Value: filepath.Dir(path.Value)}
		},
	},
	{ // 37: fileOpen
		Fn: func(args ...Object) Object {
			if len(args) < 1 { return &Error{Message: "fileOpen requires at least 1 argument"} }
			path, ok := args[0].(*String)
			if !ok { return &Error{Message: "fileOpen: path must be a string"} }
			mode := "r"
			if len(args) > 1 {
				if m, ok := args[1].(*String); ok { mode = m.Value }
			}
			
			var flag int
			switch mode {
			case "r":  flag = os.O_RDONLY
			case "r+": flag = os.O_RDWR
			case "w":  flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
			case "w+": flag = os.O_RDWR | os.O_CREATE | os.O_TRUNC
			case "a":  flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
			case "a+": flag = os.O_RDWR | os.O_CREATE | os.O_APPEND
			default:   flag = os.O_RDONLY
			}
			
			f, err := os.OpenFile(path.Value, flag, 0644)
			if err != nil { return &Error{Message: err.Error()} }
			return &FileHandle{File: f}
		},
	},
	{ // 38: fileClose
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			handle, ok := args[0].(*FileHandle)
			if !ok { return NULL }
			handle.File.Close()
			return NULL
		},
	},
	{ // 39: fileRead
		Fn: func(args ...Object) Object {
			if len(args) < 1 { return NULL }
			handle, ok := args[0].(*FileHandle)
			if !ok { return NULL }
			n := -1
			if len(args) > 1 {
				if iv, ok := args[1].(*Integer); ok { n = int(iv.Value) }
			}
			
			if n == -1 {
				content, err := io.ReadAll(handle.File)
				if err != nil { return &Error{Message: err.Error()} }
				return &String{Value: string(content)}
			}
			
			buf := make([]byte, n)
			count, err := handle.File.Read(buf)
			if err != nil && err.Error() != "EOF" { return &Error{Message: err.Error()} }
			return &String{Value: string(buf[:count])}
		},
	},
	{ // 40: fileWrite
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			handle, ok := args[0].(*FileHandle)
			data, ok2 := args[1].(*String)
			if !ok || !ok2 { return NULL }
			count, err := handle.File.WriteString(data.Value)
			if err != nil { return &Error{Message: err.Error()} }
			return &Integer{Value: int64(count)}
		},
	},
	{ // 41: fileSeek
		Fn: func(args ...Object) Object {
			if len(args) < 2 { return NULL }
			handle, ok := args[0].(*FileHandle)
			offset, ok2 := args[1].(*Integer)
			if !ok || !ok2 { return NULL }
			whence := 0
			if len(args) > 2 {
				if w, ok := args[2].(*Integer); ok { whence = int(w.Value) }
			}
			pos, err := handle.File.Seek(offset.Value, whence)
			if err != nil { return &Error{Message: err.Error()} }
			return &Integer{Value: pos}
		},
	},
	{ // 42: instanceOf
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return &Error{Message: fmt.Sprintf("wrong number of arguments. got=%d, want=2", len(args))}
			}
			obj := args[0]
			targetType := args[1]

			switch t := targetType.(type) {
			case *StructLiteral:
				instance, ok := obj.(*StructInstance)
				if !ok {
					return FALSE
				}
				if instance.Definition.Name == t.Name {
					return TRUE
				}
				return FALSE
			case *String:
				// Primitive type check via string name
				if string(obj.Type()) == t.Value {
					return TRUE
				}
				return FALSE
			default:
				return &Error{Message: fmt.Sprintf("second argument to `instanceof` must be a type or type name string, got %s", targetType.Type())}
			}
		},
	},
	{ // 43: mapHas
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			m, ok := args[0].(*Map)
			key, ok2 := args[1].(*String)
			if !ok || !ok2 { return NULL }
			_, exists := m.Pairs[key.Value]
			if exists {
				return TRUE
			}
			return FALSE
		},
	},
	{ // 44: charAt
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			s, ok := args[0].(*String)
			i, ok2 := args[1].(*Integer)
			if !ok || !ok2 { return NULL }
			if i.Value < 0 || i.Value >= int64(len(s.Value)) {
				return NULL
			}
			return &String{Value: string(s.Value[i.Value])}
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
