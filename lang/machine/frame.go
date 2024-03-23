package machine

import (
	"github.com/mna/nenuphar/lang/token"
)

// Frame records a call to a Callable value (including module toplevel) or a
// built-in function or method.
type Frame struct {
	callable Callable // current function (or toplevel) or callable
	pc       uint32   // program counter (non built-in only)
}

// Position returns the source position of the current point of execution in
// this frame.
func (fr *Frame) Position() token.Position {
	switch c := fr.callable.(type) {
	case *Function:
		line, col := c.Funcode.Pos(fr.pc).LineCol()
		return token.MakePosition(c.Funcode.Prog.Filename, -1, line, col)
	case callableWithPosition:
		// If a built-in Callable defines a Position method, use it.
		return c.Position()
	case callableWithPos:
		line, col := c.Pos().LineCol()
		return token.MakePosition("", -1, line, col)
	}
	return token.MakePosition("", -1, 0, 0)
}

type callableWithPosition interface {
	Callable
	Position() token.Position
}

type callableWithPos interface {
	Callable
	Pos() token.Pos
}
