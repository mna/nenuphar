package maincmd

import (
	"context"
	"fmt"

	"github.com/mna/mainer"
	"github.com/mna/nenuphar/lang/ast"
	"github.com/mna/nenuphar/lang/parser"
	"github.com/mna/nenuphar/lang/scanner"
	"github.com/mna/nenuphar/lang/token"
)

func (c *Cmd) Parse(ctx context.Context, stdio mainer.Stdio, args []string) error {
	var parseMode parser.Mode
	if c.WithComments {
		parseMode |= parser.Comments
	}
	return ParseFiles(ctx, stdio, parseMode, token.PosLong, "", args...)
}

func ParseFiles(ctx context.Context, stdio mainer.Stdio, parseMode parser.Mode, posMode token.PosMode, nodeFmt string, files ...string) error {
	printer := ast.Printer{
		Output:  stdio.Stdout,
		Pos:     posMode,
		NodeFmt: nodeFmt,
	}
	fs, chunks, err := parser.ParseFiles(ctx, parseMode, files...)
	for _, ch := range chunks {
		start, _ := ch.Span()
		file := fs.File(start)
		if err := printer.Print(ch, file); err != nil {
			fmt.Fprintln(stdio.Stderr, err)
			return err
		}
	}
	if err != nil {
		scanner.PrintError(stdio.Stderr, err)
	}
	return err
}
