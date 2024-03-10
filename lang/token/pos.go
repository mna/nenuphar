package token

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
