package analyzer

import (
	"testing"
	"github.com/byteme/compiler/lexer"
	"github.com/byteme/compiler/parser"
	"github.com/byteme/compiler/environment"
	"strings"
)

func TestTypeMismatch(t *testing.T) {
	input := `
let x: int = 5;
let y: string = x;
`
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	
	env := environment.NewEnvironment()
	a := New(env, input, "test")
	a.Analyze(program)

	if len(a.Errors()) == 0 {
		t.Errorf("expected type mismatch error, got none")
	}
	
	expected := "type mismatch: cannot assign int to string"
	if !strings.Contains(a.Errors()[0], expected) {
		t.Errorf("wrong error message. expected to contain %q, got=%q", expected, a.Errors()[0])
	}
}

func TestUndefinedVariable(t *testing.T) {
	input := `
let x: int = y + 5;
`
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	
	env := environment.NewEnvironment()
	a := New(env, input, "test")
	a.Analyze(program)

	if len(a.Errors()) == 0 {
		t.Errorf("expected undefined variable error, got none")
	}
	
	expected := "undefined variable: y"
	if !strings.Contains(a.Errors()[0], expected) {
		t.Errorf("wrong error message. expected to contain %q, got=%q", expected, a.Errors()[0])
	}
}

func TestNamespaceScope(t *testing.T) {
	input := `
namespace Math {
	let pi: float = 3.14;
}
let radius: float = pi;
`
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	
	env := environment.NewEnvironment()
	a := New(env, input, "test")
	a.Analyze(program)

	// 'pi' is inside Math namespace, so it should be undefined in global scope
	if len(a.Errors()) == 0 {
		t.Errorf("expected undefined variable error for 'pi', got none")
	}
}
