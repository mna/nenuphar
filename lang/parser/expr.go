package parser

import (
	"github.com/mna/nenuphar/lang/ast"
	"github.com/mna/nenuphar/lang/token"
)

func (p *parser) parseExpr() ast.Expr {
	panic("unimplemented")
	//return p.parseSubExpr(0)
}

var (
	binopPriority = [...]struct{ left, right int }{
		token.OR:  {1, 1},
		token.AND: {2, 2},
		token.LT:  {3, 3}, token.LE: {3, 3}, token.GT: {3, 3},
		token.GE: {3, 3}, token.EQ: {3, 3}, token.BANGEQ: {3, 3},
		token.PIPE:      {4, 4},
		token.TILDE:     {5, 5},
		token.AMPERSAND: {6, 6},
		token.LTLT:      {7, 7}, token.GTGT: {7, 7},
		token.PLUS: {10, 10}, token.MINUS: {10, 10},
		token.STAR: {11, 11}, token.SLASH: {11, 11},
		token.PERCENT: {11, 11}, token.SLASHSLASH: {11, 11},
		token.CIRCUMFLEX: {14, 13}, // right associative
	}
	unopPriority = 12
)

func (p *parser) parseIdentExpr() *ast.IdentExpr {
	var exp ast.IdentExpr
	exp.Lit = p.val.Raw
	exp.Start = p.expect(token.IDENT)
	return &exp
}

func (p *parser) parseExprList() ([]ast.Expr, []token.Pos) {
	var exprs []ast.Expr
	var commas []token.Pos

	exprs = append(exprs, p.parseExpr())
	for p.tok == token.COMMA {
		commas = append(commas, p.expect(token.COMMA))
		exprs = append(exprs, p.parseExpr())
	}
	return exprs, commas
}
