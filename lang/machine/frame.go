package machine

import (
	"github.com/mna/nenuphar/lang/ast"
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
func (fr *Frame) Position() ast.Position {
	switch c := fr.callable.(type) {
	case *types.Function:
		return c.funcode.Position(fr.pc)
	case callableWithPosition:
		// If a built-in Callable defines a Position method, use it.
		return c.Position()
	}
	return ast.MakePosition(&builtinFilename, 0, 0)
}

type callableWithPosition interface {
	types.Callable
	Position() ast.Position
}
