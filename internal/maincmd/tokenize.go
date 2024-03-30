package maincmd

import (
	"context"
	"fmt"

	"github.com/mna/mainer"
	"github.com/mna/nenuphar/lang/scanner"
)

func (c *Cmd) Tokenize(ctx context.Context, stdio mainer.Stdio, args []string) error {
	// TODO: use FormatPos, provide position mode as flag

	fs, toksByFile, err := scanner.ScanFiles(ctx, args...)
	for _, toks := range toksByFile {
		for _, tok := range toks {
			fmt.Fprintf(stdio.Stdout, "%s: %s", fs.Position(tok.Value.Pos), tok.Token)
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
