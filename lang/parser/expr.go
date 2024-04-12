package parser

import (
	"fmt"

	"github.com/mna/nenuphar/lang/ast"
	"github.com/mna/nenuphar/lang/token"
)

func (p *parser) parseExpr() ast.Expr {
	return p.parseSubExpr(0)
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

// parses a SubExpr where the binary operator has a priority higher than the
// provided priority (for precedence climbing).
func (p *parser) parseSubExpr(priority int) ast.Expr {
	var left ast.Expr

	if p.tok.IsUnop() {
		var unop ast.UnaryOpExpr
		unop.Type = p.tok
		unop.Op = p.expect(p.tok)
		unop.Right = p.parseSubExpr(unopPriority)
		left = &unop
	} else {
		fmt.Println(">>>>> SUB EXPR ", priority, p.tok)
		left = p.parseSimpleExpr()
	}

	for p.tok.IsBinop() && binopPriority[p.tok].left > priority {
		var bin ast.BinOpExpr
		bin.Left = left
		bin.Type = p.tok
		bin.Op = p.expect(p.tok)
		bin.Right = p.parseSubExpr(binopPriority[bin.Type].right)
		left = &bin
	}
	fmt.Println(">>>>> SUB EXPR DONE ", priority, p.tok)

	return left
}

func (p *parser) parseSimpleExpr() ast.Expr {
	switch {
	case p.tok.IsAtom():
		return p.parseAtomExpr()
	case p.tok == token.LBRACE:
		return p.parseMapExpr()
	case p.tok == token.LBRACK:
		return p.parseArrayExpr()
	case p.tok == token.FUNCTION:
		return p.parseFuncExpr()
	case p.tok == token.CLASS:
		return p.parseClassExpr()
	default:
		return p.parseTupleOrSuffixedExpr()
	}
}

func (p *parser) parseAtomExpr() *ast.LiteralExpr {
	var val any
	switch p.tok {
	case token.INT:
		val = p.val.Int
	case token.FLOAT:
		val = p.val.Float
	case token.STRING:
		val = p.val.String
	}
	lit := &ast.LiteralExpr{
		Type:  p.tok,
		Raw:   p.val.Raw,
		Value: val,
	}
	lit.Start = p.expect(p.tok)
	return lit
}

func (p *parser) parseMapExpr() *ast.MapExpr {
	var expr ast.MapExpr
	expr.Lbrace = p.expect(token.LBRACE)

	var items []*ast.KeyVal
	var commas []token.Pos
	for !tokenIn(p.tok, token.RBRACE, token.EOF) {
		items = append(items, p.parseKeyVal())
		if p.tok == token.COMMA {
			// may or may not be the last, trailing comma is valid
			commas = append(commas, p.expect(token.COMMA))
		} else {
			// no comma after keyval, must be the last
			break
		}
	}

	expr.Items = items
	expr.Commas = commas
	expr.Rbrace = p.expect(token.RBRACE)
	return &expr
}

func (p *parser) parseKeyVal() *ast.KeyVal {
	var kv ast.KeyVal

	// parse the key
	switch p.tok {
	case token.LBRACK:
		kv.Lbrack = p.expect(token.LBRACK)
		kv.Key = p.parseExpr()
		kv.Rbrack = p.expect(token.RBRACK)
	case token.STRING:
		kv.Key = p.parseAtomExpr()
	case token.IDENT:
		kv.Key = p.parseIdentExpr()
	default:
		p.expect(token.IDENT, token.LBRACK, token.STRING)
		panic("unreachable")
	}

	kv.Colon = p.expect(token.COLON)
	kv.Value = p.parseExpr()
	return &kv
}

func (p *parser) parseArrayExpr() *ast.ArrayLikeExpr {
	var expr ast.ArrayLikeExpr
	expr.Type = token.LBRACK
	expr.Left = p.expect(token.LBRACK)

	var items []ast.Expr
	var commas []token.Pos
	for !tokenIn(p.tok, token.RBRACK, token.EOF) {
		items = append(items, p.parseExpr())
		if p.tok == token.COMMA {
			// may or may not be the last, trailing comma is valid
			commas = append(commas, p.expect(token.COMMA))
		} else {
			// no comma after value, must be the last
			break
		}
	}

	expr.Right = p.expect(token.RBRACK)
	return &expr
}

func (p *parser) parseFuncExpr() *ast.FuncExpr {
	var expr ast.FuncExpr
	expr.Fn = p.expect(token.FUNCTION)
	expr.Sig = p.parseFuncSignature()
	expr.Body = p.parseBlock(token.END)
	expr.End = p.expect(token.END)
	return &expr
}

func (p *parser) parseClassExpr() *ast.ClassExpr {
	var expr ast.ClassExpr
	expr.Class = p.expect(token.CLASS)
	expr.Inherits = p.parseClassInherits()
	expr.Body = p.parseClassBody()
	return &expr
}

func (p *parser) parseTupleOrSuffixedExpr() ast.Expr {
	primary, isTuple := p.parseTupleOrPrimaryExpr()
	if isTuple {
		return primary
	}

	fmt.Println(">>>>> SUFFIXED EXPR ", primary, p.tok)
loop:
	for p.tok != token.EOF {
		switch p.tok {
		case token.DOT:
			primary = p.parseDotExpr(primary)
		case token.LBRACK:
			primary = p.parseIndexExpr(primary)
		case token.LPAREN, token.LBRACE, token.STRING, token.BANG:
			primary = p.parseCallExpr(primary)
		default:
			break loop
		}
	}
	return primary
}

func (p *parser) parseTupleOrPrimaryExpr() (e ast.Expr, isTuple bool) {
	if p.tok == token.IDENT {
		return p.parseIdentExpr(), false
	}

	lparen := p.expect(token.LPAREN)
	if p.tok == token.RPAREN {
		// empty tuple
		return &ast.ArrayLikeExpr{
			Type:  token.LPAREN,
			Left:  lparen,
			Right: p.expect(token.RPAREN),
		}, true
	}

	// at this point, an expr is required
	expr := p.parseExpr()
	if p.tok == token.RPAREN {
		// paren expression, a tuple would require a trailing comma
		return &ast.ParenExpr{
			Lparen: lparen,
			Expr:   expr,
			Rparen: p.expect(token.RPAREN),
		}, false
	}

	// must be a tuple
	items := []ast.Expr{expr}
	commas := []token.Pos{p.expect(token.COMMA)}
	for !tokenIn(p.tok, token.RPAREN, token.EOF) {
		items = append(items, p.parseExpr())
		if p.tok == token.COMMA {
			// may or may not be the last, trailing comma is valid
			commas = append(commas, p.expect(token.COMMA))
		} else {
			// no comma after value, must be the last
			break
		}
	}
	return &ast.ArrayLikeExpr{
		Type:   token.LPAREN,
		Left:   lparen,
		Items:  items,
		Commas: commas,
		Right:  p.expect(token.RPAREN),
	}, true
}

func (p *parser) parseDotExpr(left ast.Expr) *ast.DotExpr {
	var expr ast.DotExpr
	expr.Left = left
	expr.Dot = p.expect(token.DOT)
	expr.Right = p.parseIdentExpr()
	return &expr
}

func (p *parser) parseIndexExpr(prefix ast.Expr) *ast.IndexExpr {
	var expr ast.IndexExpr
	expr.Prefix = prefix
	expr.Lbrack = p.expect(token.LBRACK)
	expr.Index = p.parseExpr()
	expr.Rbrack = p.expect(token.RBRACK)
	return &expr
}

func (p *parser) parseCallExpr(fn ast.Expr) *ast.CallExpr {
	fmt.Println(">>>>> CALL EXPR ", fn, p.tok)
	var expr ast.CallExpr
	expr.Fn = fn
	switch p.tok {
	case token.LPAREN:
		expr.Lparen = p.expect(token.LPAREN)
		if p.tok != token.RPAREN {
			expr.Args, expr.Commas = p.parseExprList()
		}
		expr.Rparen = p.expect(token.RPAREN)

	case token.LBRACE:
		expr.Args = []ast.Expr{p.parseMapExpr()}

	case token.STRING:
		expr.Args = []ast.Expr{p.parseAtomExpr()}

	case token.BANG:
		expr.Bang = p.expect(token.BANG)

	default:
		p.expect(token.LPAREN, token.LBRACE, token.STRING, token.BANG)
		panic("unreachable")
	}
	return &expr
}

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
