package parser

import (
	"fmt"
	"strconv"

	"github.com/byteme/compiler/ast"
	"github.com/byteme/compiler/lexer"
	"github.com/byteme/compiler/token"
)

const (
	_ int = iota
	LOWEST
	ASSIGN      // =
	EQUALS      // ==
	LESSGREATER // > or <
	SUM         // +
	PRODUCT     // *
	PREFIX      // -X or !X
	CALL        // myFunction(X)
	INDEX       // array[index]
	ACCESS      // foo.bar
)

var precedences = map[token.TokenType]int{
	token.ASSIGN:   ASSIGN,
	token.EQ:       EQUALS,
	token.NOT_EQ:   EQUALS,
	token.LT:       LESSGREATER,
	token.GT:       LESSGREATER,
	token.LTE:      LESSGREATER,
	token.GTE:      LESSGREATER,
	token.PLUS:     SUM,
	token.MINUS:    SUM,
	token.BIT_OR:   SUM,
	token.BIT_XOR:  SUM,
	token.SLASH:    PRODUCT,
	token.ASTERISK: PRODUCT,
	token.MOD:      PRODUCT,
	token.LSHIFT:   PRODUCT,
	token.RSHIFT:   PRODUCT,
	token.BIT_AND:  PRODUCT,
	token.LPAREN:   CALL,
	token.LBRACKET: INDEX,
	token.DOT:      ACCESS,
}

func (p *Parser) registerParsers() {
	p.prefixParseFns = make(map[token.TokenType]prefixParseFn)
	p.registerPrefix(token.IDENT, p.parseIdentifier)
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.FLOAT, p.parseFloatLiteral)
	p.registerPrefix(token.STRING, p.parseStringLiteral)
	p.registerPrefix(token.CHAR, p.parseCharLiteral)
	p.registerPrefix(token.BANG, p.parsePrefixExpression)
	p.registerPrefix(token.MINUS, p.parsePrefixExpression)
	p.registerPrefix(token.BIT_NOT, p.parsePrefixExpression)
	p.registerPrefix(token.TRUE, p.parseBoolean)
	p.registerPrefix(token.FALSE, p.parseBoolean)
	p.registerPrefix(token.LPAREN, p.parseGroupedExpression)
	p.registerPrefix(token.LBRACKET, p.parseArrayLiteral)
	p.registerPrefix(token.IF, p.parseIfExpression)
	p.registerPrefix(token.FUNCTION, p.parseFunctionLiteral)
	p.registerPrefix(token.ASYNC, p.parseAsyncFunctionLiteral)
	p.registerPrefix(token.SPAWN, p.parseSpawnExpression)
	p.registerPrefix(token.AWAIT, p.parseAwaitExpression)
	p.registerPrefix(token.NULL, p.parseNull)

	p.infixParseFns = make(map[token.TokenType]infixParseFn)
	p.registerInfix(token.PLUS, p.parseInfixExpression)
	p.registerInfix(token.MINUS, p.parseInfixExpression)
	p.registerInfix(token.SLASH, p.parseInfixExpression)
	p.registerInfix(token.ASTERISK, p.parseInfixExpression)
	p.registerInfix(token.MOD, p.parseInfixExpression)
	p.registerInfix(token.LSHIFT, p.parseInfixExpression)
	p.registerInfix(token.RSHIFT, p.parseInfixExpression)
	p.registerInfix(token.BIT_AND, p.parseInfixExpression)
	p.registerInfix(token.BIT_OR, p.parseInfixExpression)
	p.registerInfix(token.BIT_XOR, p.parseInfixExpression)
	p.registerInfix(token.EQ, p.parseInfixExpression)
	p.registerInfix(token.NOT_EQ, p.parseInfixExpression)
	p.registerInfix(token.LT, p.parseGenericCallExpression)
	p.registerInfix(token.GT, p.parseInfixExpression)
	p.registerInfix(token.LTE, p.parseInfixExpression)
	p.registerInfix(token.GTE, p.parseInfixExpression)
	p.registerInfix(token.LPAREN, p.parseCallExpression)
	p.registerInfix(token.LBRACKET, p.parseIndexExpression)
	p.registerInfix(token.ASSIGN, p.parseInfixExpression) // Treat as infix
	p.registerInfix(token.DOT, p.parseInfixExpression)
}

func (p *Parser) parseArrayLiteral() ast.Expression {
	array := &ast.ArrayLiteral{Token: p.curToken}
	array.Elements = p.parseExpressionList(token.RBRACKET)
	return array
}

func (p *Parser) parseIndexExpression(left ast.Expression) ast.Expression {
	exp := &ast.IndexExpression{Token: p.curToken, Left: left}

	p.nextToken()
	exp.Index = p.parseExpression(LOWEST)

	if !p.expectPeek(token.RBRACKET) {
		return nil
	}

	return exp
}

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(ast.Expression) ast.Expression
)

type Parser struct {
	l      *lexer.Lexer
	errors []string

	curToken  token.Token
	peekToken token.Token

	prefixParseFns map[token.TokenType]prefixParseFn
	infixParseFns  map[token.TokenType]infixParseFn
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
	}

	p.registerParsers()

	// Read two tokens, so curToken and peekToken are both set
	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) peekError(t token.TokenType) {
	msg := fmt.Sprintf("[%d:%d] expected next token to be %s, got %s instead",
		p.peekToken.Line, p.peekToken.Column, t, p.peekToken.Type)
	p.errors = append(p.errors, msg)
}

func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	program.Statements = []ast.Statement{}

	for p.curToken.Type != token.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}

	return program
}

func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
	case token.LET:
		return p.parseLetStatement()
	case token.CONST:
		return p.parseConstStatement()
	case token.RETURN:
		return p.parseReturnStatement()
	case token.WHILE:
		return p.parseWhileStatement()
	case token.TRY:
		return p.parseTryStatement()
	case token.THROW:
		return p.parseThrowStatement()
	case token.PUBLIC:
		p.nextToken() // consume 'public'
		switch p.curToken.Type {
		case token.NAMESPACE:
			return p.parseNamespaceStatement(true)
		case token.LET:
			return p.parseLetStatement()
		case token.CONST:
			return p.parseConstStatement()
		case token.FUNCTION:
			return p.parseExpressionStatement() // Function literals are parsed as expressions
		case token.STRUCT:
			return p.parseStructStatement()
		default:
			return nil
		}
	case token.NAMESPACE:
		return p.parseNamespaceStatement(false)
	case token.STRUCT:
		return p.parseStructStatement()
	case token.INTERFACE:
		return p.parseInterfaceStatement()
	case token.ENUM:
		return p.parseEnumStatement()
	case token.IMPORT:
		return p.parseImportStatement()
	case token.FROM:
		return p.parseFromImportStatement()
	default:
		return p.parseExpressionStatement()
	}
}

func (p *Parser) parseFromImportStatement() *ast.ImportStatement {
	stmt := &ast.ImportStatement{Token: p.curToken}

	if !p.expectPeek(token.STRING) {
		return nil
	}

	stmt.Path = &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.IMPORT) {
		return nil
	}

	stmt.Imports = []*ast.Identifier{}

	if p.peekTokenIs(token.IDENT) {
		p.nextToken()
		stmt.Imports = append(stmt.Imports, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})

		for p.peekTokenIs(token.COMMA) {
			p.nextToken() // skip comma
			p.nextToken() // move to ident
			stmt.Imports = append(stmt.Imports, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})
		}
	}

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseImportStatement() *ast.ImportStatement {
	stmt := &ast.ImportStatement{Token: p.curToken}

	if !p.expectPeek(token.STRING) {
		return nil
	}

	stmt.Path = &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}

	if p.peekTokenIs(token.IDENT) {
		// e.g. import "math" as math
		p.nextToken() 
		stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseTryStatement() *ast.TryStatement {
	stmt := &ast.TryStatement{Token: p.curToken}

	if !p.expectPeek(token.LBRACE) { return nil }
	stmt.Body = p.parseBlockStatement()

	if p.peekTokenIs(token.CATCH) {
		p.nextToken() // cur is catch
		if !p.expectPeek(token.LPAREN) { return nil }
		if !p.expectPeek(token.IDENT) { return nil }
		stmt.CatchVar = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		
		if p.peekTokenIs(token.COLON) {
			p.nextToken() // cur is :
			if !p.expectPeek(token.IDENT) { return nil }
			stmt.CatchVarType = p.curToken.Literal
		}
		
		if !p.expectPeek(token.RPAREN) { return nil }
		if !p.expectPeek(token.LBRACE) { return nil }
		stmt.CatchBody = p.parseBlockStatement()
	}

	if p.peekTokenIs(token.FINALLY) {
		p.nextToken() // cur is finally
		if !p.expectPeek(token.LBRACE) { return nil }
		stmt.Finally = p.parseBlockStatement()
	}

	return stmt
}

func (p *Parser) parseWhileStatement() *ast.WhileStatement {
	stmt := &ast.WhileStatement{Token: p.curToken}
	if !p.expectPeek(token.LPAREN) { return nil }
	p.nextToken()
	stmt.Condition = p.parseExpression(LOWEST)
	if !p.expectPeek(token.RPAREN) { return nil }
	if !p.expectPeek(token.LBRACE) { return nil }
	stmt.Body = p.parseBlockStatement()
	return stmt
}

func (p *Parser) parseThrowStatement() *ast.ThrowStatement {
	stmt := &ast.ThrowStatement{Token: p.curToken}
	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)
	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseInterfaceStatement() *ast.InterfaceStatement {
	stmt := &ast.InterfaceStatement{Token: p.curToken}
	if !p.expectPeek(token.IDENT) { return nil }
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.LBRACE) { return nil }

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.FUNCTION) {
			sig := &ast.MethodSignature{}
			if !p.expectPeek(token.IDENT) { return nil }
			sig.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
			
			if !p.expectPeek(token.LPAREN) { return nil }
			sig.Parameters = p.parseFunctionParameters()
			
			if p.peekTokenIs(token.ARROW) {
				p.nextToken()
				p.nextToken()
				sig.ReturnType = p.curToken.Literal
			}
			stmt.Methods = append(stmt.Methods, sig)
			if p.peekTokenIs(token.SEMICOLON) { p.nextToken() }
		}
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseEnumStatement() *ast.EnumStatement {
	stmt := &ast.EnumStatement{Token: p.curToken}
	if !p.expectPeek(token.IDENT) { return nil }
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.LBRACE) { return nil }
	p.nextToken() // cur is now first member or }

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.IDENT) {
			ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
			stmt.Members = append(stmt.Members, ident)
		}
		if p.peekTokenIs(token.COMMA) { p.nextToken() }
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseTypeParameters() []*ast.Identifier {
	idents := []*ast.Identifier{}
	if p.peekTokenIs(token.GT) {
		p.nextToken()
		return idents
	}
	p.nextToken()
	idents = append(idents, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})
	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		idents = append(idents, &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal})
	}
	if !p.expectPeek(token.GT) { return nil }
	return idents
}

func (p *Parser) parseLetStatement() *ast.LetStatement {
	stmt := &ast.LetStatement{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}

	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if p.peekTokenIs(token.COLON) {
		p.nextToken() // cur is :
		p.nextToken() // cur is type
		stmt.Type = p.parseTypeString()
	}

	if p.peekTokenIs(token.ASSIGN) {
		p.nextToken() // move to =
		p.nextToken() // move to expression
		stmt.Value = p.parseExpression(LOWEST)
	}

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseConstStatement() *ast.ConstStatement {
	stmt := &ast.ConstStatement{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}

	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if p.peekTokenIs(token.COLON) {
		p.nextToken() // cur is :
		p.nextToken() // cur is type
		stmt.Type = p.parseTypeString()
	}

	if !p.expectPeek(token.ASSIGN) {
		return nil
	}

	p.nextToken()

	stmt.Value = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	stmt := &ast.ReturnStatement{Token: p.curToken}

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
		return stmt
	}

	p.nextToken()

	stmt.ReturnValue = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseNamespaceStatement(isPublic bool) ast.Statement {
	stmt := &ast.NamespaceLiteral{Token: p.curToken, IsPublic: isPublic}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if p.peekTokenIs(token.EXTENDS) {
		p.nextToken()
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		stmt.Parent = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	stmt.Body = p.parseBlockStatement()

	// Return as expression statement for now or modify Program to accept it
	return &ast.ExpressionStatement{Token: stmt.Token, Expression: stmt}
}

func (p *Parser) parseStructStatement() ast.Statement {
	stmt := &ast.StructLiteral{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if p.peekTokenIs(token.LT) {
		p.nextToken()
		stmt.TypeParameters = p.parseTypeParameters()
	}

	if p.peekTokenIs(token.EXTENDS) {
		p.nextToken()
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		stmt.Parent = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	stmt.Fields = []*ast.Parameter{}

	for !p.peekTokenIs(token.RBRACE) && !p.peekTokenIs(token.EOF) {
		p.nextToken()
		fieldName := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		
		if !p.expectPeek(token.COLON) {
			return nil
		}
		
		p.nextToken() // cur is type
		fieldType := p.parseTypeString()
		
		stmt.Fields = append(stmt.Fields, &ast.Parameter{Name: fieldName, Type: fieldType})
		
		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
		}
	}

	if !p.expectPeek(token.RBRACE) {
		return nil
	}

	return &ast.ExpressionStatement{Token: stmt.Token, Expression: stmt}
}

func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {
	stmt := &ast.ExpressionStatement{Token: p.curToken}

	stmt.Expression = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseExpression(precedence int) ast.Expression {
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}
	leftExp := prefix()

	for !p.peekTokenIs(token.SEMICOLON) && precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}

		p.nextToken()

		leftExp = infix(leftExp)
	}

	return leftExp
}

func (p *Parser) parseNull() ast.Expression {
	return &ast.NullLiteral{Token: p.curToken}
}

func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	lit := &ast.IntegerLiteral{Token: p.curToken}

	value, err := strconv.ParseInt(p.curToken.Literal, 0, 64)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as integer", p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}

	lit.Value = value
	return lit
}

func (p *Parser) parseFloatLiteral() ast.Expression {
	lit := &ast.FloatLiteral{Token: p.curToken}

	value, err := strconv.ParseFloat(p.curToken.Literal, 64)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as float", p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}

	lit.Value = value
	return lit
}

func (p *Parser) parseCharLiteral() ast.Expression {
	if len(p.curToken.Literal) == 0 {
		return &ast.CharLiteral{Token: p.curToken, Value: 0}
	}
	return &ast.CharLiteral{Token: p.curToken, Value: rune(p.curToken.Literal[0])}
}

func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseBoolean() ast.Expression {
	return &ast.BooleanLiteral{Token: p.curToken, Value: p.curTokenIs(token.TRUE)}
}

func (p *Parser) parsePrefixExpression() ast.Expression {
	expression := &ast.PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
	}

	p.nextToken()

	expression.Right = p.parseExpression(PREFIX)

	return expression
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	expression := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}

	precedence := p.curPrecedence()
	p.nextToken()
	expression.Right = p.parseExpression(precedence)

	return expression
}

func (p *Parser) parseGroupedExpression() ast.Expression {
	p.nextToken()

	exp := p.parseExpression(LOWEST)

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return exp
}

func (p *Parser) parseIfExpression() ast.Expression {
	expression := &ast.IfExpression{Token: p.curToken}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	p.nextToken()
	expression.Condition = p.parseExpression(LOWEST)

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	expression.Consequence = p.parseBlockStatement()

	if p.peekTokenIs(token.ELSE) {
		p.nextToken()

		if !p.expectPeek(token.LBRACE) {
			return nil
		}

		expression.Alternative = p.parseBlockStatement()
	}

	return expression
}

func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

	p.nextToken()

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}

	return block
}

func (p *Parser) parseAsyncFunctionLiteral() ast.Expression {
	p.nextToken() // skip 'async'
	fn := p.parseFunctionLiteral().(*ast.FunctionLiteral)
	fn.IsAsync = true
	return fn
}

func (p *Parser) parseFunctionLiteral() ast.Expression {
	lit := &ast.FunctionLiteral{Token: p.curToken}

	// 1. Optional Name
	if p.peekTokenIs(token.IDENT) {
		p.nextToken()
		lit.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	}

	// 2. Optional Type Parameters
	if p.peekTokenIs(token.LT) {
		p.nextToken()
		lit.TypeParameters = p.parseTypeParameters()
	}

	// 3. Parameters (and possibly Receiver)
	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	
	params := p.parseFunctionParameters()
	
	// If followed by ANOTHER '(' or an IDENT (if we didn't have a name yet), 
	// then the first part was a receiver.
	if p.peekTokenIs(token.LPAREN) || (lit.Name == nil && p.peekTokenIs(token.IDENT)) {
		// The 'params' we just parsed is actually the Receiver
		if len(params) > 0 {
			lit.Receiver = params[0]
		}
		
		// Now parse the name if we haven't yet
		if p.peekTokenIs(token.IDENT) {
			p.nextToken()
			lit.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		}
		
		// Now parse the real parameters
		if !p.expectPeek(token.LPAREN) {
			return nil
		}
		lit.Parameters = p.parseFunctionParameters()
	} else {
		lit.Parameters = params
	}

	// 4. Return type
	if p.peekTokenIs(token.ARROW) {
		p.nextToken() // ->
		p.nextToken() // cur is type
		lit.ReturnType = p.parseTypeString()
	}

	// 5. Body
	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	lit.Body = p.parseBlockStatement()

	return lit
}

func (p *Parser) parseFunctionParameters() []*ast.Parameter {
	identifiers := []*ast.Parameter{}

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return identifiers
	}

	p.nextToken()

	ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	
	paramType := "any"
	if p.peekTokenIs(token.COLON) {
		p.nextToken() // cur is :
		p.nextToken() // cur is type
		paramType = p.parseTypeString()
	}
	
	identifiers = append(identifiers, &ast.Parameter{Name: ident, Type: paramType})

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		paramType := "any"
		if p.peekTokenIs(token.COLON) {
			p.nextToken() // cur is :
			p.nextToken() // cur is type
			paramType = p.parseTypeString()
		}
		identifiers = append(identifiers, &ast.Parameter{Name: ident, Type: paramType})
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return identifiers
}

func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {
	exp := &ast.CallExpression{Token: p.curToken, Function: function}
	
	// Support func<T, U>(...)
	if p.peekTokenIs(token.LT) {
		p.nextToken() // cur is <
		p.nextToken() // cur is first type
		exp.TypeArguments = append(exp.TypeArguments, p.curToken.Literal)
		for p.peekTokenIs(token.COMMA) {
			p.nextToken()
			p.nextToken()
			exp.TypeArguments = append(exp.TypeArguments, p.curToken.Literal)
		}
		if !p.expectPeek(token.GT) { return nil }
	}

	exp.Arguments = p.parseExpressionList(token.RPAREN)
	return exp
}

func (p *Parser) parseGenericCallExpression(left ast.Expression) ast.Expression {
	if !p.isGenericCallSpeculation() {
		return p.parseInfixExpression(left)
	}

	exp := &ast.CallExpression{Token: p.curToken, Function: left}
	p.nextToken() // move past <
	
	if !p.curTokenIs(token.GT) {
		exp.TypeArguments = append(exp.TypeArguments, p.curToken.Literal)
		for p.peekTokenIs(token.COMMA) {
			p.nextToken()
			p.nextToken()
			exp.TypeArguments = append(exp.TypeArguments, p.curToken.Literal)
		}
		if !p.expectPeek(token.GT) { return nil }
	}

	if p.peekTokenIs(token.LPAREN) {
		p.nextToken()
		exp.Arguments = p.parseExpressionList(token.RPAREN)
	}
	return exp
}

func (p *Parser) isGenericCallSpeculation() bool {
	// Pattern: curToken (<) | peekToken (IDENT) | Clone.NextToken (maybe GT)
	
	if p.peekToken.Type != token.IDENT {
		return false
	}

	l := p.l.Clone()
	
	// Scan past any other type arguments
	for {
		tok := l.NextToken()
		if tok.Type == token.COMMA {
			tok = l.NextToken()
			if tok.Type != token.IDENT { return false }
			continue
		}
		if tok.Type == token.GT {
			next := l.NextToken()
			return next.Type == token.LPAREN || next.Type == token.DOT
		}
		return false
	}
}

func (p *Parser) parseSpawnExpression() ast.Expression {
	stmt := &ast.SpawnExpression{Token: p.curToken}
	
	p.nextToken()
	
	exp := p.parseExpression(LOWEST)
	call, ok := exp.(*ast.CallExpression)
	if !ok {
		p.errors = append(p.errors, "spawn must be followed by a function call")
		return nil
	}
	
	stmt.Call = call
	return stmt
}

func (p *Parser) parseAwaitExpression() ast.Expression {
	stmt := &ast.AwaitExpression{Token: p.curToken}
	
	p.nextToken()
	
	stmt.Expression = p.parseExpression(LOWEST)
	return stmt
}

func (p *Parser) parseExpressionList(end token.TokenType) []ast.Expression {
	list := []ast.Expression{}

	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}

	p.nextToken()
	list = append(list, p.parseExpression(LOWEST))

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		list = append(list, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(end) {
		return nil
	}

	return list
}

// --- Helper Functions ---

func (p *Parser) curTokenIs(t token.TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) expectPeek(t token.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	} else {
		p.peekError(t)
		return false
	}
}

func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) registerPrefix(tokenType token.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

func (p *Parser) registerInfix(tokenType token.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

func (p *Parser) parseTypeString() string {
	typeName := p.curToken.Literal
	
	// Handle generics like array<int>
	if p.peekTokenIs(token.LT) {
		p.nextToken() // <
		p.nextToken() // cur is type inside <
		typeName += "<" + p.parseTypeString() + ">"
		if p.peekTokenIs(token.GT) {
			p.nextToken()
		}
	}

	for p.peekTokenIs(token.BIT_OR) {
		p.nextToken() // |
		p.nextToken() // cur is next type
		typeName += "|" + p.parseTypeString()
	}
	return typeName
}

func (p *Parser) noPrefixParseFnError(t token.TokenType) {
	msg := fmt.Sprintf("no prefix parse function for %s found", t)
	p.errors = append(p.errors, msg)
}
