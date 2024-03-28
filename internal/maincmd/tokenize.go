package maincmd

import (
	"context"
	"fmt"

	"github.com/mna/mainer"
	"github.com/mna/nenuphar/lang/scanner"
)

func (c *Cmd) Tokenize(ctx context.Context, stdio mainer.Stdio, args []string) error {
	toksByFile, err := scanner.ScanFiles(ctx, args...)
	for i, toks := range toksByFile {
		for _, tok := range toks {
			fmt.Fprintf(stdio.Stdout, "%s: %s", tok.Value.Pos.ToPosition(args[i], 0), tok.Token)
			if lit := tok.Token.Literal(tok.Value); lit != "" {
				fmt.Fprintf(stdio.Stdout, " %s", lit)
			}
			fmt.Fprintln(stdio.Stdout)
		}
	}
	if err != nil {
		// errors already have the position information with the filename as
		// part of the message
		errs := err.(interface{ Unwrap() []error }).Unwrap()
		for _, err := range errs {
			fmt.Fprintln(stdio.Stderr, err)
		}
	}
	return err
}
