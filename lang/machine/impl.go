package machine

import (
	"fmt"
	"math"
	"reflect"
	"strings"

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
	CallInternal(thread *Thread, args types.Tuple, kwargs []types.Tuple) (types.Value, error)
}

// Call calls the function or Callable value v with the specified positional
// and keyword arguments.
func Call(th *Thread, v types.Value, args types.Tuple, kwargs []types.Tuple) (types.Value, error) {
	var fn *types.Function
	var cb types.Callable

	switch v := v.(type) {
	case *types.Function:
		fn = v
	case types.Callable:
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
		// TODO(adonovan): opt: zero fr.Locals and reuse it if it is large enough.
		*fr = Frame{}

		th.callStack = th.callStack[:len(th.callStack)-1] // pop
	}()

	var result types.Value
	var err error

	if fn != nil {
		fr.callable = fn
		result, err = Run(th, fn)
	} else {
		fr.callable = cb
		result, err = cb.CallInternal(th, args, kwargs)
	}

	// Sanity check: nil is not a valid value.
	if result == nil && err == nil {
		err = fmt.Errorf("internal error: nil (not None) returned from %s", fn)
	}

	// Always return an EvalError with an accurate frame.
	if err != nil {
		if _, ok := err.(*EvalError); !ok {
			err = th.evalError(err)
		}
	}

	return result, err
}

// CompareDepth compares two values. The comparison operation must be one of
// EQL, NEQ, LT, LE, GT, or GE. CompareDepth returns an error if an ordered
// comparison was requested for a pair of values that do not support it.
//
// The depth parameter limits the maximum depth of recursion in cyclic data
// structures.
func CompareDepth(op token.Token, x, y types.Value, depth uint64) (bool, error) {
	if depth < 1 {
		// TODO: critical, non-catchable error
		return false, fmt.Errorf("comparison exceeded maximum recursion depth")
	}
	if sameType(x, y) {
		if xcomp, ok := x.(types.Ordered); ok {
			t, err := xcomp.Cmp(y, depth)
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
	// TODO(mna): any better way to do this?
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

// Binary applies a strict binary operator (not AND or OR) to its operands. For
// equality tests or ordered comparisons, use CompareDepth instead.
func Binary(op token.Token, x, y types.Value) (types.Value, error) {
	// TODO: revisit when language is more complete to determine if PLUS
	// applies to strings, etc.
	switch op {
	case token.PLUS:
		switch x := x.(type) {
		case types.String:
			if y, ok := y.(types.String); ok {
				return x + y, nil
			}
		case types.Int:
			switch y := y.(type) {
			case types.Int:
				return x + y, nil
			case types.Float:
				xf := types.Float(x)
				return xf + y, nil
			}
		case types.Float:
			switch y := y.(type) {
			case types.Float:
				return x + y, nil
			case types.Int:
				yf := types.Float(y)
				return x + yf, nil
			}
		case *types.Array:
			if y, ok := y.(*types.Array); ok {
				z := make([]types.Value, 0, x.Len()+y.Len())
				z = append(z, x.elems...)
				z = append(z, y.elems...)
				return types.NewList(z), nil
			}
		case types.Tuple:
			if y, ok := y.(types.Tuple); ok {
				z := make(types.Tuple, 0, len(x)+len(y))
				z = append(z, x...)
				z = append(z, y...)
				return z, nil
			}
		}

	case token.MINUS:
		switch x := x.(type) {
		case types.Int:
			switch y := y.(type) {
			case types.Int:
				return x - y, nil
			case types.Float:
				xf := types.Float(x)
				return xf - y, nil
			}
		case types.Float:
			switch y := y.(type) {
			case types.Float:
				return x - y, nil
			case types.Int:
				yf := types.Float(y)
				return x - yf, nil
			}
		case *types.Set: // difference
			if y, ok := y.(*types.Set); ok {
				iter := y.Iterate()
				defer iter.Done()
				return x.Difference(iter)
			}
		}

	case token.STAR:
		switch x := x.(type) {
		case types.Int:
			switch y := y.(type) {
			case types.Int:
				return x * y, nil
			case types.Float:
				xf := types.Float(x)
				return xf * y, nil
			case types.String:
				return stringRepeat(y, x)
			case types.Bytes:
				return bytesRepeat(y, x)
			case *types.Array:
				elems, err := tupleRepeat(Tuple(y.elems), x)
				if err != nil {
					return nil, err
				}
				return NewList(elems), nil
			case types.Tuple:
				return tupleRepeat(y, x)
			}
		case types.Float:
			switch y := y.(type) {
			case types.Float:
				return x * y, nil
			case types.Int:
				yf := types.Float(y)
				return x * yf, nil
			}
		case types.String:
			if y, ok := y.(types.Int); ok {
				return stringRepeat(x, y)
			}
		case types.Bytes:
			if y, ok := y.(types.Int); ok {
				return bytesRepeat(x, y)
			}
		case *types.Array:
			if y, ok := y.(types.Int); ok {
				elems, err := tupleRepeat(Tuple(x.elems), y)
				if err != nil {
					return nil, err
				}
				return NewList(elems), nil
			}
		case types.Tuple:
			if y, ok := y.(types.Int); ok {
				return tupleRepeat(x, y)
			}
		}

	case token.SLASH:
		switch x := x.(type) {
		case types.Int:
			xf := types.Float(x)
			switch y := y.(type) {
			case types.Int:
				yf := types.Float(y)
				if yf == 0.0 {
					return nil, fmt.Errorf("floating-point division by zero")
				}
				return xf / yf, nil
			case types.Float:
				if y == 0.0 {
					return nil, fmt.Errorf("floating-point division by zero")
				}
				return xf / y, nil
			}
		case types.Float:
			switch y := y.(type) {
			case types.Float:
				if y == 0.0 {
					return nil, fmt.Errorf("floating-point division by zero")
				}
				return x / y, nil
			case types.Int:
				yf := types.Float(y)
				if yf == 0.0 {
					return nil, fmt.Errorf("floating-point division by zero")
				}
				return x / yf, nil
			}
		}

	case token.SLASHSLASH:
		switch x := x.(type) {
		case types.Int:
			switch y := y.(type) {
			case types.Int:
				if y == 0 {
					return nil, fmt.Errorf("floored division by zero")
				}
				return x / y, nil
			case types.Float:
				xf := types.Float(x)
				if y == 0.0 {
					return nil, fmt.Errorf("floored division by zero")
				}
				return floor(xf / y), nil
			}
		case types.Float:
			switch y := y.(type) {
			case types.Float:
				if y == 0.0 {
					return nil, fmt.Errorf("floored division by zero")
				}
				return floor(x / y), nil
			case types.Int:
				yf := types.Float(y)
				if yf == 0.0 {
					return nil, fmt.Errorf("floored division by zero")
				}
				return floor(x / yf), nil
			}
		}

	case token.PERCENT:
		switch x := x.(type) {
		case types.Int:
			switch y := y.(type) {
			case types.Int:
				if y == 0 {
					return nil, fmt.Errorf("integer modulo by zero")
				}
				return x % y, nil
			case types.Float:
				xf := types.Float(x)
				if y == 0 {
					return nil, fmt.Errorf("floating-point modulo by zero")
				}
				return xf.Mod(y), nil
			}
		case types.Float:
			switch y := y.(type) {
			case types.Float:
				if y == 0.0 {
					return nil, fmt.Errorf("floating-point modulo by zero")
				}
				return x.Mod(y), nil
			case types.Int:
				if y == 0 {
					return nil, fmt.Errorf("floating-point modulo by zero")
				}
				yf := types.Float(y)
				return x.Mod(yf), nil
			}
		case types.String:
			return interpolate(string(x), y)
		}

	case token.NOT_IN:
		z, err := Binary(token.IN, x, y)
		if err != nil {
			return nil, err
		}
		return !z.Truth(), nil

	case token.IN:
		switch y := y.(type) {
		case *types.Array:
			for _, elem := range y.elems {
				if eq, err := Equal(elem, x); err != nil {
					return nil, err
				} else if eq {
					return types.True, nil
				}
			}
			return types.False, nil
		case types.Tuple:
			for _, elem := range y {
				if eq, err := Equal(elem, x); err != nil {
					return nil, err
				} else if eq {
					return types.True, nil
				}
			}
			return types.False, nil
		case types.Mapping: // e.g. dict
			// Ignore error from Get as we cannot distinguish true
			// errors (value cycle, type error) from "key not found".
			_, found, _ := y.Get(x)
			return types.Bool(found), nil
		case *types.Set:
			ok, err := y.Has(x)
			return types.Bool(ok), err
		case types.String:
			needle, ok := x.(types.String)
			if !ok {
				return nil, fmt.Errorf("'in <string>' requires string as left operand, not %s", x.Type())
			}
			return types.Bool(strings.Contains(string(y), string(needle))), nil
		case types.Bytes:
			switch needle := x.(type) {
			case types.Bytes:
				return types.Bool(strings.Contains(string(y), string(needle))), nil
			case types.Int:
				var b byte
				if err := AsInt(needle, &b); err != nil {
					return nil, fmt.Errorf("int in bytes: %s", err)
				}
				return types.Bool(strings.IndexByte(string(y), b) >= 0), nil
			default:
				return nil, fmt.Errorf("'in bytes' requires bytes or int as left operand, not %s", x.Type())
			}
		case rangeValue:
			i, err := NumberToInt(x)
			if err != nil {
				return nil, fmt.Errorf("'in <range>' requires integer as left operand, not %s", x.Type())
			}
			return types.Bool(y.contains(i)), nil
		}

	case token.PIPE:
		switch x := x.(type) {
		case types.Int:
			if y, ok := y.(types.Int); ok {
				return x | y, nil
			}

		case *types.Map: // union
			if y, ok := y.(*types.Map); ok {
				return x.Union(y), nil
			}

		case *types.Set: // union
			if y, ok := y.(*types.Set); ok {
				iter := Iterate(y)
				defer iter.Done()
				return x.Union(iter)
			}
		}

	case token.AMPERSAND:
		switch x := x.(type) {
		case types.Int:
			if y, ok := y.(Int); ok {
				return x & y, nil
			}
		case *types.Set: // intersection
			if y, ok := y.(*types.Set); ok {
				iter := y.Iterate()
				defer iter.Done()
				return x.Intersection(iter)
			}
		}

	case token.CIRCUMFLEX:
		switch x := x.(type) {
		case types.Int:
			if y, ok := y.(types.Int); ok {
				return x ^ y, nil
			}
		case *types.Set: // symmetric difference
			if y, ok := y.(*types.Set); ok {
				iter := y.Iterate()
				defer iter.Done()
				return x.SymmetricDifference(iter)
			}
		}

	case token.LTLT, token.GTGT:
		if x, ok := x.(types.Int); ok {
			y, err := AsInt32(y)
			if err != nil {
				return nil, err
			}
			if y < 0 {
				return nil, fmt.Errorf("negative shift count: %v", y)
			}
			if op == token.LTLT {
				if y >= 512 {
					return nil, fmt.Errorf("shift count too large: %v", y)
				}
				return x << uint(y), nil
			}
			return x >> uint(y), nil
		}

	default:
		// unknown operator
		goto unknown
	}

	// user-defined types
	// (nil, nil) => unhandled
	if x, ok := x.(types.HasBinary); ok {
		z, err := x.Binary(op, y, types.Left)
		if z != nil || err != nil {
			return z, err
		}
	}
	if y, ok := y.(types.HasBinary); ok {
		z, err := y.Binary(op, x, types.Right)
		if z != nil || err != nil {
			return z, err
		}
	}

	// unsupported operand types
unknown:
	return nil, fmt.Errorf("unknown binary op: %s %s %s", x.Type(), op, y.Type())
}

// Unary applies a unary operator (+, -, ~, not) to its operand.
func Unary(op token.Token, x types.Value) (types.Value, error) {
	// The NOT operator is not customizable.
	if op == token.NOT {
		return !x.Truth(), nil
	}

	// Int, Float, and user-defined types
	if x, ok := x.(types.HasUnary); ok {
		// (nil, nil) => unhandled
		y, err := x.Unary(op)
		if y != nil || err != nil {
			return y, err
		}
	}

	return nil, fmt.Errorf("unknown unary op: %s %s", op, x.Type())
}

// Iterate return a new iterator for the value if iterable, nil otherwise. If
// the result is non-nil, the caller must call Done when finished with it.
//
// Warning: Iterate(x) != nil does not imply Len(x) >= 0. Some iterables may
// have unknown length.
func Iterate(x types.Value) types.Iterator {
	if x, ok := x.(types.Iterable); ok {
		return x.Iterate()
	}
	return nil
}

// setIndex implements x[y] = z.
func setIndex(x, y, z types.Value) error {
	switch x := x.(type) {
	case types.HasSetKey:
		if err := x.SetKey(y, z); err != nil {
			return err
		}

	case types.HasSetIndex:
		n := x.Len()
		i, err := AsInt32(y)
		if err != nil {
			return err
		}
		origI := i
		if i < 0 {
			i += n
		}
		if i < 0 || i >= n {
			return outOfRange(origI, n, x)
		}
		return x.SetIndex(i, z)

	default:
		return fmt.Errorf("%s value does not support item assignment", x.Type())
	}
	return nil
}

// getIndex implements x[y].
func getIndex(x, y types.Value) (types.Value, error) {
	switch x := x.(type) {
	case types.Mapping: // dict
		z, found, err := x.Get(y)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("key %v not in %s", y, x.Type())
		}
		return z, nil

	case types.Indexable: // string, list, tuple
		n := x.Len()
		i, err := AsInt32(y)
		if err != nil {
			return nil, fmt.Errorf("%s index: %s", x.Type(), err)
		}
		origI := i
		if i < 0 {
			i += n
		}
		if i < 0 || i >= n {
			return nil, outOfRange(origI, n, x)
		}
		return x.Index(i), nil
	}
	return nil, fmt.Errorf("unhandled index operation %s[%s]", x.Type(), y.Type())
}

// getAttr implements x.dot.
func getAttr(x types.Value, name string) (types.Value, error) {
	hasAttr, ok := x.(types.HasAttrs)
	if !ok {
		return nil, fmt.Errorf("%s has no .%s field or method", x.Type(), name)
	}

	var errmsg string
	v, err := hasAttr.Attr(name)
	if err == nil {
		if v != nil {
			return v, nil // success
		}
		// (nil, nil) => generic error
		errmsg = fmt.Sprintf("%s has no .%s field or method", x.Type(), name)
	} else if nsa, ok := err.(types.NoSuchAttrError); ok {
		errmsg = string(nsa)
	} else {
		return nil, err // return error as is
	}

	// TODO: add spelling hint
	//if n := spell.Nearest(name, hasAttr.AttrNames()); n != "" {
	//	errmsg = fmt.Sprintf("%s (did you mean .%s?)", errmsg, n)
	//}

	return nil, fmt.Errorf("%s", errmsg)
}

// setField implements x.name = y.
func setField(x types.Value, name string, y types.Value) error {
	if x, ok := x.(types.HasSetField); ok {
		err := x.SetField(name, y)
		if _, ok := err.(types.NoSuchAttrError); ok {
			// TODO: No such field: check spelling.
			//if n := spell.Nearest(name, x.AttrNames()); n != "" {
			//	err = fmt.Errorf("%s (did you mean .%s?)", err, n)
			//}
		}
		return err
	}

	return fmt.Errorf("can't assign to .%s field of %s", name, x.Type())
}
