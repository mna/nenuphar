package maincmd

import (
	"context"

	"github.com/mna/mainer"
	"github.com/mna/nenuphar/lang/scanner"
)

func (c *Cmd) Tokenize(ctx context.Context, stdio mainer.Stdio, args []string) error {
	toksByFile, err := scanner.ScanFiles(ctx, args...)
	for i, toks := range toksByFile {
		// TODO: write tokens to stdout
		_, _ = i, toks
	}
	if err != nil {
		// TODO: write errors to stderr
	}
	return nil
}
