package parser

import (
	"github.com/mna/nenuphar/lang/ast"
	"github.com/mna/nenuphar/lang/token"
)

func (p *parser) processComments(chunk *ast.Chunk) {
	var av adjacentVisitor

	for _, c := range p.pendingComments {
		if c.Node == nil {
			c.Node = chunk // should never happen, always a Block in a Chunk
		}

		av.init(c, p.file)
		ast.Walk(&av, c.Node)
		if av.lastAdjacent != nil {
			c.Node = av.lastAdjacent
		}
	}
	chunk.Comments = p.pendingComments
}

type adjacentVisitor struct {
	comment      *ast.Comment
	lastAdjacent ast.Node
	file         *token.File
}

func (v *adjacentVisitor) init(c *ast.Comment, file *token.File) {
	v.comment = c
	v.file = file
	v.lastAdjacent = nil
}

func (v *adjacentVisitor) Visit(n ast.Node, dir ast.VisitDirection) ast.Visitor {
	if dir == ast.VisitExit {
		return nil
	}

	// only look for adjacent nodes that are statements (i.e. do not
	// associate a comment with an identifier or an integer expression)
	if _, ok := n.(ast.Stmt); !ok {
		return v
	}

	if token.PosAdjacent(n, v.comment, v.file) {
		v.lastAdjacent = n
		return v
	}
	return nil
}
