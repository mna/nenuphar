package ast

import (
	"fmt"
	"io"
	"strings"

	"github.com/mna/nenuphar/lang/token"
)

// Printer controls pretty-printing of the AST nodes.
type Printer struct {
	// Output is the io.Writer to print to.
	Output io.Writer

	// Pos indicates the position printing mode.
	Pos token.PosMode

	// NodeFmt is the format string to use to print the nodes. The verb must
	// be either `s` or `v`, a width can be set, and the `#` and `-` flags are
	// supported (`-` only when a width is set, to pad with spaces on the right
	// instead of the left). Defaults to `%v`.
	NodeFmt string

	// set during printing
	file  *token.File
	depth int
	err   error
}

// PrintComments pretty-prints the comments of the specified chunk and file.
func (p *Printer) PrintComments(chunk *Chunk, file *token.File) error {
	if file == nil || len(chunk.Comments) == 0 {
		return nil
	}

	if p.NodeFmt == "" {
		p.NodeFmt = "%v"
	}
	p.file = file
	p.depth = 0
	p.err = nil
	p.printComments(chunk)
	return p.err
}

func (p *Printer) printComments(chunk *Chunk) {
	var node Node
	for _, c := range chunk.Comments {
		if c.Node != node {
			node = c.Node
			p.printNode(node, 0)
		}
		p.printNode(c, 1)
	}
}

// Print pretty-prints the AST node n from the specified file.
func (p *Printer) Print(n Node, file *token.File) error {
	if file == nil {
		return nil
	}

	if p.NodeFmt == "" {
		p.NodeFmt = "%v"
	}
	p.file = file
	p.depth = 0
	p.err = nil
	Walk(p, n)
	return p.err
}

// Visit implements the Visitor interface for the Printer. It is not meant
// to be called directly, it is called by Print to walk the AST and print it.
func (p *Printer) Visit(n Node, dir VisitDirection) Visitor {
	if dir == VisitExit || p.err != nil {
		if requiresIndentDedent(n) {
			p.depth--
		}
		return nil
	}

	if requiresIndentDedent(n) {
		p.depth++
	}

	p.printNode(n, p.depth-1)
	return p
}

func (p *Printer) printNode(n Node, indent int) {
	if p.err != nil {
		return
	}

	format := "%s"
	args := []interface{}{strings.Repeat(". ", indent)}
	if p.Pos != token.PosNone {
		format += "[%s:%s] "
		args = append(args,
			token.FormatPos(p.Pos, p.file, n.StartPos(), true),
			token.FormatPos(p.Pos, p.file, n.EndPos(), false),
		)
	}
	format += p.NodeFmt + "\n"
	args = append(args, n)

	_, p.err = fmt.Fprintf(p.Output, format, args...)
}

func requiresIndentDedent(n Node) bool {
	//switch n := n.(type) {
	//case *IfStmt:
	//	// no indent on Else/ElseIf
	//	return n.Type == token.If
	//}
	return true
}
