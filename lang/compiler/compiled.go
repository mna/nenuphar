package compiler

import (
	"go/token"
	"sync"
)

// A Funcode is the code of a compiled function. Funcodes are serialized by the
// encoder.function method, which must be updated whenever this declaration is
// changed.
type Funcode struct {
	Prog                  *Program
	Pos                   token.Pos // position of def or lambda token
	Name                  string    // name of this function
	Code                  []byte    // the byte code
	pclinetab             []uint16  // mapping from pc to linenum
	Locals                []Binding // locals, parameters first
	Cells                 []int     // indices of Locals that require cells
	Freevars              []Binding // for tracing
	Defers                []Defer   // defer blocks, nested ones must come after the more general ones
	Catches               []Defer   // catch blocks, nested ones must come after the more general ones
	MaxStack              int
	NumParams             int
	NumKwonlyParams       int
	HasVarargs, HasKwargs bool

	// -- transient state --

	lntOnce sync.Once
	lnt     []pclinecol // decoded line number table
}

type pclinecol struct {
	pc        uint32
	line, col int32
}
