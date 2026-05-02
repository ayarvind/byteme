package parser

import (
	"testing"

	"github.com/byteme/compiler/ast"
	"github.com/byteme/compiler/lexer"
)

func TestLetStatements(t *testing.T) {
	input := `
let x: int = 5;
let y: string = "hello";
let foobar: bool = true;
`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 3 {
		t.Fatalf("program.Statements does not contain 3 statements. got=%d",
			len(program.Statements))
	}

	tests := []struct {
		expectedIdentifier string
		expectedType       string
	}{
		{"x", "int"},
		{"y", "string"},
		{"foobar", "bool"},
	}

	for i, tt := range tests {
		stmt := program.Statements[i]
		if !testLetStatement(t, stmt, tt.expectedIdentifier, tt.expectedType) {
			return
		}
	}
}

func TestNamespaceParsing(t *testing.T) {
	input := `
public namespace Math extends Base {
	let pi: float = 3.14;
}
`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain 1 statement. got=%d",
			len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("stmt is not ast.ExpressionStatement. got=%T", program.Statements[0])
	}

	ns, ok := stmt.Expression.(*ast.NamespaceLiteral)
	if !ok {
		t.Fatalf("exp is not ast.NamespaceLiteral. got=%T", stmt.Expression)
	}

	if ns.Name.Value != "Math" {
		t.Errorf("ns.Name.Value not 'Math'. got=%s", ns.Name.Value)
	}

	if ns.Parent.Value != "Base" {
		t.Errorf("ns.Parent.Value not 'Base'. got=%s", ns.Parent.Value)
	}

	if !ns.IsPublic {
		t.Errorf("ns.IsPublic not true")
	}
}

func TestConcurrencyParsing(t *testing.T) {
	input := `
spawn compute(1, 2);
let res = await fetch("url");
`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	// Test spawn
	stmt1, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok { t.Fatalf("stmt1 not expression statement") }
	_, ok = stmt1.Expression.(*ast.SpawnExpression)
	if !ok { t.Fatalf("exp not spawn expression. got=%T", stmt1.Expression) }

	// Test await
	stmt2, ok := program.Statements[1].(*ast.LetStatement)
	if !ok { t.Fatalf("stmt2 not let statement") }
	_, ok = stmt2.Value.(*ast.AwaitExpression)
	if !ok { t.Fatalf("value not await expression. got=%T", stmt2.Value) }
}

func TestStructParsing(t *testing.T) {
	input := `
struct User {
	id: int,
	name: string
}
`
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	stmt := program.Statements[0].(*ast.ExpressionStatement)
	str := stmt.Expression.(*ast.StructLiteral)

	if str.Name.Value != "User" {
		t.Errorf("struct name not 'User'. got=%s", str.Name.Value)
	}

	if len(str.Fields) != 2 {
		t.Errorf("fields length not 2. got=%d", len(str.Fields))
	}
}

func checkParserErrors(t *testing.T, p *Parser) {
	errors := p.Errors()
	if len(errors) == 0 {
		return
	}

	t.Errorf("parser has %d errors", len(errors))
	for _, msg := range errors {
		t.Errorf("parser error: %q", msg)
	}
	t.FailNow()
}

func testLetStatement(t *testing.T, s ast.Statement, name string, typeName string) bool {
	if s.TokenLiteral() != "let" {
		t.Errorf("s.TokenLiteral not 'let'. got=%q", s.TokenLiteral())
		return false
	}

	letStmt, ok := s.(*ast.LetStatement)
	if !ok {
		t.Errorf("s not *ast.LetStatement. got=%T", s)
		return false
	}

	if letStmt.Name.Value != name {
		t.Errorf("letStmt.Name.Value not '%s'. got=%s", name, letStmt.Name.Value)
		return false
	}

	if letStmt.Type != typeName {
		t.Errorf("letStmt.Type not '%s'. got=%s", typeName, letStmt.Type)
		return false
	}

	return true
}
