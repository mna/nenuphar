package filetest

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/kylelemons/godebug/diff"
)

var testUpdateAllTests = flag.Bool("test.update-all-tests", false, "If set, sets all test.update-*-tests.")

// SourceFiles returns the list of source files in dir corresponding to the
// specified extension.
func SourceFiles(t *testing.T, dir, ext string) []os.FileInfo {
	t.Helper()

	if ext != "" && ext[0] != '.' {
		ext = "." + ext
	}

	dents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	res := make([]os.FileInfo, 0, len(dents))
	for _, dent := range dents {
		if !dent.Type().IsRegular() {
			continue
		}
		if ext != "" && filepath.Ext(dent.Name()) != ext {
			continue
		}
		fi, err := dent.Info()
		if err != nil {
			t.Fatal(err)
		}
		res = append(res, fi)
	}
	return res
}

// DiffOutput validates that output is the same as the expected result in the
// corresponding golden file. If updateFlag is true, it updates the golden file
// with output instead.
func DiffOutput(t *testing.T, fi os.FileInfo, output, resultDir string, updateFlag *bool) {
	t.Helper()
	DiffCustom(t, fi, "output", ".want", output, resultDir, updateFlag)
}

// DiffErrors validates that the errors output is the same as the expected
// result in the corresponding golden file. If updateFlag is true, it updates
// the golden file with output instead.
func DiffErrors(t *testing.T, fi os.FileInfo, output, resultDir string, updateFlag *bool) {
	t.Helper()
	DiffCustom(t, fi, "errors", ".err", output, resultDir, updateFlag)
}

// DiffCustom is the general version of DiffOutput and DiffErrors, to check
// for any other kind of output file. Just provide a label to use in the
// error logs (e.g. "output", "errors", "comments") and the file extension
// to use for the golden file (including the leading dot) in addition to the
// same arguments as for DiffOutput.
func DiffCustom(t *testing.T, fi os.FileInfo, label, ext, output, resultDir string, updateFlag *bool) {
	t.Helper()

	wantFile := filepath.Join(resultDir, fi.Name()+ext)
	diffOrUpdate(t, label, wantFile, output, updateFlag)
}

func diffOrUpdate(t *testing.T, label, goldFile, output string, updateFlag *bool) {
	if *updateFlag || *testUpdateAllTests {
		if err := os.WriteFile(goldFile, []byte(output), 0600); err != nil {
			t.Fatal(err)
		}
		return
	}

	wantb, err := os.ReadFile(goldFile)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	want := string(wantb)
	if testing.Verbose() {
		t.Logf("got %s:\n%s\n", label, output)
	}
	if patch := diff.Diff(want, output); patch != "" {
		if testing.Verbose() {
			t.Logf("want %s:\n%s\n", label, want)
		}
		t.Errorf("diff %s:\n%s\n", label, patch)
	}
}
