// Package ast defines the types to represent the abstract syntax tree (AST)
// of the language. It is a quasi-lossless AST, in that it could recreate
// the source code precisely except for the following:
//   - semicolons are replaced by whitespace where unnecessary (outside for
//     loop headers)
//   - newlines are normalized to "\n"
//   - other whitespace is normalized to " " (e.g. tabs)
//
// Comments are not part of any node, instead they are parsed only if
// requested and stored separately in a map that associates them with the AST
// node they are most likely linked to. As such, they are not taken into
// consideration when reporting node positions, but they may affect the span
// of blocks (and indirectly, of the chunk).
//
// Note that this package is tested via the parser package's tests.
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
	expr() // TODO: might not be needed as useful methods will likely be added (e.g. Assignable?)
}

// Stmt represents a statement in the AST.
type Stmt interface {
	Node

	// BlockEnding returns true if the statement should only appear as the last
	// statement in a block (return, break, continue, goto and throw).
	BlockEnding() bool
}

type (
	// ====================
	// HELPERS (not nodes)
	// ====================

	FuncSignature struct {
		Lparen    token.Pos // zero if '!'
		Params    []*IdentExpr
		Commas    []token.Pos // at least len(Params)-1, can be len(Params)
		DotDotDot token.Pos   // zero if no '...', otherwise last param is vararg
		Rparen    token.Pos   // zero if '!'
		Bang      token.Pos   // position of the '!' token if no param and present
	}

	ClassInherit struct {
		Lparen token.Pos // zero if '!'
		Expr   Expr      // may be nil, inherits expression
		Rparen token.Pos // zero if '!'
		Bang   token.Pos // position of the '!' token if no expr and present
	}

	ClassBody struct {
		Methods []*FuncStmt
		Fields  []*AssignStmt // must all be DeclStmt
		End     token.Pos
	}

	KeyVal struct {
		Lbrack token.Pos // zero if not in brackets
		Key    Expr      // *IdentExpr, *LiteralExpr or Expr inside brackets
		Rbrack token.Pos // zero if not in brackets
		Colon  token.Pos
		Value  Expr
	}
)

var formatReplacer = strings.NewReplacer(
	"\r\n", "⏎",
	"\n", "⏎",
	"\t", "⭾",
	"\v", "⭿",
)

func format(f fmt.State, verb rune, n Node, label string, counts map[string]int) {
	if verb != 'v' && verb != 's' {
		fmt.Fprintf(f, "%%!%c(%T)", verb, n)
		return
	}

	// replace tabs and newlines with the corresponding unicode key
	label = formatReplacer.Replace(label)

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
