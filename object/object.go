package object

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"github.com/byteme/compiler/ast"
	"github.com/byteme/compiler/environment"
)

var (
	NULL  = &Null{}
	TRUE  = &Boolean{Value: true}
	FALSE = &Boolean{Value: false}
)

type ObjectType string

const (
	INTEGER_OBJ      = "INTEGER"
	FLOAT_OBJ        = "FLOAT"
	BOOLEAN_OBJ      = "BOOLEAN"
	STRING_OBJ       = "STRING"
	CHAR_OBJ         = "CHAR"
	NULL_OBJ         = "NULL"
	RETURN_VALUE_OBJ = "RETURN_VALUE"
	FUNCTION_OBJ     = "FUNCTION"
	FUTURE_OBJ       = "FUTURE"
	NAMESPACE_OBJ    = "NAMESPACE"
	ARRAY_OBJ        = "array"
	CHANNEL_OBJ      = "channel"
	BUILTIN_OBJ      = "builtin"
	STRUCT_LITERAL_OBJ = "STRUCT_LITERAL"
	STRUCT_OBJ         = "STRUCT_INSTANCE"
	MAP_OBJ            = "map"
	ERROR_OBJ          = "ERROR"
	ITERATOR_OBJ       = "ITERATOR"
)

type Object interface {
	Type() ObjectType
	Inspect() string
}

type Integer struct{ Value int64 }
func (i *Integer) Type() ObjectType { return INTEGER_OBJ }
func (i *Integer) Inspect() string  { return fmt.Sprintf("%d", i.Value) }

type Float struct{ Value float64 }
func (f *Float) Type() ObjectType { return FLOAT_OBJ }
func (f *Float) Inspect() string  { return fmt.Sprintf("%g", f.Value) }

type Boolean struct{ Value bool }
func (b *Boolean) Type() ObjectType { return BOOLEAN_OBJ }
func (b *Boolean) Inspect() string  { return fmt.Sprintf("%t", b.Value) }

type String struct{ Value string }
func (s *String) Type() ObjectType { return STRING_OBJ }
func (s *String) Inspect() string  { return s.Value }

type Char struct{ Value rune }
func (c *Char) Type() ObjectType { return CHAR_OBJ }
func (c *Char) Inspect() string  { return string(c.Value) }

type Null struct{}
func (n *Null) Type() ObjectType { return NULL_OBJ }
func (n *Null) Inspect() string  { return "null" }

type ReturnValue struct{ Value Object }
func (rv *ReturnValue) Type() ObjectType { return RETURN_VALUE_OBJ }
func (rv *ReturnValue) Inspect() string  { return rv.Value.Inspect() }

type Function struct {
	Parameters []*ast.Parameter
	Body       *ast.BlockStatement
	Env        *environment.Environment
	IsAsync    bool
}
func (f *Function) Type() ObjectType { return FUNCTION_OBJ }
func (f *Function) Inspect() string  { return "fn" }

type CompiledFunction struct {
	Instructions  []byte
	NumLocals     int
	NumParameters int
	IsAsync       bool
	SourceMap     map[int]int // offset -> line number
	Name          string
	Filename      string
}
func (cf *CompiledFunction) Type() ObjectType { return "COMPILED_FUNCTION" }
func (cf *CompiledFunction) Inspect() string  { return fmt.Sprintf("CompiledFunction[%p]", cf) }

type Closure struct {
	Fn   *CompiledFunction
	Free []Object
}
func (c *Closure) Type() ObjectType { return "CLOSURE" }
func (c *Closure) Inspect() string  { return fmt.Sprintf("Closure[%p]", c) }

// Future represents a value that will be available later (for async/await)
type Future struct {
	ValueChan chan Object
	result    Object
	resolved  bool
}
func (f *Future) Type() ObjectType { return FUTURE_OBJ }
func (f *Future) Inspect() string  { return "future" }
func (f *Future) Get() Object {
	if f.resolved {
		return f.result
	}
	f.result = <-f.ValueChan
	f.resolved = true
	return f.result
}

type Namespace struct {
	Name string
	Env  *environment.Environment
}
func (ns *Namespace) Type() ObjectType { return NAMESPACE_OBJ }
func (ns *Namespace) Inspect() string  { return fmt.Sprintf("namespace %s", ns.Name) }

type Array struct {
	Elements []Object
}

func (a *Array) Type() ObjectType { return ARRAY_OBJ }
func (a *Array) Inspect() string {
	var out bytes.Buffer
	out.WriteString("[")
	for i, e := range a.Elements {
		out.WriteString(e.Inspect())
		if i < len(a.Elements)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteString("]")
	return out.String()
}

type Channel struct {
	Value chan Object
}
func (c *Channel) Type() ObjectType { return CHANNEL_OBJ }
func (c *Channel) Inspect() string  { return fmt.Sprintf("channel(%p)", c.Value) }

type BuiltinFn func(args ...Object) Object

func GetVMContext() (*[]Object, *[]Object) {
	return vmConstantsPtr, vmGlobalsPtr
}

type Builtin struct {
	Fn BuiltinFn
}

func (b *Builtin) Type() ObjectType { return BUILTIN_OBJ }
func (b *Builtin) Inspect() string  { return "builtin function" }

type StructLiteral struct {
	Name       string
	ParentName string
	Parent     *StructLiteral
	Fields     []*ast.Parameter
	Methods    map[string]*CompiledFunction
}
func (s *StructLiteral) Type() ObjectType { return STRUCT_LITERAL_OBJ }
func (s *StructLiteral) Inspect() string  { return fmt.Sprintf("struct %s", s.Name) }

func (s *StructLiteral) GetAllFields() []*ast.Parameter {
	if s.Parent == nil {
		return s.Fields
	}
	// Avoid infinite recursion just in case
	return append(s.Parent.GetAllFields(), s.Fields...)
}

type StructInstance struct {
	Definition *StructLiteral
	Fields     map[string]Object
}
func (s *StructInstance) Type() ObjectType { return STRUCT_OBJ }
func (s *StructInstance) Inspect() string {
	var out bytes.Buffer
	out.WriteString(s.Definition.Name)
	out.WriteString("{")
	for k, v := range s.Fields {
		out.WriteString(fmt.Sprintf("%s: %s, ", k, v.Inspect()))
	}
	out.WriteString("}")
	return out.String()
}

type BoundMethod struct {
	Receiver Object
	Method   *CompiledFunction
}
func (bm *BoundMethod) Type() ObjectType { return "BOUND_METHOD" }
func (bm *BoundMethod) Inspect() string  { return fmt.Sprintf("BoundMethod[%p]", bm.Method) }

type MapPair struct {
	Key   Object
	Value Object
}

type Map struct {
	Pairs map[string]MapPair
}

func (m *Map) Type() ObjectType { return MAP_OBJ }
func (m *Map) Inspect() string {
	var out bytes.Buffer
	out.WriteString("{")
	for _, pair := range m.Pairs {
		out.WriteString(fmt.Sprintf("%s: %s, ", pair.Key.Inspect(), pair.Value.Inspect()))
	}
	out.WriteString("}")
	return out.String()
}

func (m *Map) Get(key string) (Object, bool) {
	pair, ok := m.Pairs[key]
	if !ok {
		return nil, false
	}
	return pair.Value, true
}

func NewStringMap(m map[string]Object) *Map {
	pairs := make(map[string]MapPair)
	for k, v := range m {
		pairs[k] = MapPair{Key: &String{Value: k}, Value: v}
	}
	return &Map{Pairs: pairs}
}

type Error struct {
	Message string
}
func (e *Error) Type() ObjectType { return ERROR_OBJ }
func (e *Error) Inspect() string  { return "ERROR: " + e.Message }
func (e *Error) Error() string    { return e.Message }

// FileHandle represents an open file
type FileHandle struct {
	File *os.File
}
func (f *FileHandle) Type() ObjectType { return "FILE_HANDLE" }
func (f *FileHandle) Inspect() string  { return fmt.Sprintf("FileHandle[%p]", f.File) }

type ArrayIterator struct {
	Array *Array
	Index int
}
func (ai *ArrayIterator) Type() ObjectType { return ITERATOR_OBJ }
func (ai *ArrayIterator) Inspect() string  { return "ArrayIterator" }
func (ai *ArrayIterator) Next() (Object, bool) {
	if ai.Index >= len(ai.Array.Elements) {
		return nil, false
	}
	val := ai.Array.Elements[ai.Index]
	ai.Index++
	return val, true
}

type MapIterator struct {
	Keys   []string
	Values []Object
	Index  int
}
func (mi *MapIterator) Type() ObjectType { return ITERATOR_OBJ }
func (mi *MapIterator) Inspect() string  { return "MapIterator" }
func (mi *MapIterator) Next() (Object, bool) {
	if mi.Index >= len(mi.Keys) {
		return nil, false
	}
	val := mi.Values[mi.Index]
	mi.Index++
	return val, true
}

type Generator struct {
	VMState interface{} // Will hold the VM pointer to avoid circular dependency in object package
}
var GeneratorNext func(g *Generator) (Object, bool)
var MakeGenerator func(fn *Closure, args []Object) Object

func (g *Generator) Type() ObjectType { return ITERATOR_OBJ }
func (g *Generator) Inspect() string  { return "Generator" }
func (g *Generator) Next() (Object, bool) {
	if GeneratorNext != nil {
		return GeneratorNext(g)
	}
	return nil, false
}

type EnumVariantDef struct {
	Name  string
	Types []string
}

type EnumDefinition struct {
	Name     string
	Variants map[string]*EnumVariantDef
}

func (e *EnumDefinition) Type() ObjectType { return "ENUM_DEFINITION" }
func (e *EnumDefinition) Inspect() string  { return fmt.Sprintf("enum %s", e.Name) }

type EnumInstance struct {
	Definition *EnumDefinition
	Variant    string
	Values     []Object
}

func (e *EnumInstance) Type() ObjectType { return "ENUM_INSTANCE" }
func (e *EnumInstance) Inspect() string {
	if len(e.Values) == 0 {
		return fmt.Sprintf("%s.%s", e.Definition.Name, e.Variant)
	}
	vals := make([]string, len(e.Values))
	for i, v := range e.Values {
		vals[i] = v.Inspect()
	}
	return fmt.Sprintf("%s.%s(%s)", e.Definition.Name, e.Variant, strings.Join(vals, ", "))
}

type EnumConstructor struct {
	Definition *EnumDefinition
	Variant    string
}

func (e *EnumConstructor) Type() ObjectType { return "ENUM_CONSTRUCTOR" }
func (e *EnumConstructor) Inspect() string  { return fmt.Sprintf("constructor %s.%s", e.Definition.Name, e.Variant) }
