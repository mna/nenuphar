package resolver_test

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/mna/mainer"
	"github.com/mna/nenuphar/internal/filetest"
	"github.com/mna/nenuphar/internal/maincmd"
	"github.com/mna/nenuphar/lang/machine"
	"github.com/mna/nenuphar/lang/resolver"
	"github.com/mna/nenuphar/lang/token"
	"github.com/stretchr/testify/assert"
)

var testUpdateResolverTests = flag.Bool("test.update-resolver-tests", false, "If set, replace expected resolver test results with actual results.")

func TestResolver(t *testing.T) {
	ctx := context.Background()
	srcDir, resultDir := filepath.Join("testdata", "in"), filepath.Join("testdata", "out")

	// for this test and this test only, add a _G symbol to the universe.
	machine.Universe["_G"] = machine.Nil
	t.Cleanup(func() { delete(machine.Universe, "_G") })

	for _, fi := range filetest.SourceFiles(t, srcDir, ".nen") {
		t.Run(fi.Name(), func(t *testing.T) {
			var buf, ebuf bytes.Buffer
			stdio := mainer.Stdio{
				Stdout: &buf,
				Stderr: &ebuf,
			}

			// error is ignored, we just want it to be printed to ebuf
			_ = maincmd.ResolveFiles(ctx, stdio, 0, resolver.NameBlocks,
				token.PosOffsets, "%#v", filepath.Join(srcDir, fi.Name()))
			filetest.DiffOutput(t, fi, buf.String(), resultDir, testUpdateResolverTests)
			filetest.DiffErrors(t, fi, ebuf.String(), resultDir, testUpdateResolverTests)

			if t.Failed() && testing.Verbose() {
				b, err := os.ReadFile(filepath.Join(srcDir, fi.Name()))
				if assert.NoError(t, err) {
					t.Logf("source file:\n%s\n", string(b))
				}
			}
		})
	}
}
