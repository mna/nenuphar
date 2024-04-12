package ast

import (
	"errors"
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
}

// Print pretty-prints the AST node n from the specified file. If n is an
// *ast.Chunk and it has comments, they will be printed along with their
// associated node. The file argument is only required for printing positions,
// if p.Pos == token.PosNone, it does not have to be provided.
func (p *Printer) Print(n Node, file *token.File) error {
	if file == nil && p.Pos != token.PosNone {
		return errors.New("file must be provided to print positions")
	}

	pp := &printer{
		w:       p.Output,
		pos:     p.Pos,
		nodeFmt: p.NodeFmt,
		file:    file,
	}
	if p.NodeFmt == "" {
		pp.nodeFmt = "%v"
	}

	if ch, ok := n.(*Chunk); ok && len(ch.Comments) > 0 {
		// index comments by their associated node for printing
		m := make(map[Node][]*Comment, len(ch.Comments))
		for _, c := range ch.Comments {
			m[c.Node] = append(m[c.Node], c)
		}
		pp.comments = m
	}

	Walk(pp, n)
	return pp.err
}

type printer struct {
	w        io.Writer
	pos      token.PosMode
	nodeFmt  string
	comments map[Node][]*Comment
	file     *token.File
	depth    int
	err      error
}

func (p *printer) Visit(n Node, dir VisitDirection) Visitor {
	if dir == VisitExit || p.err != nil {
		p.depth--
		return nil
	}

	p.depth++
	p.printNode(n, p.depth-1)
	p.printNodeComments(n, p.depth)
	return p
}

func (p *printer) printNode(n Node, indent int) {
	if p.err != nil {
		return
	}

	format := "%s"
	args := []interface{}{strings.Repeat(". ", indent)}
	if p.pos != token.PosNone {
		format += "[%s:%s] "
		start, end := n.Span()
		args = append(args,
			token.FormatPos(p.pos, p.file, start, true),
			token.FormatPos(p.pos, p.file, end, false),
		)
	}
	format += p.nodeFmt + "\n"
	args = append(args, n)

	_, p.err = fmt.Fprintf(p.w, format, args...)
}

func (p *printer) printNodeComments(n Node, indent int) {
	comments := p.comments[n]
	for _, c := range comments {
		p.printNode(c, indent)
	}
}

func requiresIndentDedent(n Node) bool {
	//switch n := n.(type) {
	//case *IfStmt:
	//	// no indent on Else/ElseIf
	//	return n.Type == token.If
	//}
	return true
}
