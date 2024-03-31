// Package ast defines the types to represent the abstract syntax tree (AST)
// of the language. It is a quasi-lossless AST, in that it could recreate
// the source code precisely except for the following:
//   - semicolons are replaced by whitespace
//   - newlines are normalized to "\n"
//   - other whitespace is normalized to " " (e.g. tabs)
//
// Comments are not part of any node, instead they are parsed only if
// requested and stored separately in a map that associates them with the AST
// node they are most likely linked to. As such, they are not taken into
// consideration when reporting node positions, but they may affect the span
// of blocks (and indirectly, of the chunk).
package ast

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mna/nenuphar/lang/token"
)

// Node represents any node in the AST.
type Node interface {
	// Every Node implements the fmt.Formatter interface so they can print a
	// description of themselves. The only supported verbs are 'v' and 's'.
	// The '#' flag can be used to print count information about children
	// nodes. A width can be set to define the number of runes to print for
	// the node description - by default, that width is padded with spaces
	// on the left if the description is shorter, otherwise it is truncated
	// to that width. The '-' flag can be used to pad with spaces on the
	// right instead, and the '+' flag can be used to prevent padding
	// altogether - it only truncates if longer.
	fmt.Formatter

	// Span reports the start and end position of the node.
	Span() (start, end token.Pos)

	// Walk enters each node inside itself to implement the Visitor pattern.
	Walk(v Visitor)
}

// Expr represents an expression in the AST.
type Expr interface {
	Node
	expr()
}

// Stmt represents a statement in the AST.
type Stmt interface {
	Node

	// BlockEnding returns true if the statement should only appear as the last
	// statement in a block (return, break, continue, goto and throw).
	BlockEnding() bool
}

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
		// requested.
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

	// ====================
	// STATEMENTS
	// ====================

	// AssignStmt represents an assignment statement, e.g. x = y + z which may
	// also be a, b, c = 1, 2, 3 or an AugAssignStmt x += 2. It is also used to
	// represent DeclStmt.
	AssignStmt struct {
		DeclType    token.Token // zero if not a DeclStmt
		DeclStart   token.Pos   // zero if not a DeclStmt
		Left        []Expr      // only 1 for augassign
		LeftCommas  []token.Pos // always len(Left)-1, commas separating the Left
		AssignTok   token.Token // either EQ or between PLUSEQ and GTGTEQ
		AssignPos   token.Pos   // start pos of AssignTok
		Right       []Expr      // only 1 for augassign
		RightCommas []token.Pos // always len(Right)-1, commas separating the Right expressions
	}

	// ExprStmt represents an expression used as statement, which is only valid
	// for function calls (possibly wrapped in ParenExpr).
	ExprStmt struct {
		Expr Expr
	}

	// IfGuardStmt represents an if..then..elseif..else or a guard..else
	// statement.
	IfGuardStmt struct {
		Type  token.Token // if, elseif or guard
		Start token.Pos   // Position of Type token
		Cond  Expr        // nil if bind-type statement
		Decl  *AssignStmt // nil if cond-type statement
		Then  token.Pos   // zero for guard
		True  *Block      // nil for guard
		Else  token.Pos   // zero if no else/elseif
		False *Block      // nil if no else, single stmt in block if elseif (an IfGuardStmt)
	}

	SimpleBlockStmt struct {
		Type  token.Token // do, defer, catch
		Start token.Pos   // position of Type
		Body  *Block
	}
)

func (n *Chunk) Format(f fmt.State, verb rune) { format(f, verb, n, "chunk", nil) }
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

func (n *Comment) Format(f fmt.State, verb rune) { format(f, verb, n, n.Val, nil) }
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

func (n *AssignStmt) Format(f fmt.State, verb rune) {
	lbl := "assignment"
	if n.DeclType > 0 {
		lbl = n.DeclType.String() + " declaration"
	} else if n.AssignTok != token.EQ {
		lbl = "augmented assignment"
	}
	format(f, verb, n, lbl, map[string]int{"left": len(n.Left), "right": len(n.Right)})
}
func (n *AssignStmt) Span() (start, end token.Pos) {
	if n.DeclStart > 0 {
		start = n.DeclStart
	} else {
		start, _ = n.Left[0].Span()
	}
	_, end = n.Right[len(n.Right)-1].Span()
	return start, end
}
func (n *AssignStmt) Walk(v Visitor) {
	for _, e := range n.Left {
		Walk(v, e)
	}
	for _, e := range n.Right {
		Walk(v, e)
	}
}

func (n *ExprStmt) Format(f fmt.State, verb rune) { format(f, verb, n, "expr", nil) }
func (n *ExprStmt) Span() (start, end token.Pos)  { return n.Expr.Span() }
func (n *ExprStmt) Walk(v Visitor)                { Walk(v, n.Expr) }

func (n *IfGuardStmt) Format(f fmt.State, verb rune) { format(f, verb, n, n.Type.String(), nil) }
func (n *IfGuardStmt) Span() (start, end token.Pos) {
	if n.True != nil {
		_, end = n.True.Span()
	}
	if n.False != nil {
		_, end = n.False.Span()
	}
	return n.Start, end
}
func (n *IfGuardStmt) Walk(v Visitor) {
	if n.Cond != nil {
		Walk(v, n.Cond)
	}
	if n.Decl != nil {
		Walk(v, n.Decl)
	}
	if n.True != nil {
		Walk(v, n.True)
	}
	if n.False != nil {
		Walk(v, n.False)
	}
}

func (n *SimpleBlockStmt) Format(f fmt.State, verb rune) { format(f, verb, n, n.Type.String(), nil) }
func (n *SimpleBlockStmt) Span() (start, end token.Pos) {
	_, end = n.Body.Span()
	return n.Start, end
}
func (n *SimpleBlockStmt) Walk(v Visitor) {
	if n.Body != nil {
		Walk(v, n.Body)
	}
}

func format(f fmt.State, verb rune, n Node, label string, counts map[string]int) {
	if verb != 'v' && verb != 's' {
		fmt.Fprintf(f, "%%!%c(%T)", verb, n)
		return
	}

	// replace tabs and newlines with the corresponding unicode key
	label = strings.ReplaceAll(label, "\r\n", "⏎")
	label = strings.ReplaceAll(label, "\n", "⏎")
	label = strings.ReplaceAll(label, "\t", "⭾")
	label = strings.ReplaceAll(label, "\v", "⭿")

	if w, ok := f.Width(); ok {
		minus, plus := f.Flag('-'), f.Flag('+')
		runes := []rune(label)
		if len(runes) >= w {
			runes = runes[:w]
		} else if minus {
			runes = append(runes, []rune(strings.Repeat(" ", w-len(runes)))...)
		} else if !plus {
			runes = append([]rune(strings.Repeat(" ", w-len(runes))), runes...)
		}
		label = string(runes)
	}

	fmt.Fprint(f, label)
	if f.Flag('#') && len(counts) > 0 {
		keys := make([]string, 0, len(counts))
		for k := range counts {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		fmt.Fprint(f, " {")
		for i, k := range keys {
			if i > 0 {
				fmt.Fprint(f, ", ")
			}
			fmt.Fprintf(f, "%s=%d", k, counts[k])
		}
		fmt.Fprint(f, "}")
	}
}
