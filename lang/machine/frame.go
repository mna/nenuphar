package machine

import (
	"github.com/mna/nenuphar/lang/token"
	"github.com/mna/nenuphar/lang/types"
)

// Frame records a call to a Callable value (including module toplevel) or a
// built-in function or method.
type Frame struct {
	callable types.Value // current function (or toplevel) or callable
	pc       uint32      // program counter (non built-in only)
}

// Position returns the source position of the current point of execution in
// this frame.
func (fr *Frame) Position() token.Position {
	switch c := fr.callable.(type) {
	case *types.Function:
		line, col := c.Funcode.Pos(fr.pc).LineCol()
		return token.MakePosition(c.Funcode.Prog.Filename, line, col)
	case callableWithPosition:
		// If a built-in Callable defines a Position method, use it.
		return c.Position()
	case callableWithPos:
		line, col := c.Pos().LineCol()
		return token.MakePosition("", line, col)
	}
	return token.MakePosition("", 0, 0)
}

type callableWithPosition interface {
	Callable
	Position() token.Position
}

type callableWithPos interface {
	Callable
	Pos() token.Pos
}
