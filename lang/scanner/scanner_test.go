package scanner_test

import (
	"bytes"
	"context"
	"flag"
	"path/filepath"
	"testing"

	"github.com/mna/mainer"
	"github.com/mna/nenuphar/internal/filetest"
	"github.com/mna/nenuphar/internal/maincmd"
	"github.com/mna/nenuphar/lang/token"
)

var testUpdateScannerTests = flag.Bool("test.update-scanner-tests", false, "If set, replace expected scanner test results with actual results.")

func TestScan(t *testing.T) {
	ctx := context.Background()
	srcDir, resultDir := filepath.Join("testdata", "in"), filepath.Join("testdata", "out")

	for _, fi := range filetest.SourceFiles(t, srcDir, ".nen") {
		t.Run(fi.Name(), func(t *testing.T) {
			var buf, ebuf bytes.Buffer
			stdio := mainer.Stdio{
				Stdout: &buf,
				Stderr: &ebuf,
			}

			// error is ignored, we just want it to be printed to ebuf
			_ = maincmd.TokenizeFiles(ctx, stdio, token.PosOffsets, filepath.Join(srcDir, fi.Name()))
			filetest.DiffOutput(t, fi, buf.String(), resultDir, testUpdateScannerTests)
			filetest.DiffErrors(t, fi, ebuf.String(), resultDir, testUpdateScannerTests)
		})
	}
}
