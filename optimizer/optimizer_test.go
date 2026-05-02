package optimizer

import (
	"testing"
	"github.com/byteme/compiler/lexer"
	"github.com/byteme/compiler/parser"
	"github.com/byteme/compiler/ast"
)

func TestDeadCodeElimination(t *testing.T) {
	input := `
fn() -> int {
	let x: int = 1;
	return x;
	let y: int = 2;
}
`
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()

	opt := New()
	optimized := opt.Optimize(program).(*ast.Program)

	// Check the function body
	stmt := optimized.Statements[0].(*ast.ExpressionStatement)
	fn := stmt.Expression.(*ast.FunctionLiteral)
	
	// Should only have 2 statements: let x = 1 and return x
	if len(fn.Body.Statements) != 2 {
		t.Errorf("expected 2 statements in function body, got=%d", len(fn.Body.Statements))
	}

	if _, ok := fn.Body.Statements[1].(*ast.ReturnStatement); !ok {
		t.Errorf("last statement should be return, got=%T", fn.Body.Statements[1])
	}
}

func TestConstantIfPruning(t *testing.T) {
	input := `
if (true) {
	let a = 1;
} else {
	let b = 2;
}

if (false) {
	let c = 3;
}
`
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()

	opt := New()
	optimized := opt.Optimize(program).(*ast.Program)

	// The first 'if(true)' should be replaced by its consequence (a BlockStatement)
	// The second 'if(false)' should be removed entirely
	if len(optimized.Statements) != 1 {
		t.Errorf("expected 1 statement after optimization, got=%d", len(optimized.Statements))
	}

	if _, ok := optimized.Statements[0].(*ast.BlockStatement); !ok {
		t.Errorf("expected BlockStatement, got=%T", optimized.Statements[0])
	}
}
