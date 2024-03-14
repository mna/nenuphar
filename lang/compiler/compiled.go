package compiler

import (
	"sync"

	"github.com/mna/nenuphar/lang/token"
)

// A Program is a source code file compiled in executable form. Programs are
// serialized by the Program.Encode method, which must be updated whenever this
// declaration is changed.
type Program struct {
	Filename  string
	Loads     []Binding     // name (really, string) and position of each load stmt
	Names     []string      // names of attributes and predeclared variables
	Constants []interface{} // = string | int64 | float64
	Functions []*Funcode
	Globals   []Binding // for error messages and tracing
	Toplevel  *Funcode  // module initialization function
}

// A Funcode is the code of a compiled function. Funcodes are serialized by the
// encoder.function method, which must be updated whenever this declaration is
// changed.
type Funcode struct {
	Prog       *Program
	Name       string    // name of this function
	Code       []byte    // the byte code
	Locals     []Binding // locals, parameters first
	Cells      []int     // indices of Locals that require cells
	Freevars   []Binding // for tracing
	Defers     []Defer   // defer blocks, nested ones must come after the more general ones
	Catches    []Defer   // catch blocks, nested ones must come after the more general ones
	MaxStack   int
	NumParams  int // includes the catchall vararg, if any
	HasVarargs bool

	pos       token.Pos // position of fn token
	pclinetab []uint16  // mapping from pc to linenum

	// -- transient state --

	lntOnce sync.Once
	lnt     []pclinecol // decoded line number table
}

type pclinecol struct {
	pc  uint32
	pos token.Pos
}

// Pos returns the source position for program counter pc.
func (fn *Funcode) Pos(pc uint32) token.Pos {
	fn.lntOnce.Do(fn.decodeLNT)

	// Binary search to find last LNT entry not greater than pc.
	// To avoid dynamic dispatch, this is a specialization of
	// sort.Search using this predicate:
	//   !(i < len(fn.lnt)-1 && fn.lnt[i+1].pc <= pc)
	n := len(fn.lnt)
	i, j := 0, n
	for i < j {
		h := int(uint(i+j) >> 1)
		if !(h >= n-1 || fn.lnt[h+1].pc > pc) {
			i = h + 1
		} else {
			j = h
		}
	}

	if i < n {
		return fn.lnt[i].pos
	}
	return token.MakePos(0, 0)
}

// decodeLNT decodes the line number table and populates fn.lnt.
// It is called at most once.
func (fn *Funcode) decodeLNT() {
	// Conceptually the table contains rows of the form
	// (pc uint32, line int32, col int32), sorted by pc.
	// We use a delta encoding, since the differences
	// between successive pc, line, and column values
	// are typically small and positive (though line and
	// especially column differences may be negative).
	// The delta encoding starts from
	// {pc: 0, line: fn.Pos.Line, col: fn.Pos.Col}.
	//
	// Each entry is packed into one or more 16-bit values:
	//    Δpc        uint4
	//    Δline      int5
	//    Δcol       int6
	//    incomplete uint1
	// The top 4 bits are the unsigned delta pc.
	// The next 5 bits are the signed line number delta.
	// The next 6 bits are the signed column number delta.
	// The bottom bit indicates that more rows follow because
	// one of the deltas was maxed out.
	// These field widths were chosen from a sample of real programs,
	// and allow >97% of rows to be encoded in a single uint16.

	fn.lnt = make([]pclinecol, 0, len(fn.pclinetab)) // a minor overapproximation
	line, col := fn.pos.LineCol()
	entry := pclinecol{
		pc:  0,
		pos: fn.pos,
	}
	for _, x := range fn.pclinetab {
		entry.pc += uint32(x) >> 12
		line += int((int16(x) << 4) >> (16 - 5)) // sign extend Δline
		col += int((int16(x) << 9) >> (16 - 6))  // sign extend Δcol
		if (x & 1) == 0 {
			entry.pos = token.MakePos(line, col)
			fn.lnt = append(fn.lnt, entry)
		}
	}
}

// A Binding is the name and position of a binding identifier.
type Binding struct {
	Name string
	Pos  token.Pos
}

// Defer is a defer or catch block that runs on exit of a block (defer) or if
// any error is raised by the instructions that it covers (catch). Emitted code
// for a defer block must ensure that there is a JMP over the defer block (to
// PC0), that StartPC is the pc after that JMP, and that the defer block does
// not fall through to the protected block - it must end with a DEFEREXIT or
// CATCHJMP beyond PC1 or a CALL to a function that always throws an error (a
// "rethrow"), etc.
type Defer struct {
	PC0, PC1 uint32 // start and end of protected instructions (inclusive), precondition: PC0 <= PC1
	StartPC  uint32 // start of the defer/catch instructions
}

func (c Defer) Covers(pc int64) bool {
	return int64(c.PC0) <= pc && pc <= int64(c.PC1)
}
