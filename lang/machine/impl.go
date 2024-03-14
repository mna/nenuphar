package machine

import (
	"fmt"
	"math"
	"reflect"

	"github.com/mna/nenuphar/lang/token"
	"github.com/mna/nenuphar/lang/types"
)

// Some machine opcodes are more complex and/or need to be exposed via a
// low-level interface to be available for higher-level APIs. Those functions
// belong in this file.

// A Callable value f may be the operand of a function call, f(x). Clients
// should use the Call function, never the CallInternal method.
type Callable interface {
	types.Value
	Name() string
	CallInternal(thread *Thread, args types.Tuple) (types.Value, error)
}

// Call calls the function or Callable value v with the specified arguments.
func Call(th *Thread, v types.Value, args types.Tuple) (types.Value, error) {
	var fn *types.Function
	var cb Callable

	switch v := v.(type) {
	case *types.Function:
		fn = v
	case Callable:
		cb = v
	default:
		return nil, fmt.Errorf("invalid call of non-callable (%s)", fn.Type())
	}

	// Allocate and push a new frame. As an optimization, use slack portion of
	// thread.callStack slice as a freelist of empty frames.
	var fr *Frame
	if n := len(th.callStack); n < cap(th.callStack) {
		fr = th.callStack[n : n+1][0]
	}
	if fr == nil {
		fr = new(Frame)
	}

	if th.callStack == nil {
		th.init()
	}

	th.callStack = append(th.callStack, fr) // push

	// Use defer to ensure that panics from built-ins pass through the
	// interpreter without leaving it in a bad state.
	defer func() {
		// clear out any references
		*fr = Frame{}
		th.callStack = th.callStack[:len(th.callStack)-1] // pop
	}()

	var (
		result types.Value
		err    error
	)

	if fn != nil {
		fr.callable = fn
		result, err = Run(th, fn, args)
	} else {
		fr.callable = cb
		result, err = cb.CallInternal(th, args)
	}

	// Sanity check: nil is not a valid value.
	if result == nil && err == nil {
		err = fmt.Errorf("internal error: nil (not Nil) returned from %s", fn)
	}
	return result, err
}

// CompareDepth compares two values. The comparison operation must be one of
// EQL, NEQ, LT, LE, GT, or GE. CompareDepth returns an error if an ordered
// comparison was requested for a pair of values that do not support it.
//
// The depth parameter limits the maximum depth of recursion in cyclic data
// structures.
func CompareDepth(op token.Token, x, y types.Value, depth uint64) (bool, error) { // TODO: figure out type of depth vs Cmp
	if depth < 1 {
		// TODO: critical, non-catchable error
		return false, fmt.Errorf("comparison exceeded maximum recursion depth")
	}
	if sameType(x, y) {
		if xcomp, ok := x.(types.Ordered); ok {
			t, err := xcomp.Cmp(y, int(depth))
			if err != nil {
				return false, err
			}
			return threeway(op, t), nil
		}

		// use identity comparison
		switch op {
		case token.EQL:
			return x == y, nil
		case token.NEQ:
			return x != y, nil
		}
		return false, fmt.Errorf("%s %s %s not implemented", x.Type(), op, y.Type())
	}

	// different types

	// int/float ordered comparisons
	switch x := x.(type) {
	case types.Int:
		if y, ok := y.(types.Float); ok {
			var cmp int
			if y != y {
				cmp = -1 // y is NaN
			} else if !math.IsInf(float64(y), 0) {
				// TODO(mna): a bit naive for now
				if xf := float64(x); xf == float64(y) {
					cmp = 0
				} else if xf < float64(y) {
					cmp = -1
				} else {
					cmp = +1
				}
			} else if y > 0 {
				cmp = -1 // y is +Inf
			} else {
				cmp = +1 // y is -Inf
			}
			return threeway(op, cmp), nil
		}
	case types.Float:
		if y, ok := y.(types.Int); ok {
			var cmp int
			if x != x {
				cmp = +1 // x is NaN
			} else if !math.IsInf(float64(x), 0) {
				// TODO(mna): a bit naive for now
				if yf := float64(y); float64(x) == yf {
					cmp = 0
				} else if yf < float64(x) {
					cmp = +1
				} else {
					cmp = -1
				}
			} else if x > 0 {
				cmp = +1 // x is +Inf
			} else {
				cmp = -1 // x is -Inf
			}
			return threeway(op, cmp), nil
		}
	}

	// All other values of different types compare unequal.
	switch op {
	case token.EQL:
		return false, nil
	case token.NEQ:
		return true, nil
	}
	return false, fmt.Errorf("%s %s %s not implemented", x.Type(), op, y.Type())
}

func sameType(x, y types.Value) bool {
	// TODO(mna): any better way to do this? Doesn't seem overly costly though,
	// mostly pointer casting.
	return reflect.TypeOf(x) == reflect.TypeOf(y)
}

// threeway interprets a three-way comparison value cmp (-1, 0, +1)
// as a boolean comparison (e.g. x < y).
func threeway(op token.Token, cmp int) bool {
	switch op {
	case token.EQL:
		return cmp == 0
	case token.NEQ:
		return cmp != 0
	case token.LE:
		return cmp <= 0
	case token.LT:
		return cmp < 0
	case token.GE:
		return cmp >= 0
	case token.GT:
		return cmp > 0
	}
	panic(op)
}

// Truth returns the truthy value of v, which is True for every value except
// False and Nil.
func Truth(v types.Value) types.Bool {
	switch v := v.(type) {
	case types.Bool:
		return v
	case types.NilType:
		return types.False
	default:
		return types.True
	}
}
