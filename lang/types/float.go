package types

import (
	"fmt"

	"github.com/mna/nenuphar-wip/syntax"
)

// Float is the type of a floating point number.
type Float float64

var (
	_ Value    = Float(0)
	_ Ordered  = Float(0)
	_ HasUnary = Float(0)
)

func (f Float) String() string {
	return fmt.Sprintf("%g", f)
}

func (f Float) Type() string { return "float" }
func (f Float) Freeze()      {} // immutable
func (f Float) Truth() Bool  { return f != 0.0 }

// Cmp implements comparison of two Float values.
func (f Float) Cmp(v Value, depth int) (int, error) {
	g := v.(Float)
	return floatCmp(f, g), nil
}

// floatCmp performs a three-valued comparison on floats, which are totally
// ordered with NaN > +Inf.
func floatCmp(x, y Float) int {
	if x > y {
		return +1
	} else if x < y {
		return -1
	} else if x == y {
		return 0
	}

	// At least one operand is NaN.
	if x == x {
		return -1 // y is NaN
	} else if y == y {
		return +1 // x is NaN
	}
	return 0 // both NaN
}

// Unary implements the operations +float and -float.
func (f Float) Unary(op syntax.Token) (Value, error) {
	switch op {
	case syntax.MINUS:
		return -f, nil
	case syntax.PLUS:
		return +f, nil
	}
	return nil, nil
}
