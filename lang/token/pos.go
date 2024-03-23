package token

import (
	"fmt"
)

const (
	lineBits = 18
	colBits  = 32 - lineBits

	// MaxLines is the maximum 1-based line number value that can be encoded in
	// Pos.
	MaxLines = (1 << lineBits) - 1
	// MaxCols is the maximum 1-based column number value that can be encoded in
	// Pos.
	MaxCols = (1 << colBits) - 1

	lineMask = MaxLines
	colMask  = MaxCols
)

// Pos is an efficient encoding of a 1-based line and column position in a
// 32-bit unsigned integer. A value of 0 for either line or column should be
// interpreted as "unknown".
type Pos uint32

// MakePos creates a Pos value encoding the provided line and col. It is the
// caller's responsibility to ensure the values are > 0 and <= the maximum
// allowed.
func MakePos(line, col int) Pos {
	return Pos(col<<lineBits | line)
}

// LineCol returns the line and column values encoded in Pos.
func (p Pos) LineCol() (int, int) {
	l := p & lineMask
	c := (p >> lineBits) & colMask
	return int(l), int(c)
}

// Unknown returns true if either line or column value is unknown.
func (p Pos) Unknown() bool {
	l, c := p.LineCol()
	return l == 0 || c == 0
}

func (p Pos) ToPosition(file string, offset int) Position {
	return Position{Filename: file, Pos: p, Offset: offset}
}

// Position fully describes a location in a file, including its filename.
type Position struct {
	Filename string
	Pos      Pos
	Offset   int // offset in bytes
}

// MakePosition returns a position with the specified components.
func MakePosition(file string, offset, line, col int) Position {
	return Position{Filename: file, Pos: MakePos(line, col), Offset: offset}
}

func (p Position) String() string {
	file := p.Filename
	if file == "" {
		file = "<unknown file>"
	}
	l, c := p.Pos.LineCol()
	if l > 0 {
		if c > 0 {
			return fmt.Sprintf("%s:%d:%d", file, l, c)
		}
		return fmt.Sprintf("%s:%d", file, l)
	}
	return file
}
