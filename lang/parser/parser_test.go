package parser_test

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mna/mainer"
	"github.com/mna/nenuphar/internal/filetest"
	"github.com/mna/nenuphar/internal/maincmd"
	"github.com/mna/nenuphar/lang/parser"
	"github.com/mna/nenuphar/lang/token"
	"github.com/stretchr/testify/assert"
)

var testUpdateParserTests = flag.Bool("test.update-parser-tests", false, "If set, replace expected parser test results with actual results.")

func TestParser(t *testing.T) {
	ctx := context.Background()
	srcDir, resultDir := filepath.Join("testdata", "in"), filepath.Join("testdata", "out")

	modes := map[string]parser.Mode{
		"default":  parser.Mode(0),
		"comments": parser.Comments,
	}
	for name, mode := range modes {
		t.Run(name, func(t *testing.T) {
			for _, fi := range filetest.SourceFiles(t, srcDir, ".nen") {
				t.Run(fi.Name(), func(t *testing.T) {
					var buf, ebuf bytes.Buffer
					stdio := mainer.Stdio{
						Stdout: &buf,
						Stderr: &ebuf,
					}

					// error is ignored, we just want it to be printed to ebuf
					_ = maincmd.ParseFiles(ctx, stdio, mode, token.PosOffsets, "%#v", filepath.Join(srcDir, fi.Name()))
					ext := fmt.Sprintf(".want%d", mode)
					filetest.DiffCustom(t, fi, "output", ext, buf.String(), resultDir, testUpdateParserTests)
					filetest.DiffErrors(t, fi, ebuf.String(), resultDir, testUpdateParserTests)

					if t.Failed() && testing.Verbose() {
						b, err := os.ReadFile(filepath.Join(srcDir, fi.Name()))
						if assert.NoError(t, err) {
							t.Logf("source file:\n%s\n", string(b))
						}
					}
				})
			}
		})
	}
}
