package parser

import (
	"github.com/mna/nenuphar/lang/ast"
	"github.com/mna/nenuphar/lang/token"
)

func (p *parser) parseDeclStmt() *ast.AssignStmt {
	var stmt ast.AssignStmt
	stmt.DeclType = p.tok
	stmt.DeclStart = p.expect(token.LET, token.CONST)

	var idents []ast.Expr
	var commas []token.Pos

	idents = append(idents, p.parseIdentExpr())
	for p.tok == token.COMMA {
		commas = append(commas, p.expect(token.COMMA))
		idents = append(idents, p.parseIdentExpr())
	}

	stmt.Left = idents
	stmt.LeftCommas = commas

	if p.tok == token.EQ {
		stmt.AssignTok = token.EQ
		stmt.AssignPos = p.expect(token.EQ)
		stmt.Right, stmt.RightCommas = p.parseExprList()
	}
	return &stmt
}

func (p *parser) parseIfStmt(topLevel bool) *ast.IfGuardStmt {
	var stmt ast.IfGuardStmt
	if topLevel {
		stmt.Type = token.IF
	} else {
		stmt.Type = token.ELSEIF
	}
	stmt.Start = p.expect(p.tok)

	expect := []token.Token{token.ELSE}
	if tokenIn(p.tok, token.LET, token.CONST) {
		stmt.Decl = p.parseDeclStmt()
	} else {
		stmt.Cond = p.parseExpr()
		expect = append(expect, token.ELSEIF)
	}
	stmt.Then = p.expect(token.THEN)
	// stop at ELSEIF even for an if-decl, it will be an error
	stmt.True = p.parseBlock(token.ELSEIF, token.ELSE, token.END)

	if p.tok != token.END {
		// there is an ELSE/ELSEIF, parse it
		stmt.Else = p.expect(expect...)
		if p.tok == token.ELSEIF {
			var elseIfBlock ast.Block
			elseIfStmt := p.parseIfStmt(false)
			elseIfBlock.Start, elseIfBlock.End = elseIfStmt.Span()
			elseIfBlock.Stmts = []ast.Stmt{elseIfStmt}
			stmt.False = &elseIfBlock
		} else {
			stmt.False = p.parseBlock(token.END)
		}
	}
	if topLevel {
		stmt.End = p.expect(token.END)
	}
	return &stmt
}

func (p *parser) parseForStmt() ast.Stmt {
	// TODO: for is quite complex, needs to determine what comes after "for"
	panic("unimplemented")
}

func (p *parser) parseFuncStmt() *ast.FuncStmt {
	var stmt ast.FuncStmt
	stmt.Fn = p.expect(token.FUNCTION)
	stmt.Name = p.parseIdentExpr()
	stmt.Sig = p.parseFuncSignature()
	stmt.Body = p.parseBlock(token.END)
	stmt.End = p.expect(token.END)
	return &stmt
}

func (p *parser) parseFuncSignature() *ast.FuncSignature {
	var sig ast.FuncSignature
	if p.tok == token.BANG {
		sig.Bang = p.expect(token.BANG)
		return &sig
	}
	sig.Lparen = p.expect(token.LPAREN)

	if !tokenIn(p.tok, token.RPAREN, token.EOF) {
		var params []*ast.IdentExpr
		var commas []token.Pos
		for p.tok == token.IDENT {
			params = append(params, p.parseIdentExpr())
			if p.tok == token.COMMA {
				commas = append(commas, p.expect(token.COMMA))
			} else {
				break
			}
		}
		// only way it could exit loop is if it hit RPAREN or DOTDOTDOT
		if p.tok == token.DOTDOTDOT {
			sig.DotDotDot = p.expect(token.DOTDOTDOT)
			params = append(params, p.parseIdentExpr())
			// can have a trailing comma
			if p.tok == token.COMMA {
				commas = append(commas, p.expect(token.COMMA))
			}
		}
		sig.Params = params
		sig.Commas = commas
	}
	sig.Rparen = p.expect(token.RPAREN)
	return &sig
}

func (p *parser) parseSimpleStmt() *ast.SimpleBlockStmt {
	var stmt ast.SimpleBlockStmt
	stmt.Type = p.tok
	stmt.Start = p.expect(p.tok)
	stmt.Body = p.parseBlock(token.END)
	return &stmt
}

func (p *parser) parseReturnLikeStmt(exprAllowed bool) *ast.ReturnLikeStmt {
	var stmt ast.ReturnLikeStmt
	stmt.Type = p.tok
	stmt.Start = p.expect(p.tok)
	if exprAllowed && maybeExprStart(p.tok) {
		stmt.Expr = p.parseExpr()
	} else if (p.tok == token.IDENT) || stmt.Type == token.GOTO {
		stmt.Expr = p.parseIdentExpr()
	}
	return &stmt
}

func (p *parser) parseClassStmt() *ast.ClassStmt {
	var stmt ast.ClassStmt
	stmt.Class = p.expect(token.CLASS)
	stmt.Name = p.parseIdentExpr()
	stmt.Inherits = p.parseClassInherits()
	stmt.Body = p.parseClassBody()
	return &stmt
}

func (p *parser) parseClassInherits() *ast.ClassInherit {
	var inherits ast.ClassInherit
	if p.tok == token.BANG {
		inherits.Bang = p.expect(token.BANG)
		return &inherits
	}

	inherits.Lparen = p.expect(token.LPAREN)
	if p.tok != token.RPAREN {
		// inherits expression is optional
		inherits.Expr = p.parseExpr()
	}
	inherits.Rparen = p.expect(token.RPAREN)
	return &inherits
}

func (p *parser) parseClassBody() *ast.ClassBody {
	var body ast.ClassBody

	var methods []*ast.FuncStmt
	var fields []*ast.AssignStmt
	for !tokenIn(p.tok, token.END, token.EOF) {
		if p.tok == token.FUNCTION {
			methods = append(methods, p.parseFuncStmt())
		} else if tokenIn(p.tok, token.LET, token.CONST) {
			fields = append(fields, p.parseDeclStmt())
		} else {
			// record the expected token error
			p.expect(token.FUNCTION, token.LET, token.CONST)
		}
	}

	body.Methods = methods
	body.Fields = fields
	body.End = p.expect(token.END)
	return &body
}

func (p *parser) parseGuardStmt() *ast.IfGuardStmt {
	var stmt ast.IfGuardStmt
	stmt.Type = token.GUARD
	stmt.Start = p.expect(token.GUARD)

	if tokenIn(p.tok, token.LET, token.CONST) {
		stmt.Decl = p.parseDeclStmt()
	} else {
		stmt.Cond = p.parseExpr()
	}
	stmt.Else = p.expect(token.ELSE)
	stmt.False = p.parseBlock(token.END)
	stmt.End = p.expect(token.END)
	return &stmt
}

func (p *parser) parseLabelStmt() *ast.LabelStmt {
	var stmt ast.LabelStmt
	stmt.Lcolon = p.expect(token.COLONCOLON)
	stmt.Name = p.parseIdentExpr()
	stmt.Rcolon = p.expect(token.COLONCOLON)
	return &stmt
}

func (p *parser) parseExprOrAssignStmt() ast.Stmt {
	expr := p.parseExpr()
	if tokenIn(p.tok, token.COMMA, token.EQ) {
		return p.parseAssignStmt(expr)
	}
	if p.tok.IsAugBinop() {
		return p.parseAugAssignStmt(expr)
	}
	if !ast.IsValidStmt(expr) {
		start, end := expr.Span()
		p.errorExpected(start, "function call")
		return &ast.BadStmt{Start: start, End: end}
	}
	return &ast.ExprStmt{Expr: expr}
}
