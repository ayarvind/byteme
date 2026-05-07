package evaluator

import (
	"testing"
	"github.com/byteme/compiler/ast"
	"github.com/byteme/compiler/lexer"
	"github.com/byteme/compiler/parser"
	"github.com/byteme/compiler/environment"
	"github.com/byteme/compiler/object"
	"github.com/byteme/compiler/analyzer"
	"github.com/byteme/compiler/optimizer"
)

func testEval(input string) object.Object {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	
	env := environment.NewEnvironment()
	
	// Analyze
	a := analyzer.New(env, input, "test")
	a.Analyze(program)
	
	// Optimize
	opt := optimizer.New()
	optimized := opt.Optimize(program).(*ast.Program)
	
	return Eval(optimized, env)
}

func TestEvalIntegerExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"5", 5},
		{"10", 10},
		{"-5", -5},
		{"-10", -10},
		{"5 + 5 + 5 + 5 - 10", 10},
		{"2 * 2 * 2 * 2 * 2", 32},
		{"-50 + 100 + -50", 0},
		{"5 * 2 + 10", 20},
		{"5 + 2 * 10", 25},
		{"20 + 2 * -10", 0},
		{"50 / 2 * 2 + 10", 60},
		{"2 * (5 + 10)", 30},
		{"3 * 3 * 3 + 10", 37},
		{"3 * (3 * 3) + 10", 37},
		{"(5 + 10 * 2 + 15 / 3) * 2 + -10", 50},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testIntegerObject(t, evaluated, tt.expected)
	}
}

func TestNamespaceEvaluation(t *testing.T) {
	input := `
namespace Math {
	let pi: float = 3.14;
}
Math.pi;
`
	evaluated := testEval(input)
	float, ok := evaluated.(*object.Float)
	if !ok {
		t.Fatalf("object is not Float. got=%T (%+v)", evaluated, evaluated)
	}
	if float.Value != 3.14 {
		t.Errorf("object has wrong value. got=%g, want=3.14", float.Value)
	}
}

func TestAsyncAwait(t *testing.T) {
	input := `
async fn getData() -> int {
	return 42;
}
let res = await getData();
res;
`
	evaluated := testEval(input)
	testIntegerObject(t, evaluated, 42)
}

func testIntegerObject(t *testing.T, obj object.Object, expected int64) bool {
	result, ok := obj.(*object.Integer)
	if !ok {
		t.Errorf("object is not Integer. got=%T (%+v)", obj, obj)
		return false
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%d, want=%d",
			result.Value, expected)
		return false
	}
	return true
}
