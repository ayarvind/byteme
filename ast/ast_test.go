package ast

import (
	"testing"
	"github.com/byteme/compiler/token"
)

func TestString(t *testing.T) {
	program := &Program{
		Statements: []Statement{
			&LetStatement{
				Token: token.Token{Type: token.LET, Literal: "let"},
				Name: &Identifier{
					Token: token.Token{Type: token.IDENT, Literal: "myVar"},
					Value: "myVar",
				},
				Type: "int",
				Value: &Identifier{
					Token: token.Token{Type: token.IDENT, Literal: "anotherVar"},
					Value: "anotherVar",
				},
			},
		},
	}

	if program.String() != "let myVar: int = anotherVar;" {
		t.Errorf("program.String() wrong. got=%q", program.String())
	}
}

func TestNamespaceString(t *testing.T) {
	program := &Program{
		Statements: []Statement{
			&ExpressionStatement{
				Expression: &NamespaceLiteral{
					Token: token.Token{Type: token.NAMESPACE, Literal: "namespace"},
					Name: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "Math"}, Value: "Math"},
					IsPublic: true,
					Body: &BlockStatement{
						Token: token.Token{Type: token.LBRACE, Literal: "{"},
						Statements: []Statement{},
					},
				},
			},
		},
	}

	expected := "public namespace Math {}"
	if program.String() != expected {
		t.Errorf("namespace string wrong. expected=%q, got=%q", expected, program.String())
	}
}
