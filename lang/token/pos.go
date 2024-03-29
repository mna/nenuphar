package token

import (
	"fmt"
	"go/token"
	"strconv"
)

type (
	File     = token.File
	FileSet  = token.FileSet
	Position = token.Position
	Pos      = token.Pos
)

const NoPos = token.NoPos

var NewFileSet = token.NewFileSet

// PosMode is the mode that controls printing of position information.
type PosMode int

// List of supported position modes.
const (
	PosNone    PosMode = iota
	PosOffsets         // [startoffset:endoffset]
	PosLong            // [filename:startline:col:endline:col]
	PosRaw             // [%d:%d] of the raw uninterpreted gotoken.Pos values
)

var posLabels = [...]string{
	PosNone:    "none",
	PosOffsets: "offsets",
	PosLong:    "long",
	PosRaw:     "raw",
}

// String returns the string representation of the PosMode.
func (m PosMode) String() string {
	if m >= 0 && int(m) < len(posLabels) {
		return posLabels[m]
	}
	return strconv.Itoa(int(m))
}

// PosSpan is an interface the defines the method for a value that can report a
// start-end position span, where the end position is one past the final
// position (e.g. [1-5) means a value starting at 1 up to and including 4). The
// length of the span is end - start.
//
// All ast.Node types implement this interface.
type PosSpan interface {
	Span() (start, end Pos)
}

// PosCovers returns true if test is fully inside the position span of ref.
// Both ref and test must be values that belong to the same file.
func PosInside(ref, test PosSpan) bool {
	ref0, ref1 := ref.Span()
	tst0, tst1 := test.Span()
	return tst0 >= ref0 && tst1 <= ref1
}

// PosAdjacent returns true if test is "adjacent to" ref following the
// comment-to-Node association rules documented in the parser package. Both ref
// and test must be values that belong to the file provided as argument,
// otherwise it panics.
//
// test is considered adjacent to ref if:
//  1. test.start or test.end is within the range of ref
//  2. test.end < ref.start but is on the same or the preceding line
//  3. test.start > ref.end but is on the same line
func PosAdjacent(ref, test PosSpan, file *File) bool {
	ref0, ref1 := ref.Span()
	tst0, tst1 := test.Span()

	if ref0 <= tst1 && ref1 >= tst0 {
		/* 1. */ return true
	}

	if tst1 < ref0 {
		tst1l, ref0l := file.Line(tst1), file.Line(ref0)
		if tst1l == ref0l || tst1l == ref0l-1 {
			/* 2. */ return true
		}
	}
	if tst0 > ref1 {
		tst0l, ref1l := file.Line(tst0), file.Line(ref1)
		if tst0l == ref1l {
			/* 3. */ return true
		}
	}
	return false
}

// FormatPos formats the position according to the mode. If filename is false and the
// mode is PosLong, the filename is not included (useful to print a range of from:to
// positions, where the filename is already part of the from label).
func FormatPos(mode PosMode, file *File, pos Pos, filename bool) string {
	var lbl string
	switch mode {
	case PosOffsets:
		lbl = "-"
		if pos.IsValid() {
			lbl = strconv.Itoa(file.Offset(pos))
		}

	case PosLong:
		if pos.IsValid() {
			lpos := file.Position(pos)
			if filename {
				lbl = fmt.Sprintf("%s:%d:%d", lpos.Filename, lpos.Line, lpos.Column)
			} else {
				lbl = fmt.Sprintf(":%d:%d", lpos.Line, lpos.Column)
			}
		} else if filename {
			lbl = fmt.Sprintf("%s:-:-", file.Name())
		} else {
			lbl = ":-:-"
		}

	case PosRaw:
		lbl = strconv.Itoa(int(pos))
	}
	return lbl
}
