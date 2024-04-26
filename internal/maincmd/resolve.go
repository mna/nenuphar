package maincmd

import (
	"context"
	"fmt"

	"github.com/mna/mainer"
	"github.com/mna/nenuphar/lang/ast"
	"github.com/mna/nenuphar/lang/parser"
	"github.com/mna/nenuphar/lang/resolver"
	"github.com/mna/nenuphar/lang/scanner"
	"github.com/mna/nenuphar/lang/token"
)

func (c *Cmd) Resolve(ctx context.Context, stdio mainer.Stdio, args []string) error {
	var parseMode parser.Mode
	if c.WithComments {
		parseMode |= parser.Comments
	}
	return ResolveFiles(ctx, stdio, parseMode, token.PosLong, "", args...)
}

func ResolveFiles(ctx context.Context, stdio mainer.Stdio, parseMode parser.Mode, posMode token.PosMode, nodeFmt string, files ...string) error {
	printer := ast.Printer{
		Output:  stdio.Stdout,
		Pos:     posMode,
		NodeFmt: nodeFmt,
	}
	fs, chunks, perr := parser.ParseFiles(ctx, parseMode, files...)
	if perr != nil {
		// cannot resolve AST if parsing has errors
		scanner.PrintError(stdio.Stderr, perr)
		return perr
	}

	rerr := resolver.ResolveFiles(ctx, fs, chunks, nil, isUniversal)
	for _, ch := range chunks {
		start, _ := ch.Span()
		file := fs.File(start)
		if err := printer.Print(ch, file); err != nil {
			fmt.Fprintln(stdio.Stderr, err)
			return err
		}
	}
	if rerr != nil {
		scanner.PrintError(stdio.Stderr, rerr)
	}
	return rerr
}
