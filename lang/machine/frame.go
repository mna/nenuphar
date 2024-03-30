package machine

import (
	"github.com/mna/nenuphar/lang/compiler"
)

// Frame records a call to a Callable value (including module toplevel) or a
// built-in function or method.
type Frame struct {
	callable Callable // current function (or toplevel) or callable
	pc       uint32   // program counter (non built-in only)
}

// Position returns the filename and source position of the current point of
// execution in this frame.
func (fr *Frame) Position() (string, compiler.Position) {
	switch c := fr.callable.(type) {
	case *Function:
		return c.Funcode.Prog.Filename, c.Funcode.Pos(fr.pc)
	case callableWithFilenameAndPosition:
		return c.Filename(), c.Position()
	case callableWithPosition:
		return "", c.Position()
	case callableWithFilename:
		return c.Filename(), compiler.Position{}
	}
	return "", compiler.Position{}
}

type callableWithPosition interface {
	Callable
	Position() compiler.Position
}

type callableWithFilename interface {
	Callable
	Filename() string
}

type callableWithFilenameAndPosition interface {
	callableWithFilename
	callableWithPosition
}
