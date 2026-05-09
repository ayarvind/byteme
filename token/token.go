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
	CHAR   TokenType = "CHAR"   // 'a'
	TEMPLATE_STRING TokenType = "TEMPLATE_STRING" // `hello ${name}`

	// Operators
	ASSIGN   TokenType = "="
	PLUS     TokenType = "+"
	MINUS    TokenType = "-"
	BANG     TokenType = "!"
	ASTERISK TokenType = "*"
	SLASH    TokenType = "/"
	INC      TokenType = "++"
	DEC      TokenType = "--"

	PLUS_ASSIGN  TokenType = "+="
	MINUS_ASSIGN TokenType = "-="
	MUL_ASSIGN   TokenType = "*="
	DIV_ASSIGN   TokenType = "/="
	PIPE         TokenType = "|>"

	LT TokenType = "<"
	GT TokenType = ">"
	LTE TokenType = "<="
	GTE TokenType = ">="
	LSHIFT TokenType = "<<"
	RSHIFT TokenType = ">>"

	BIT_AND TokenType = "&"
	BIT_OR  TokenType = "|"
	BIT_XOR TokenType = "^"
	BIT_NOT TokenType = "~"

	EQ     TokenType = "=="
	NOT_EQ TokenType = "!="

	AND    TokenType = "&&"
	OR     TokenType = "||"

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
	AT        TokenType = "@"
	MOD       TokenType = "%"
	QUESTION  TokenType = "?"
	LAMBDA_ARROW TokenType = "=>"

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
	TRY       TokenType = "TRY"
	CATCH     TokenType = "CATCH"
	FINALLY   TokenType = "FINALLY"
	THROW     TokenType = "THROW"
	INTERFACE TokenType = "INTERFACE"
	IMPLEMENTS TokenType = "IMPLEMENTS"
	ENUM      TokenType = "ENUM"
	IMPORT    TokenType = "IMPORT"
	FROM      TokenType = "FROM"
	NULL       TokenType = "NULL"
	FOR        TokenType = "FOR"
	IN         TokenType = "IN"
	BREAK      TokenType = "BREAK"
	CONTINUE   TokenType = "CONTINUE"
	YIELD      TokenType = "YIELD"
	VOID       TokenType = "VOID"
	AS         TokenType = "AS"
)

type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
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
	"try":       TRY,
	"catch":     CATCH,
	"finally":   FINALLY,
	"throw":     THROW,
	"interface": INTERFACE,
	"implements": IMPLEMENTS,
	"enum":      ENUM,
	"import":    IMPORT,
	"from":      FROM,
	"null":      NULL,
	"for":       FOR,
	"in":         IN,
	"break":     BREAK,
	"continue":  CONTINUE,
	"yield":     YIELD,
	"void":      VOID,
	"as":        AS,
	"@":         AT,
}

func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
