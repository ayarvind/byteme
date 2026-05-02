package environment

import (
	"fmt"
	"sync"
)

type AccessModifier int

const (
	PRIVATE AccessModifier = iota
	PUBLIC
)

type Symbol struct {
	Name   string
	Type   string
	Access AccessModifier
	IsConst bool
}

type Environment struct {
	store  map[string]Symbol
	values map[string]interface{}
	outer  *Environment
	name   string
	mu     sync.RWMutex
}

func NewEnvironment() *Environment {
	s := make(map[string]Symbol)
	v := make(map[string]interface{})
	return &Environment{store: s, values: v, outer: nil}
}

func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	return env
}

func (e *Environment) Set(name string, typeName string, access AccessModifier, isConst bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.store[name]; ok {
		return fmt.Errorf("symbol %s already defined in this scope", name)
	}
	e.store[name] = Symbol{Name: name, Type: typeName, Access: access, IsConst: isConst}
	return nil
}

func (e *Environment) Get(name string) (Symbol, bool) {
	e.mu.RLock()
	sym, ok := e.store[name]
	e.mu.RUnlock()
	if !ok && e.outer != nil {
		return e.outer.Get(name)
	}
	return sym, ok
}

func (e *Environment) GetInCurrentScope(name string) (Symbol, bool) {
	sym, ok := e.store[name]
	return sym, ok
}

// GetPublic returns a symbol only if it is public or if we are in the same environment
func (e *Environment) GetWithAccessCheck(name string, callerEnv *Environment) (Symbol, error) {
	sym, ok := e.Get(name)
	if !ok {
		return Symbol{}, fmt.Errorf("undefined symbol: %s", name)
	}

	// Simple access check: if it's private, the callerEnv must be this environment or a descendant
	if sym.Access == PRIVATE {
		if !e.isDescendant(callerEnv) {
			return Symbol{}, fmt.Errorf("cannot access private symbol: %s", name)
		}
	}

	return sym, nil
}

func (e *Environment) SetVal(name string, val interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.values[name] = val
}

func (e *Environment) SetValInScope(name string, val interface{}) bool {
	e.mu.Lock()
	_, ok := e.values[name]
	if ok {
		e.values[name] = val
		e.mu.Unlock()
		return true
	}
	e.mu.Unlock()
	if e.outer != nil {
		return e.outer.SetValInScope(name, val)
	}
	return false
}

func (e *Environment) GetVal(name string) (interface{}, bool) {
	e.mu.RLock()
	val, ok := e.values[name]
	e.mu.RUnlock()
	if !ok && e.outer != nil {
		return e.outer.GetVal(name)
	}
	return val, ok
}

func (e *Environment) isDescendant(other *Environment) bool {
	curr := other
	for curr != nil {
		if curr == e {
			return true
		}
		curr = curr.outer
	}
	return false
}
