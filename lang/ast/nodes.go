package ast

import (
	"fmt"
	"os"
	"strings"

	"github.com/mna/nenuphar/lang/token"
)

type (
	// Chunk represents a Chunk production. It is exactly the same as Block
	// except that it keeps track of its name and the EOF, which is useful for
	// empty files to get a valid position.
	Chunk struct {
		// Name is the filename, which may be empty if the chunk is not a file.
		Name string

		// Comments is filled only if parsing comments was requested, and it lists
		// comments ordered by position in the chunk. Note that the comments are
		// not necessarily associated with the *Chunk, see each Comment.Node field
		// for the associated node.
		Comments []*Comment

		// Block is the block of statements contained in the chunk.
		Block *Block
		EOF   token.Pos // position of the EOF marker
	}

	// Comment represents a single comment, either short or long.
	Comment struct {
		// Node this comment is associated with, only set if parsing comments was
		// requested, and only after parsing (via post-processing).
		Node     Node
		Start    token.Pos // Position of the starting '-'
		Raw, Val string
	}

	// Block represents a block of statements.
	Block struct {
		// Both Start and End and saved because the block may start and end before
		// or after the statements due to comments.
		Start token.Pos
		End   token.Pos
		Stmts []Stmt
	}
)

func (n *Chunk) Format(f fmt.State, verb rune) {
	lbl := "chunk"
	if n.Name != "" {
		lbl += " " + strings.ReplaceAll(n.Name, string(os.PathSeparator), "/")
	}
	format(f, verb, n, lbl, nil)
}
func (n *Chunk) Span() (start, end token.Pos) {
	if n.Block != nil {
		return n.Block.Span()
	}
	return n.EOF, n.EOF
}
func (n *Chunk) Walk(v Visitor) {
	if n.Block != nil {
		Walk(v, n.Block)
	}
}

func (n *Comment) Format(f fmt.State, verb rune) { format(f, verb, n, "comment "+n.Val, nil) }
func (n *Comment) Span() (start, end token.Pos)  { return n.Start, n.Start + token.Pos(len(n.Raw)) }
func (n *Comment) Walk(_ Visitor)                {}

func (n *Block) Format(f fmt.State, verb rune) {
	format(f, verb, n, "block", map[string]int{"stmts": len(n.Stmts)})
}
func (n *Block) Span() (start, end token.Pos) { return n.Start, n.End }
func (n *Block) Walk(v Visitor) {
	for _, s := range n.Stmts {
		Walk(v, s)
	}
}
