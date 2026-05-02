package optimizer

import (
	"github.com/byteme/compiler/ast"
)

type Optimizer struct{}

func New() *Optimizer {
	return &Optimizer{}
}

func (o *Optimizer) Optimize(node ast.Node) ast.Node {
	switch n := node.(type) {
	case *ast.Program:
		n.Statements = o.optimizeStatements(n.Statements)
		return n

	case *ast.BlockStatement:
		n.Statements = o.optimizeStatements(n.Statements)
		return n

	case *ast.ExpressionStatement:
		optimized := o.Optimize(n.Expression)
		if optimized == nil {
			return nil
		}
		if stmt, ok := optimized.(ast.Statement); ok {
			return stmt
		}
		n.Expression = optimized.(ast.Expression)
		return n

	case *ast.IfExpression:
		cond := o.Optimize(n.Condition)
		if cond == nil {
			return nil
		}
		n.Condition = cond.(ast.Expression)
		
		// Constant folding for if-expressions
		if boolLit, ok := n.Condition.(*ast.BooleanLiteral); ok {
			if boolLit.Value {
				return o.Optimize(n.Consequence)
			} else {
				if n.Alternative != nil {
					return o.Optimize(n.Alternative)
				}
				return nil // Entire if-expression is dead
			}
		}
		
		n.Consequence = o.Optimize(n.Consequence).(*ast.BlockStatement)
		if n.Alternative != nil {
			alt := o.Optimize(n.Alternative)
			if alt != nil {
				n.Alternative = alt.(*ast.BlockStatement)
			} else {
				n.Alternative = nil
			}
		}
		return n

	case *ast.FunctionLiteral:
		n.Body = o.Optimize(n.Body).(*ast.BlockStatement)
		return n

	case *ast.NamespaceLiteral:
		n.Body = o.Optimize(n.Body).(*ast.BlockStatement)
		return n

	case *ast.InfixExpression:
		left := o.Optimize(n.Left)
		right := o.Optimize(n.Right)
		if left == nil || right == nil {
			return nil
		}
		n.Left = left.(ast.Expression)
		n.Right = right.(ast.Expression)
		return n

	case *ast.PrefixExpression:
		right := o.Optimize(n.Right)
		if right == nil {
			return nil
		}
		n.Right = right.(ast.Expression)
		return n
	}

	return node
}

func (o *Optimizer) optimizeStatements(statements []ast.Statement) []ast.Statement {
	newStatements := []ast.Statement{}

	for _, stmt := range statements {
		optimized := o.Optimize(stmt)
		if optimized == nil {
			continue
		}
		
		// If it's a block that was simplified to statements, flatten it
		// (Optional, but helps with nested pruning)
		
		newStatements = append(newStatements, optimized.(ast.Statement))

		// Dead Code Elimination: if we hit a return, everything after is dead
		if _, ok := optimized.(*ast.ReturnStatement); ok {
			break
		}
	}

	return newStatements
}
