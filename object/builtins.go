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
	"math"
	"strings"
	"unicode"
	"bufio"
	"sort"
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
	{ // 45: toInt
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			switch arg := args[0].(type) {
			case *Integer: return arg
			case *Float: return &Integer{Value: int64(arg.Value)}
			default: return NULL
			}
		},
	},
	{ // 46: toFloat
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			switch arg := args[0].(type) {
			case *Integer: return &Float{Value: float64(arg.Value)}
			case *Float: return arg
			default: return NULL
			}
		},
	},
	{ // 47: mathSin
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			val := 0.0
			if f, ok := args[0].(*Float); ok { val = f.Value } else if i, ok := args[0].(*Integer); ok { val = float64(i.Value) }
			return &Float{Value: math.Sin(val)}
		},
	},
	{ // 48: mathCos
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			val := 0.0
			if f, ok := args[0].(*Float); ok { val = f.Value } else if i, ok := args[0].(*Integer); ok { val = float64(i.Value) }
			return &Float{Value: math.Cos(val)}
		},
	},
	{ // 49: mathTan
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			val := 0.0
			if f, ok := args[0].(*Float); ok { val = f.Value } else if i, ok := args[0].(*Integer); ok { val = float64(i.Value) }
			return &Float{Value: math.Tan(val)}
		},
	},
	{ // 50: mathSqrt
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			val := 0.0
			if f, ok := args[0].(*Float); ok { val = f.Value } else if i, ok := args[0].(*Integer); ok { val = float64(i.Value) }
			return &Float{Value: math.Sqrt(val)}
		},
	},
	{ // 51: mathPow
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			v1, v2 := 0.0, 0.0
			if f, ok := args[0].(*Float); ok { v1 = f.Value } else if i, ok := args[0].(*Integer); ok { v1 = float64(i.Value) }
			if f, ok := args[1].(*Float); ok { v2 = f.Value } else if i, ok := args[1].(*Integer); ok { v2 = float64(i.Value) }
			return &Float{Value: math.Pow(v1, v2)}
		},
	},
	{ // 52: mathLog
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			val := 0.0
			if f, ok := args[0].(*Float); ok { val = f.Value } else if i, ok := args[0].(*Integer); ok { val = float64(i.Value) }
			return &Float{Value: math.Log(val)}
		},
	},
	{ // 53: mathLog10
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			val := 0.0
			if f, ok := args[0].(*Float); ok { val = f.Value } else if i, ok := args[0].(*Integer); ok { val = float64(i.Value) }
			return &Float{Value: math.Log10(val)}
		},
	},
	{ // 54: mathExp
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			val := 0.0
			if f, ok := args[0].(*Float); ok { val = f.Value } else if i, ok := args[0].(*Integer); ok { val = float64(i.Value) }
			return &Float{Value: math.Exp(val)}
		},
	},
	{ // 55: mathAsin
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			val := 0.0
			if f, ok := args[0].(*Float); ok { val = f.Value } else if i, ok := args[0].(*Integer); ok { val = float64(i.Value) }
			return &Float{Value: math.Asin(val)}
		},
	},
	{ // 56: mathAcos
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			val := 0.0
			if f, ok := args[0].(*Float); ok { val = f.Value } else if i, ok := args[0].(*Integer); ok { val = float64(i.Value) }
			return &Float{Value: math.Acos(val)}
		},
	},
	{ // 57: mathAtan
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			val := 0.0
			if f, ok := args[0].(*Float); ok { val = f.Value } else if i, ok := args[0].(*Integer); ok { val = float64(i.Value) }
			return &Float{Value: math.Atan(val)}
		},
	},
	{ // 58: mathAtan2
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			v1, v2 := 0.0, 0.0
			if f, ok := args[0].(*Float); ok { v1 = f.Value } else if i, ok := args[0].(*Integer); ok { v1 = float64(i.Value) }
			if f, ok := args[1].(*Float); ok { v2 = f.Value } else if i, ok := args[1].(*Integer); ok { v2 = float64(i.Value) }
			return &Float{Value: math.Atan2(v1, v2)}
		},
	},
	{ // 59: mathAbs
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			if f, ok := args[0].(*Float); ok { return &Float{Value: math.Abs(f.Value)} }
			if i, ok := args[0].(*Integer); ok {
				val := i.Value
				if val < 0 { val = -val }
				return &Integer{Value: val}
			}
			return NULL
		},
	},
	{ // 60: mathCeil
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			val := 0.0
			if f, ok := args[0].(*Float); ok { val = f.Value } else if i, ok := args[0].(*Integer); ok { return i }
			return &Float{Value: math.Ceil(val)}
		},
	},
	{ // 61: mathFloor
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			val := 0.0
			if f, ok := args[0].(*Float); ok { val = f.Value } else if i, ok := args[0].(*Integer); ok { return i }
			return &Float{Value: math.Floor(val)}
		},
	},
	{ // 62: strToLower
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			s, ok := args[0].(*String)
			if !ok { return NULL }
			return &String{Value: strings.ToLower(s.Value)}
		},
	},
	{ // 63: strToUpper
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			s, ok := args[0].(*String)
			if !ok { return NULL }
			return &String{Value: strings.ToUpper(s.Value)}
		},
	},
	{ // 64: strTrim
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			s, ok1 := args[0].(*String)
			cutset, ok2 := args[1].(*String)
			if !ok1 || !ok2 { return NULL }
			return &String{Value: strings.Trim(s.Value, cutset.Value)}
		},
	},
	{ // 65: strTrimSpace
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			s, ok := args[0].(*String)
			if !ok { return NULL }
			return &String{Value: strings.TrimSpace(s.Value)}
		},
	},
	{ // 66: strSplit
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			s, ok1 := args[0].(*String)
			sep, ok2 := args[1].(*String)
			if !ok1 || !ok2 { return NULL }
			res := strings.Split(s.Value, sep.Value)
			elements := make([]Object, len(res))
			for i, str := range res {
				elements[i] = &String{Value: str}
			}
			return &Array{Elements: elements}
		},
	},
	{ // 67: strJoin
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			arr, ok1 := args[0].(*Array)
			sep, ok2 := args[1].(*String)
			if !ok1 || !ok2 { return NULL }
			res := make([]string, len(arr.Elements))
			for i, e := range arr.Elements {
				res[i] = e.Inspect()
			}
			return &String{Value: strings.Join(res, sep.Value)}
		},
	},
	{ // 68: strContains
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			s, ok1 := args[0].(*String)
			sub, ok2 := args[1].(*String)
			if !ok1 || !ok2 { return NULL }
			if strings.Contains(s.Value, sub.Value) {
				return TRUE
			}
			return FALSE
		},
	},
	{ // 69: strHasPrefix
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			s, ok1 := args[0].(*String)
			pre, ok2 := args[1].(*String)
			if !ok1 || !ok2 { return NULL }
			if strings.HasPrefix(s.Value, pre.Value) {
				return TRUE
			}
			return FALSE
		},
	},
	{ // 70: strHasSuffix
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			s, ok1 := args[0].(*String)
			suf, ok2 := args[1].(*String)
			if !ok1 || !ok2 { return NULL }
			if strings.HasSuffix(s.Value, suf.Value) {
				return TRUE
			}
			return FALSE
		},
	},
	{ // 71: strIndex
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			s, ok1 := args[0].(*String)
			sub, ok2 := args[1].(*String)
			if !ok1 || !ok2 { return NULL }
			return &Integer{Value: int64(strings.Index(s.Value, sub.Value))}
		},
	},
	{ // 72: strLastIndex
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			s, ok1 := args[0].(*String)
			sub, ok2 := args[1].(*String)
			if !ok1 || !ok2 { return NULL }
			return &Integer{Value: int64(strings.LastIndex(s.Value, sub.Value))}
		},
	},
	{ // 73: strReplace
		Fn: func(args ...Object) Object {
			if len(args) != 4 { return NULL }
			s, ok1 := args[0].(*String)
			old, ok2 := args[1].(*String)
			new, ok3 := args[2].(*String)
			n, ok4 := args[3].(*Integer)
			if !ok1 || !ok2 || !ok3 || !ok4 { return NULL }
			return &String{Value: strings.Replace(s.Value, old.Value, new.Value, int(n.Value))}
		},
	},
	{ // 74: strRepeat
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			s, ok1 := args[0].(*String)
			n, ok2 := args[1].(*Integer)
			if !ok1 || !ok2 { return NULL }
			return &String{Value: strings.Repeat(s.Value, int(n.Value))}
		},
	},
	{ // 75: strCount
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			s, ok1 := args[0].(*String)
			sub, ok2 := args[1].(*String)
			if !ok1 || !ok2 { return NULL }
			return &Integer{Value: int64(strings.Count(s.Value, sub.Value))}
		},
	},
	{ // 76: strFields
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			s, ok := args[0].(*String)
			if !ok { return NULL }
			res := strings.Fields(s.Value)
			elements := make([]Object, len(res))
			for i, str := range res {
				elements[i] = &String{Value: str}
			}
			return &Array{Elements: elements}
		},
	},
	{ // 77: strTrimLeft
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			s, ok1 := args[0].(*String)
			cutset, ok2 := args[1].(*String)
			if !ok1 || !ok2 { return NULL }
			return &String{Value: strings.TrimLeft(s.Value, cutset.Value)}
		},
	},
	{ // 78: strTrimRight
		Fn: func(args ...Object) Object {
			if len(args) != 2 { return NULL }
			s, ok1 := args[0].(*String)
			cutset, ok2 := args[1].(*String)
			if !ok1 || !ok2 { return NULL }
			return &String{Value: strings.TrimRight(s.Value, cutset.Value)}
		},
	},
	{ // 79: strIsAlpha
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			s, ok := args[0].(*String)
			if !ok { return NULL }
			if s.Value == "" { return FALSE }
			for _, r := range s.Value {
				if !unicode.IsLetter(r) { return FALSE }
			}
			return TRUE
		},
	},
	{ // 80: strIsDigit
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			s, ok := args[0].(*String)
			if !ok { return NULL }
			if s.Value == "" { return FALSE }
			for _, r := range s.Value {
				if !unicode.IsDigit(r) { return FALSE }
			}
			return TRUE
		},
	},
	{ // 81: strIsSpace
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			s, ok := args[0].(*String)
			if !ok { return NULL }
			if s.Value == "" { return FALSE }
			for _, r := range s.Value {
				if !unicode.IsSpace(r) { return FALSE }
			}
			return TRUE
		},
	},
	{ // 82: strReverse
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			s, ok := args[0].(*String)
			if !ok { return NULL }
			runes := []rune(s.Value)
			for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
				runes[i], runes[j] = runes[j], runes[i]
			}
			return &String{Value: string(runes)}
		},
	},
	{ // 83: ioReadInput
		Fn: func(args ...Object) Object {
			if len(args) == 1 {
				if prompt, ok := args[0].(*String); ok {
					fmt.Print(prompt.Value)
				}
			}
			reader := bufio.NewReader(os.Stdin)
			text, _ := reader.ReadString('\n')
			return &String{Value: strings.TrimRight(text, "\r\n")}
		},
	},
	{ // 84: arrayPush
		Fn: func(args ...Object) Object {
			if len(args) < 2 { return NULL }
			arr, ok := args[0].(*Array)
			if !ok { return &Error{Message: "first argument to push must be an array"} }
			arr.Elements = append(arr.Elements, args[1:]...)
			return arr
		},
	},
	{ // 85: arrayPop
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			arr, ok := args[0].(*Array)
			if !ok { return &Error{Message: "argument to pop must be an array"} }
			if len(arr.Elements) == 0 { return NULL }
			last := arr.Elements[len(arr.Elements)-1]
			arr.Elements = arr.Elements[:len(arr.Elements)-1]
			return last
		},
	},
	{ // 86: arraySlice
		Fn: func(args ...Object) Object {
			if len(args) < 2 { return NULL }
			arr, ok := args[0].(*Array)
			if !ok { return NULL }
			start, ok1 := args[1].(*Integer)
			if !ok1 { return NULL }
			
			end := int64(len(arr.Elements))
			if len(args) == 3 {
				if e, ok2 := args[2].(*Integer); ok2 {
					end = e.Value
				}
			}
			
			if start.Value < 0 || end > int64(len(arr.Elements)) || start.Value > end {
				return &Array{Elements: []Object{}}
			}
			
			newElements := make([]Object, end-start.Value)
			copy(newElements, arr.Elements[start.Value:end])
			return &Array{Elements: newElements}
		},
	},
	{ // 87: arraySort
		Fn: func(args ...Object) Object {
			if len(args) != 1 { return NULL }
			arr, ok := args[0].(*Array)
			if !ok { return NULL }
			
			sort.Slice(arr.Elements, func(i, j int) bool {
				// Simple lexicographical sort based on Inspect() for now
				return arr.Elements[i].Inspect() < arr.Elements[j].Inspect()
			})
			return arr
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
