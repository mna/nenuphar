package machine

import (
	"fmt"
)

// Float is the type of a floating point number.
type Float float64

var (
	_ Value   = Float(0)
	_ Ordered = Float(0)
)

func (f Float) String() string {
	return fmt.Sprintf("%g", f)
}

func (f Float) Type() string { return "float" }

// Cmp implements comparison of two Float values.
func (f Float) Cmp(v Value) (int, error) {
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
