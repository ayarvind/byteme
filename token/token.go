package token

type TokenType string

const (
	ILLEGAL TokenType = "ILLEGAL"
	EOF     TokenType = "EOF"

	// Identifiers + literals
	IDENT  TokenType = "IDENT"  // add, foobar, x, y, ...
	INT    TokenType = "INT"    // 1343456
	FLOAT  TokenType = "FLOAT"  // 12.34
	STRING TokenType = "STRING" // "hello world"

	// Operators
	ASSIGN   TokenType = "="
	PLUS     TokenType = "+"
	MINUS    TokenType = "-"
	BANG     TokenType = "!"
	ASTERISK TokenType = "*"
	SLASH    TokenType = "/"

	LT TokenType = "<"
	GT TokenType = ">"
	LTE TokenType = "<="
	GTE TokenType = ">="

	EQ     TokenType = "=="
	NOT_EQ TokenType = "!="

	// Delimiters
	COMMA     TokenType = ","
	SEMICOLON TokenType = ";"
	COLON     TokenType = ":"
	LPAREN    TokenType = "("
	RPAREN    TokenType = ")"
	LBRACE    TokenType = "{"
	RBRACE    TokenType = "}"
	LBRACKET  TokenType = "["
	RBRACKET  TokenType = "]"
	ARROW     TokenType = "->"
	DOT       TokenType = "."

	// Keywords
	FUNCTION  TokenType = "FUNCTION"
	LET       TokenType = "LET"
	CONST     TokenType = "CONST"
	TRUE      TokenType = "TRUE"
	FALSE     TokenType = "FALSE"
	IF        TokenType = "IF"
	ELSE      TokenType = "ELSE"
	RETURN    TokenType = "RETURN"
	NAMESPACE TokenType = "NAMESPACE"
	PUBLIC    TokenType = "PUBLIC"
	PRIVATE   TokenType = "PRIVATE"
	EXTENDS   TokenType = "EXTENDS"
	ASYNC     TokenType = "ASYNC"
	AWAIT     TokenType = "AWAIT"
	SPAWN     TokenType = "SPAWN"
	STRUCT    TokenType = "STRUCT"
	WHILE     TokenType = "WHILE"
)

type Token struct {
	Type    TokenType
	Literal string
}

var keywords = map[string]TokenType{
	"fn":        FUNCTION,
	"let":       LET,
	"const":     CONST,
	"true":      TRUE,
	"false":     FALSE,
	"if":        IF,
	"else":      ELSE,
	"return":    RETURN,
	"namespace": NAMESPACE,
	"public":    PUBLIC,
	"private":   PRIVATE,
	"extends":   EXTENDS,
	"async":     ASYNC,
	"await":     AWAIT,
	"spawn":     SPAWN,
	"struct":    STRUCT,
	"while":     WHILE,
}

func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
