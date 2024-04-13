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

func (p *parser) parseIfStmt(startPos token.Pos) *ast.IfGuardStmt {
	var stmt ast.IfGuardStmt

	if !startPos.IsValid() {
		// 'if' is not already consumed, do it now
		stmt.Type = token.IF
		stmt.Start = p.expect(token.IF)
	} else {
		// 'elseif' is already consumed in parent if/elseif, but record its
		// position and type here
		stmt.Type = token.ELSEIF
		stmt.Start = startPos
	}

	expect := []token.Token{token.ELSE}
	if stmt.Type == token.IF && tokenIn(p.tok, token.LET, token.CONST) { // DeclStmt not valid in elseif
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
		tok := p.tok
		stmt.Else = p.expect(expect...)
		if tok == token.ELSEIF {
			var elseIfBlock ast.Block
			elseIfStmt := p.parseIfStmt(stmt.Else)
			elseIfBlock.Start, elseIfBlock.End = elseIfStmt.Span()
			elseIfBlock.Stmts = []ast.Stmt{elseIfStmt}
			stmt.False = &elseIfBlock
		} else {
			stmt.False = p.parseBlock(token.END)
		}
	}
	if stmt.Type == token.IF {
		// this is the top-level 'if', it owns the 'end' token
		stmt.End = p.expect(token.END)
	}
	return &stmt
}

func (p *parser) parseForStmt() ast.Stmt {
	forPos := p.expect(token.FOR)
	switch p.tok {
	case token.DO:
		// for [ cond ] do, no condition (loop forever)
		return p.parseForCondStmt(forPos, nil)
	case token.SEMICOLON:
		// for [ init ]; [ cond ]; [ post ] do, no init
		return p.parseForThreePartStmt(forPos, nil)
	case token.LET, token.CONST:
		// for DeclStmt ; [ cond ]; [ post ] do, init is DeclStmt
		declStmt := p.parseDeclStmt()
		return p.parseForThreePartStmt(forPos, declStmt)
	default:
		// parse the left expressions and decide
		left, commas := p.parseDisambiguateSuffixedExprAssignStmt()

		// next token disambiguates the statement
		switch {
		case p.tok == token.DO:
			// for [ cond ] do, with condition
			return p.parseForCondStmt(forPos, p.expectSingleExpr(left, commas))

		case tokenIn(p.tok, token.SEMICOLON, token.EQ) || p.tok.IsAugBinop():
			// for [ init ]; [ cond ]; [ post ] do, with init
			firstStmt := p.parseExprOrAssignStmt(left, commas)
			return p.parseForThreePartStmt(forPos, firstStmt)

		case p.tok == token.IN:
			// for expr in exprlist
			return p.parseForInStmt(forPos, left, commas)

		default:
			p.expect(token.DO, token.SEMICOLON, token.IN, token.EQ)
			panic("unreachable")
		}
	}
}

func (p *parser) parseForInStmt(forPos token.Pos, left []ast.Expr, commas []token.Pos) *ast.ForInStmt {
	var stmt ast.ForInStmt
	stmt.For = forPos
	stmt.Left = left
	stmt.LeftCommas = commas

	// left must be assignable
	for _, e := range left {
		if !ast.IsAssignable(e) {
			start, _ := e.Span()
			p.errorExpected(start, "assignable expression")
		}
	}

	stmt.In = p.expect(token.IN)
	stmt.Right, stmt.RightCommas = p.parseExprList()
	stmt.Do = p.expect(token.DO)
	stmt.Body = p.parseBlock(token.END)
	stmt.End = p.expect(token.END)
	return &stmt
}

func (p *parser) parseForCondStmt(forPos token.Pos, cond ast.Expr) *ast.ForLoopStmt {
	var stmt ast.ForLoopStmt
	stmt.For = forPos
	stmt.Cond = cond
	stmt.Do = p.expect(token.DO)
	stmt.Body = p.parseBlock(token.END)
	stmt.End = p.expect(token.END)
	return &stmt
}

func (p *parser) parseForThreePartStmt(forPos token.Pos, init ast.Stmt) *ast.ForLoopStmt {
	var stmt ast.ForLoopStmt
	stmt.For = forPos
	stmt.Init = init
	stmt.InitSemi = p.expect(token.SEMICOLON)

	if p.tok != token.SEMICOLON {
		stmt.Cond = p.parseExpr()
	}
	stmt.CondSemi = p.expect(token.SEMICOLON)

	if p.tok != token.DO {
		stmt.Post = p.parseExprOrAssignStmt(nil, nil)
	}

	stmt.Do = p.expect(token.DO)
	stmt.Body = p.parseBlock(token.END)
	stmt.End = p.expect(token.END)
	return &stmt
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
	stmt.End = p.expect(token.END)
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

// parses expressions and commas until an EQ, IN or augmented assignment
// operator. Returns the list of expressions and commas parsed, and the
// current parser token indicates without ambiguity the nature of the current
// statement:
//   - EQ: AssignStmt
//   - IN: for..in header statement
//   - AugBinop: AugAssignStmt
//   - Otherwise: ExprStmt (possibly an invalid one)
func (p *parser) parseDisambiguateSuffixedExprAssignStmt() (left []ast.Expr, commas []token.Pos) {
	left = []ast.Expr{p.parseExpr()}
	for p.tok == token.COMMA {
		commas = append(commas, p.expect(token.COMMA))
		left = append(left, p.parseExpr())
	}
	return left, commas
}

func (p *parser) parseExprOrAssignStmt(left []ast.Expr, commas []token.Pos) ast.Stmt {
	if left == nil {
		left, commas = p.parseDisambiguateSuffixedExprAssignStmt()
	}
	if tokenIn(p.tok, token.EQ, token.IN) {
		// parsing as AssignStmt even if p.tok == IN, will take care of error
		return p.parseAssignStmt(left, commas)
	}
	if p.tok.IsAugBinop() {
		return p.parseAugAssignStmt(p.expectSingleExpr(left, commas))
	}

	expr := p.expectSingleExpr(left, commas)
	if !ast.IsValidStmt(expr) {
		start, end := expr.Span()
		p.errorExpected(start, "function call")
		return &ast.BadStmt{Start: start, End: end}
	}
	return &ast.ExprStmt{Expr: expr}
}

func (p *parser) parseAssignStmt(left []ast.Expr, commas []token.Pos) *ast.AssignStmt {
	var stmt ast.AssignStmt
	stmt.Left = left
	stmt.LeftCommas = commas

	// left must be assignable
	for _, e := range left {
		if !ast.IsAssignable(e) {
			start, _ := e.Span()
			p.errorExpected(start, "assignable expression")
		}
	}

	stmt.AssignTok = token.EQ
	stmt.AssignPos = p.expect(token.EQ)
	stmt.Right, stmt.RightCommas = p.parseExprList()
	return &stmt
}

func (p *parser) parseAugAssignStmt(firstExpr ast.Expr) *ast.AssignStmt {
	var stmt ast.AssignStmt

	// left must be assignable
	if !ast.IsAssignable(firstExpr) {
		start, _ := firstExpr.Span()
		p.errorExpected(start, "assignable expression")
	}
	stmt.Left = []ast.Expr{firstExpr}
	stmt.AssignTok = p.tok
	stmt.AssignPos = p.expect(augBinops...)
	stmt.Right = []ast.Expr{p.parseExpr()}
	return &stmt
}
