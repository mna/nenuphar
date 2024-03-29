package ast

import (
	"fmt"
	"go/token"
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
