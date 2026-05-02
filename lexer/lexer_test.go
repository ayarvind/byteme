package lexer

import (
	"testing"

	"github.com/byteme/compiler/token"
)

func TestNextToken(t *testing.T) {
	input := `
public namespace App extends Base {
	let x: int = 5;
	const pi: float = 3.14;

	async fn fetchData(url: string) -> string {
		spawn logger.log("fetching...");
		return await network.get(url);
	}

	fn compare(a: int, b: int) -> bool {
		if (a == b) {
			return true;
		} else {
			return false;
		}
	}
}
`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.PUBLIC, "public"},
		{token.NAMESPACE, "namespace"},
		{token.IDENT, "App"},
		{token.EXTENDS, "extends"},
		{token.IDENT, "Base"},
		{token.LBRACE, "{"},
		{token.LET, "let"},
		{token.IDENT, "x"},
		{token.COLON, ":"},
		{token.IDENT, "int"},
		{token.ASSIGN, "="},
		{token.INT, "5"},
		{token.SEMICOLON, ";"},
		{token.CONST, "const"},
		{token.IDENT, "pi"},
		{token.COLON, ":"},
		{token.IDENT, "float"},
		{token.ASSIGN, "="},
		{token.FLOAT, "3.14"},
		{token.SEMICOLON, ";"},
		{token.ASYNC, "async"},
		{token.FUNCTION, "fn"},
		{token.IDENT, "fetchData"},
		{token.LPAREN, "("},
		{token.IDENT, "url"},
		{token.COLON, ":"},
		{token.IDENT, "string"},
		{token.RPAREN, ")"},
		{token.ARROW, "->"},
		{token.IDENT, "string"},
		{token.LBRACE, "{"},
		{token.SPAWN, "spawn"},
		{token.IDENT, "logger"},
		{token.DOT, "."},
		{token.IDENT, "log"},
		{token.LPAREN, "("},
		{token.STRING, "fetching..."},
		{token.RPAREN, ")"},
		{token.SEMICOLON, ";"},
		{token.RETURN, "return"},
		{token.AWAIT, "await"},
		{token.IDENT, "network"},
		{token.DOT, "."},
		{token.IDENT, "get"},
		{token.LPAREN, "("},
		{token.IDENT, "url"},
		{token.RPAREN, ")"},
		{token.SEMICOLON, ";"},
		{token.RBRACE, "}"},
		{token.FUNCTION, "fn"},
		{token.IDENT, "compare"},
		{token.LPAREN, "("},
		{token.IDENT, "a"},
		{token.COLON, ":"},
		{token.IDENT, "int"},
		{token.COMMA, ","},
		{token.IDENT, "b"},
		{token.COLON, ":"},
		{token.IDENT, "int"},
		{token.RPAREN, ")"},
		{token.ARROW, "->"},
		{token.IDENT, "bool"},
		{token.LBRACE, "{"},
		{token.IF, "if"},
		{token.LPAREN, "("},
		{token.IDENT, "a"},
		{token.EQ, "=="},
		{token.IDENT, "b"},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.RETURN, "return"},
		{token.TRUE, "true"},
		{token.SEMICOLON, ";"},
		{token.RBRACE, "}"},
		{token.ELSE, "else"},
		{token.LBRACE, "{"},
		{token.RETURN, "return"},
		{token.FALSE, "false"},
		{token.SEMICOLON, ";"},
		{token.RBRACE, "}"},
		{token.RBRACE, "}"},
		{token.RBRACE, "}"},
		{token.EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}
