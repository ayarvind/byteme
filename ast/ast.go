package ast

import (
	"bytes"
	"github.com/byteme/compiler/token"
)

type Node interface {
	TokenLiteral() string
	String()       string
}

type Statement interface {
	Node
	statementNode()
}

type Expression interface {
	Node
	expressionNode()
}

type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 { return p.Statements[0].TokenLiteral() }
	return ""
}

func (p *Program) String() string {
	var out bytes.Buffer
	for _, s := range p.Statements { out.WriteString(s.String()) }
	return out.String()
}

// --- Statements ---

type ImportStatement struct {
	Token   token.Token // The 'import' or 'from' token
	Path    *StringLiteral
	Name    *Identifier   // Optional alias or name
	Imports []*Identifier // e.g., from "path" import A, B
}
func (is *ImportStatement) statementNode()       {}
func (is *ImportStatement) TokenLiteral() string { return is.Token.Literal }
func (is *ImportStatement) String() string {
	var out bytes.Buffer
	if is.Token.Type == token.FROM {
		out.WriteString("from " + is.Path.String() + " import ")
		for i, imp := range is.Imports {
			out.WriteString(imp.String())
			if i < len(is.Imports)-1 { out.WriteString(", ") }
		}
	} else {
		out.WriteString(is.TokenLiteral() + " " + is.Path.String())
		if is.Name != nil {
			out.WriteString(" as " + is.Name.String())
		}
	}
	out.WriteString(";")
	return out.String()
}

type LetStatement struct {
	Token token.Token
	Name  *Identifier
	Value Expression
	Type  string
}
func (ls *LetStatement) statementNode()       {}
func (ls *LetStatement) TokenLiteral() string { return ls.Token.Literal }
func (ls *LetStatement) String() string {
	var out bytes.Buffer
	out.WriteString(ls.TokenLiteral() + " " + ls.Name.String() + ": " + ls.Type + " = " + ls.Value.String() + ";")
	return out.String()
}

type ConstStatement struct {
	Token token.Token
	Name  *Identifier
	Value Expression
	Type  string
}
func (cs *ConstStatement) statementNode()       {}
func (cs *ConstStatement) TokenLiteral() string { return cs.Token.Literal }
func (cs *ConstStatement) String() string {
	var out bytes.Buffer
	out.WriteString(cs.TokenLiteral() + " " + cs.Name.String() + ": " + cs.Type + " = " + cs.Value.String() + ";")
	return out.String()
}

type AssignmentStatement struct {
	Token token.Token // The = token
	Left  Expression  // Identifier or IndexExpression
	Value Expression
}
func (as *AssignmentStatement) statementNode()       {}
func (as *AssignmentStatement) TokenLiteral() string { return as.Token.Literal }
func (as *AssignmentStatement) String() string {
	return as.Left.String() + " = " + as.Value.String() + ";"
}

type WhileStatement struct {
	Token     token.Token // The 'while' token
	Condition Expression
	Body      *BlockStatement
}
func (ws *WhileStatement) statementNode()       {}
func (ws *WhileStatement) TokenLiteral() string { return ws.Token.Literal }
func (ws *WhileStatement) String() string {
	return "while (" + ws.Condition.String() + ") " + ws.Body.String()
}

type ReturnStatement struct {
	Token       token.Token
	ReturnValue Expression
}
func (rs *ReturnStatement) statementNode()       {}
func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }
func (rs *ReturnStatement) String() string {
	var out bytes.Buffer
	out.WriteString(rs.TokenLiteral() + " ")
	if rs.ReturnValue != nil { out.WriteString(rs.ReturnValue.String()) }
	out.WriteString(";")
	return out.String()
}

type ThrowStatement struct {
	Token token.Token // the 'throw' token
	Value Expression
}
func (ts *ThrowStatement) statementNode()       {}
func (ts *ThrowStatement) TokenLiteral() string { return ts.Token.Literal }
func (ts *ThrowStatement) String() string {
	return "throw " + ts.Value.String() + ";"
}

type TryStatement struct {
	Token      token.Token // the 'try' token
	Body       *BlockStatement
	CatchVar   *Identifier // e.g. 'e' in catch(e)
	CatchBody  *BlockStatement
	Finally    *BlockStatement
}
func (ts *TryStatement) statementNode()       {}
func (ts *TryStatement) TokenLiteral() string { return ts.Token.Literal }
func (ts *TryStatement) String() string {
	var out bytes.Buffer
	out.WriteString("try " + ts.Body.String())
	if ts.CatchBody != nil {
		out.WriteString(" catch(" + ts.CatchVar.String() + ") " + ts.CatchBody.String())
	}
	if ts.Finally != nil {
		out.WriteString(" finally " + ts.Finally.String())
	}
	return out.String()
}

type ExpressionStatement struct {
	Token      token.Token
	Expression Expression
}
func (es *ExpressionStatement) statementNode()       {}
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExpressionStatement) String() string {
	if es.Expression != nil { return es.Expression.String() }
	return ""
}

type BlockStatement struct {
	Token      token.Token
	Statements []Statement
}
func (bs *BlockStatement) statementNode()       {}
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStatement) String() string {
	var out bytes.Buffer
	out.WriteString("{")
	for _, s := range bs.Statements { out.WriteString(s.String()) }
	out.WriteString("}")
	return out.String()
}

// --- Expressions ---

type Identifier struct {
	Token token.Token
	Value string
}
func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }
func (i *Identifier) String() string       { return i.Value }

type IntegerLiteral struct {
	Token token.Token
	Value int64
}
func (il *IntegerLiteral) expressionNode()      {}
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }
func (il *IntegerLiteral) String() string       { return il.Token.Literal }

type FloatLiteral struct {
	Token token.Token
	Value float64
}
func (fl *FloatLiteral) expressionNode()      {}
func (fl *FloatLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FloatLiteral) String() string       { return fl.Token.Literal }

type StringLiteral struct {
	Token token.Token
	Value string
}
func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLiteral) String() string       { return sl.Token.Literal }

type BooleanLiteral struct {
	Token token.Token
	Value bool
}
func (bl *BooleanLiteral) expressionNode()      {}
func (bl *BooleanLiteral) TokenLiteral() string { return bl.Token.Literal }
func (bl *BooleanLiteral) String() string       { return bl.Token.Literal }

type ArrayLiteral struct {
	Token    token.Token // [
	Elements []Expression
}
func (al *ArrayLiteral) expressionNode()      {}
func (al *ArrayLiteral) TokenLiteral() string { return al.Token.Literal }
func (al *ArrayLiteral) String() string {
	var out bytes.Buffer
	out.WriteString("[")
	for i, e := range al.Elements {
		out.WriteString(e.String())
		if i < len(al.Elements)-1 { out.WriteString(", ") }
	}
	out.WriteString("]")
	return out.String()
}

type IndexExpression struct {
	Token token.Token // [
	Left  Expression
	Index Expression
}
func (ie *IndexExpression) expressionNode()      {}
func (ie *IndexExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IndexExpression) String() string {
	return "(" + ie.Left.String() + "[" + ie.Index.String() + "])"
}

type PrefixExpression struct {
	Token    token.Token
	Operator string
	Right    Expression
}
func (pe *PrefixExpression) expressionNode()      {}
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PrefixExpression) String() string {
	return "(" + pe.Operator + pe.Right.String() + ")"
}

type InfixExpression struct {
	Token    token.Token
	Left     Expression
	Operator string
	Right    Expression
}
func (oe *InfixExpression) expressionNode()      {}
func (oe *InfixExpression) TokenLiteral() string { return oe.Token.Literal }
func (oe *InfixExpression) String() string {
	return "(" + oe.Left.String() + " " + oe.Operator + " " + oe.Right.String() + ")"
}

type IfExpression struct {
	Token       token.Token
	Condition   Expression
	Consequence *BlockStatement
	Alternative *BlockStatement
}
func (ie *IfExpression) expressionNode()      {}
func (ie *IfExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *IfExpression) String() string {
	var out bytes.Buffer
	out.WriteString("if " + ie.Condition.String() + " " + ie.Consequence.String())
	if ie.Alternative != nil { out.WriteString("else " + ie.Alternative.String()) }
	return out.String()
}

type FunctionLiteral struct {
	Token          token.Token
	Name           *Identifier
	Parameters     []*Parameter
	Body           *BlockStatement
	ReturnType     string
	IsAsync        bool
	TypeParameters []*Identifier
}
type Parameter struct {
	Name *Identifier
	Type string
}
func (fl *FunctionLiteral) expressionNode()      {}
func (fl *FunctionLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FunctionLiteral) String() string {
	var out bytes.Buffer
	if fl.IsAsync { out.WriteString("async ") }
	out.WriteString("fn ")
	if fl.Name != nil { out.WriteString(fl.Name.Value) }
	out.WriteString("(...) " + fl.Body.String())
	return out.String()
}

type CallExpression struct {
	Token         token.Token
	Function      Expression
	Arguments     []Expression
	TypeArguments []string
}
func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CallExpression) String() string {
	return ce.Function.String() + "(...)"
}

type SpawnExpression struct {
	Token token.Token
	Call  *CallExpression
}
func (se *SpawnExpression) expressionNode()      {}
func (se *SpawnExpression) TokenLiteral() string { return se.Token.Literal }
func (se *SpawnExpression) String() string { return "spawn " + se.Call.String() }

type AwaitExpression struct {
	Token      token.Token
	Expression Expression
}
func (ae *AwaitExpression) expressionNode()      {}
func (ae *AwaitExpression) TokenLiteral() string { return ae.Token.Literal }
func (ae *AwaitExpression) String() string { return "await " + ae.Expression.String() }

type NamespaceLiteral struct {
	Token      token.Token
	Name       *Identifier
	Parent     *Identifier
	Body       *BlockStatement
	IsPublic   bool
}
func (nl *NamespaceLiteral) expressionNode()      {}
func (nl *NamespaceLiteral) TokenLiteral() string { return nl.Token.Literal }
func (nl *NamespaceLiteral) String() string {
	return "namespace " + nl.Name.String() + " " + nl.Body.String()
}

type StructLiteral struct {
	Token          token.Token
	Name           *Identifier
	Fields         []*Parameter
	TypeParameters []*Identifier
}
func (sl *StructLiteral) expressionNode()      {}
func (sl *StructLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StructLiteral) String() string       { return "struct " + sl.Name.String() }

type InterfaceStatement struct {
	Token   token.Token // 'interface'
	Name    *Identifier
	Methods []*MethodSignature
}
type MethodSignature struct {
	Name       *Identifier
	Parameters []*Parameter
	ReturnType string
}
func (is *InterfaceStatement) statementNode()       {}
func (is *InterfaceStatement) TokenLiteral() string { return is.Token.Literal }
func (is *InterfaceStatement) String() string       { return "interface " + is.Name.String() }

type EnumStatement struct {
	Token   token.Token // 'enum'
	Name    *Identifier
	Members []*Identifier
}
func (es *EnumStatement) statementNode()       {}
func (es *EnumStatement) TokenLiteral() string { return es.Token.Literal }
func (es *EnumStatement) String() string       { return "enum " + es.Name.String() }
