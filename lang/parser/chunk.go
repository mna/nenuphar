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

	p.enterBlock(&block)
	defer func() { p.exitBlock(&block) }()

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
		return p.parseIfStmt(token.Pos(0))

	case token.FOR:
		return p.parseForStmt()

	case token.FUNCTION:
		return p.parseFuncStmt()

	case token.DEFER, token.CATCH, token.DO:
		return p.parseSimpleStmt()

	case token.RETURN, token.BREAK, token.CONTINUE, token.GOTO, token.THROW:
		return p.parseReturnLikeStmt(tokenIn(p.tok, token.RETURN, token.THROW))

	case token.CLASS:
		return p.parseClassStmt()

	case token.GUARD:
		return p.parseGuardStmt()

	case token.COLONCOLON:
		return p.parseLabelStmt()

	default:
		// can be func call, assign stmt, augassign stmt, try or must unop.
		return p.parseExprOrAssignStmt(nil, nil)
	}
}

type syncMode int

const (
	syncAfter syncMode = iota
	syncAt
)

var (
	augBinops = []token.Token{
		token.PLUSEQ,
		token.MINUSEQ,
		token.STAREQ,
		token.SLASHEQ,
		token.SLASHSLASHEQ,
		token.PERCENTEQ,
		token.CIRCUMFLEXEQ,
		token.AMPERSANDEQ,
		token.PIPEEQ,
		token.TILDEEQ,
		token.LTLTEQ,
		token.GTGTEQ,
	}

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
