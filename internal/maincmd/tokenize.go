package maincmd

import (
	"context"
	"fmt"

	"github.com/mna/mainer"
	"github.com/mna/nenuphar/lang/scanner"
	"github.com/mna/nenuphar/lang/token"
)

func (c *Cmd) Tokenize(ctx context.Context, stdio mainer.Stdio, args []string) error {
	return TokenizeFiles(ctx, stdio, token.PosLong, args...)
}

func TokenizeFiles(ctx context.Context, stdio mainer.Stdio, posMode token.PosMode, files ...string) error {
	fs, toksByFile, err := scanner.ScanFiles(ctx, files...)
	for _, toks := range toksByFile {
		for _, tok := range toks {
			fmt.Fprintf(stdio.Stdout, "%s: %s", token.FormatPos(posMode, fs.File(tok.Value.Pos), tok.Value.Pos, true), tok.Token)
			if lit := tok.Token.Literal(tok.Value); lit != "" {
				fmt.Fprintf(stdio.Stdout, " %s", lit)
			}
			fmt.Fprintln(stdio.Stdout)
		}
	}
	if err != nil {
		scanner.PrintError(stdio.Stderr, err)
	}
	return err
}
