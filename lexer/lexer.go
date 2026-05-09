package lexer

import (
	"github.com/byteme/compiler/token"
	"strings"
)

type Lexer struct {
	input        string
	position     int  // current position in input (points to current char)
	readPosition int  // current reading position in input (after current char)
	ch           byte // current char under examination
	line         int
	column       int
	LastComment  string
}

func New(input string) *Lexer {
	l := &Lexer{input: input, line: 1, column: 0}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.ch == '\n' {
		l.line++
		l.column = 0
	}
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition += 1
	l.column++
}

func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	} else {
		return l.input[l.readPosition]
	}
}

func (l *Lexer) NextToken() token.Token {
	var tok token.Token

	l.skipWhitespace()

	startLine := l.line
	startColumn := l.column

	switch l.ch {
	case '=':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.EQ, Literal: string(ch) + string(l.ch), Line: startLine, Column: startColumn}
		} else if l.peekChar() == '>' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.LAMBDA_ARROW, Literal: string(ch) + string(l.ch), Line: startLine, Column: startColumn}
		} else {
			tok = l.newToken(token.ASSIGN, l.ch)
		}
	case '"':
		tok.Type = token.STRING
		tok.Literal = l.readString()
		tok.Line = startLine
		tok.Column = startColumn
	case '+':
		if l.peekChar() == '+' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.INC, Literal: string(ch) + string(l.ch), Line: startLine, Column: startColumn}
		} else if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.PLUS_ASSIGN, Literal: string(ch) + string(l.ch), Line: startLine, Column: startColumn}
		} else {
			tok = l.newToken(token.PLUS, l.ch)
		}
	case '-':
		if l.peekChar() == '>' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.ARROW, Literal: string(ch) + string(l.ch), Line: startLine, Column: startColumn}
		} else if l.peekChar() == '-' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.DEC, Literal: string(ch) + string(l.ch), Line: startLine, Column: startColumn}
		} else if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.MINUS_ASSIGN, Literal: string(ch) + string(l.ch), Line: startLine, Column: startColumn}
		} else {
			tok = l.newToken(token.MINUS, l.ch)
		}
	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.NOT_EQ, Literal: string(ch) + string(l.ch), Line: startLine, Column: startColumn}
		} else {
			tok = l.newToken(token.BANG, l.ch)
		}
	case '/':
		if l.peekChar() == '/' {
			l.skipComment()
			return l.NextToken()
		} else if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.DIV_ASSIGN, Literal: string(ch) + string(l.ch), Line: startLine, Column: startColumn}
		} else {
			tok = l.newToken(token.SLASH, l.ch)
		}
	case '*':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.MUL_ASSIGN, Literal: string(ch) + string(l.ch), Line: startLine, Column: startColumn}
		} else {
			tok = l.newToken(token.ASTERISK, l.ch)
		}
	case '<':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.LTE, Literal: string(ch) + string(l.ch), Line: startLine, Column: startColumn}
		} else if l.peekChar() == '<' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.LSHIFT, Literal: string(ch) + string(l.ch), Line: startLine, Column: startColumn}
		} else {
			tok = l.newToken(token.LT, l.ch)
		}
	case '>':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.GTE, Literal: string(ch) + string(l.ch), Line: startLine, Column: startColumn}
		} else if l.peekChar() == '>' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.RSHIFT, Literal: string(ch) + string(l.ch), Line: startLine, Column: startColumn}
		} else {
			tok = l.newToken(token.GT, l.ch)
		}
	case '&':
		if l.peekChar() == '&' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.AND, Literal: string(ch) + string(l.ch), Line: startLine, Column: startColumn}
		} else {
			tok = l.newToken(token.BIT_AND, l.ch)
		}
	case '|':
		if l.peekChar() == '|' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.OR, Literal: string(ch) + string(l.ch), Line: startLine, Column: startColumn}
		} else if l.peekChar() == '>' {
			ch := l.ch
			l.readChar()
			tok = token.Token{Type: token.PIPE, Literal: string(ch) + string(l.ch), Line: startLine, Column: startColumn}
		} else {
			tok = l.newToken(token.BIT_OR, l.ch)
		}
	case '^':
		tok = l.newToken(token.BIT_XOR, l.ch)
	case '~':
		tok = l.newToken(token.BIT_NOT, l.ch)
	case ';':
		tok = l.newToken(token.SEMICOLON, l.ch)
	case ':':
		tok = l.newToken(token.COLON, l.ch)
	case ',':
		tok = l.newToken(token.COMMA, l.ch)
	case '{':
		tok = l.newToken(token.LBRACE, l.ch)
	case '}':
		tok = l.newToken(token.RBRACE, l.ch)
	case '(':
		tok = l.newToken(token.LPAREN, l.ch)
	case ')':
		tok = l.newToken(token.RPAREN, l.ch)
	case '[':
		tok = l.newToken(token.LBRACKET, l.ch)
	case ']':
		tok = l.newToken(token.RBRACKET, l.ch)
	case '.':
		tok = l.newToken(token.DOT, l.ch)
	case '@':
		tok = l.newToken(token.AT, l.ch)
	case '%':
		tok = l.newToken(token.MOD, l.ch)
	case '?':
		tok = l.newToken(token.QUESTION, l.ch)
	case '\'':
		tok.Type = token.CHAR
		tok.Literal = l.readCharLiteral()
		tok.Line = startLine
		tok.Column = startColumn
		return tok
	case '`':
		tok.Type = token.TEMPLATE_STRING
		tok.Literal = l.readTemplateString()
		tok.Line = startLine
		tok.Column = startColumn
	case 0:
		tok.Literal = ""
		tok.Type = token.EOF
		tok.Line = startLine
		tok.Column = startColumn
	default:
		if isLetter(l.ch) {
			tok.Literal = l.readIdentifier()
			tok.Type = token.LookupIdent(tok.Literal)
			tok.Line = startLine
			tok.Column = startColumn
			return tok
		} else if isDigit(l.ch) {
			tok.Literal, tok.Type = l.readNumber()
			tok.Line = startLine
			tok.Column = startColumn
			return tok
		} else {
			tok = l.newToken(token.ILLEGAL, l.ch)
		}
	}

	l.readChar()
	return tok
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

func (l *Lexer) skipComment() {
	start := l.position
	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}
	comment := l.input[start:l.position]
	l.LastComment = strings.TrimSpace(strings.TrimPrefix(comment, "//"))
	l.skipWhitespace()
}

func (l *Lexer) readIdentifier() string {
	position := l.position
	// First char is already verified to be a letter; subsequent chars can be letter, digit, or underscore
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

func (l *Lexer) readNumber() (string, token.TokenType) {
	position := l.position
	tokenType := token.INT
	for isDigit(l.ch) {
		l.readChar()
	}
	// Only treat '.' as decimal if the next char is also a digit (avoids eating struct dot access)
	if l.ch == '.' && isDigit(l.peekChar()) {
		tokenType = token.FLOAT
		l.readChar() // consume '.'
		for isDigit(l.ch) {
			l.readChar()
		}
	}
	return l.input[position:l.position], tokenType
}

func (l *Lexer) readString() string {
	var out []rune
	l.readChar() // skip opening quote
	
	for l.ch != '"' && l.ch != 0 {
		if l.ch == '\\' {
			l.readChar()
			switch l.ch {
			case 'n': out = append(out, '\n')
			case 't': out = append(out, '\t')
			case '"': out = append(out, '"')
			case '\\': out = append(out, '\\')
			default: out = append(out, rune(l.ch))
			}
		} else {
			out = append(out, rune(l.ch))
		}
		l.readChar()
	}
	return string(out)
}

func (l *Lexer) readTemplateString() string {
	var out []rune
	l.readChar() // skip opening `
	
	for l.ch != '`' && l.ch != 0 {
		if l.ch == '\\' {
			l.readChar()
			switch l.ch {
			case 'n': out = append(out, '\n')
			case 't': out = append(out, '\t')
			case '`': out = append(out, '`')
			case '\\': out = append(out, '\\')
			default: out = append(out, rune(l.ch))
			}
		} else {
			out = append(out, rune(l.ch))
		}
		l.readChar()
	}
	return string(out)
}

func (l *Lexer) readCharLiteral() string {
	l.readChar() // skip opening quote
	var ch byte
	if l.ch == '\\' {
		l.readChar()
		switch l.ch {
		case 'n': ch = '\n'
		case 't': ch = '\t'
		case 'r': ch = '\r'
		case '\'': ch = '\''
		case '\\': ch = '\\'
		default: ch = l.ch
		}
	} else {
		ch = l.ch
	}
	l.readChar() // consume the character
	if l.ch == '\'' {
		l.readChar() // consume closing quote
	}
	return string(ch)
}

func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

func (l *Lexer) newToken(tokenType token.TokenType, ch byte) token.Token {
	return token.Token{Type: tokenType, Literal: string(ch), Line: l.line, Column: l.column}
}

func (l *Lexer) GetRemainingInput() string {
	return l.input[l.position:]
}

func (l *Lexer) Clone() *Lexer {
	return &Lexer{
		input:        l.input,
		position:     l.position,
		readPosition: l.readPosition,
		ch:           l.ch,
	}
}
