package parser

import (
	"github.com/mna/nenuphar/lang/ast"
	"github.com/mna/nenuphar/lang/token"
)

func (p *parser) parseChunk() *ast.Chunk {
	var chunk ast.Chunk
	chunk.Block = p.parseBlock()
	chunk.EOF = p.expect(token.EOF)

	if p.parseComments {
		p.processComments(&chunk)
	}
	return &chunk
}

func (p *parser) parseBlock(endToks ...token.Token) *ast.Block {
	var block ast.Block
	var list []ast.Stmt

	block.Start = p.preCommentPos

	// EOF is always an end token
	endToks = append(endToks, token.EOF)

	var ending ast.Stmt
	var endingReported bool
	for !tokenIn(p.tok, endToks...) {
		if stmt := p.parseStmt(); stmt != nil {
			if ending != nil {
				if !endingReported {
					pos, _ := stmt.Span()
					p.errorExpected(pos, "end of block")
					endingReported = true
				}
			} else if stmt.BlockEnding() {
				ending = stmt
			}
			list = append(list, stmt)
		}
	}

	block.Stmts = list
	block.End = p.val.Pos
	return &block
}

// returns nil for a statement to ignore/skip (the ";" statement).
func (p *parser) parseStmt() (stmt ast.Stmt) {
	start := p.val.Pos

	defer func() {
		if err := recover(); err != nil {
			if err == errPanicMode {
				// synchronize to the next safe point and generate a BadStmt
				// for the interval.
				stmt = &ast.BadStmt{
					Start: start,
					End:   p.syncAfterError(),
				}
				return
			}
			panic(err)
		}
	}()

	switch p.tok {
	case token.SEMICOLON:
		// ignore empty statements
		p.advance()
		return nil

	case token.LET, token.CONST:
		return p.parseDeclStmt()

	case token.IF:
		return p.parseIfStmt()

	case token.FOR:
		return p.parseForStmt()

	case token.FUNCTION:
		return p.parseFuncStmt()

	case token.DEFER, token.CATCH, token.DO:
		return p.parseSimpleStmt()

	case token.RETURN, token.BREAK, token.CONTINUE, token.GOTO, token.THROW:
		return p.parseReturnLikeStmt(tokenIn(p.tok, token.RETURN, token.THROW))

	//case token.CLASS:
	//	return p.parseClassStmt()

	//case token.GUARD:
	//	return p.parseGuardStmt()

	case token.COLONCOLON:
		return p.parseLabelStmt()

		//default:
		//	// can be func call, assign stmt, augassign stmt, try or must unop.
		//	return p.parseExprStmt()
	}
	return nil
}

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

func (p *parser) parseIfStmt() *ast.IfGuardStmt {
	var stmt ast.IfGuardStmt
	stmt.Type = token.IF
	stmt.Start = p.expect(token.IF)

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
			elseIfStmt := p.parseIfStmt()
			elseIfBlock.Start, elseIfBlock.End = elseIfStmt.Span()
			elseIfBlock.Stmts = []ast.Stmt{elseIfStmt}
			stmt.False = &elseIfBlock
		} else {
			stmt.False = p.parseBlock(token.END)
		}
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

func (p *parser) parseLabelStmt() *ast.LabelStmt {
	var stmt ast.LabelStmt
	stmt.Lcolon = p.expect(token.COLONCOLON)
	stmt.Name = p.parseIdentExpr()
	stmt.Rcolon = p.expect(token.COLONCOLON)
	return &stmt
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

type syncMode int

const (
	syncAfter syncMode = iota
	syncAt
)

var (
	// "fn" and "class" could be valid starts of expressions
	stmtStartToks = []token.Token{
		token.SEMICOLON,
		token.IF,
		token.GUARD,
		token.DO,
		token.FOR,
		token.COLONCOLON,
		token.LET,
		token.CONST,
		token.RETURN,
		token.BREAK,
		token.CONTINUE,
		token.GOTO,
		token.DEFER,
		token.CATCH,
		token.THROW,
	}

	eobToks = []token.Token{
		token.ILLEGAL,
		token.EOF,
		token.END,
		token.ELSEIF,
		token.ELSE,
	}

	// Do and Function are not safe because they may appear as part
	// of a statement, so not a good sync position.
	// (e.g. for ... do, x = fn (...) end)
	// Same for class, let and const.
	syncToks = map[token.Token]syncMode{
		token.SEMICOLON:  syncAfter,
		token.END:        syncAfter,
		token.IF:         syncAt,
		token.GUARD:      syncAt,
		token.FOR:        syncAt,
		token.COLONCOLON: syncAt,
		token.RETURN:     syncAt,
		token.BREAK:      syncAt,
		token.CONTINUE:   syncAt,
		token.GOTO:       syncAt,
		token.DEFER:      syncAt,
		token.CATCH:      syncAt,
		token.THROW:      syncAt,
	}
)

func (p *parser) syncAfterError() token.Pos {
	for p.tok != token.EOF {
		if mode, ok := syncToks[p.tok]; ok {
			if mode == syncAfter {
				p.advance()
				if p.tok == token.EOF {
					// EOF is 1 past the end of the file
					return p.val.Pos - 1
				}
			}
			return p.val.Pos
		}
		p.advance()
	}
	return p.val.Pos - 1 // EOF is 1 past the end of the file
}

// returns true if t may indicate the start of a valid expression, false
// otherwise. Not completely reliable because ExprStmt, AssignStmt and
// AugAssignStmt start with an expression, and FuncStmt and ClassStmt start
// with a keyword used in FuncExpr and ClassExpr, but is a best effort check
// and is good enough when used while parsing a block-ending statement (e.g.
// return).
func maybeExprStart(t token.Token) bool {
	return !tokenIn(t, append(stmtStartToks, eobToks...)...)
}

func tokenIn(t token.Token, toks ...token.Token) bool {
	for _, tok := range toks {
		if t == tok {
			return true
		}
	}
	return false
}
