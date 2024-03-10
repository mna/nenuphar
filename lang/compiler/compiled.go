package compiler

import (
	"go/token"
	"sync"
)

// A Program is a source code file compiled in executable form. Programs are
// serialized by the Program.Encode method, which must be updated whenever this
// declaration is changed.
type Program struct {
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
	Pos        token.Pos // position of def or lambda token
	Name       string    // name of this function
	Code       []byte    // the byte code
	pclinetab  []uint16  // mapping from pc to linenum
	Locals     []Binding // locals, parameters first
	Cells      []int     // indices of Locals that require cells
	Freevars   []Binding // for tracing
	Defers     []Defer   // defer blocks, nested ones must come after the more general ones
	Catches    []Defer   // catch blocks, nested ones must come after the more general ones
	MaxStack   int
	NumParams  int
	HasVarargs bool

	// -- transient state --

	lntOnce sync.Once
	lnt     []pclinecol // decoded line number table
}

type pclinecol struct {
	pc        uint32
	line, col int32
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
