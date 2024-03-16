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

// setIndex implements x[y] = z.
func setIndex(x, y, z types.Value) error {
	switch x := x.(type) {
	case types.HasSetKey:
		if err := x.SetKey(y, z); err != nil {
			return err
		}

	case types.HasSetIndex:
		n := x.Len()
		i, err := AsInt(y)
		if err != nil {
			return err
		}
		origI := i
		if i < 0 {
			i += n
		}
		if i < 0 || i >= n {
			return fmt.Errorf("%s index %d out of range [%d:%d]", x.Type(), origI, -n, n-1)
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
	case types.Mapping:
		z, found, err := x.Get(y)
		if err != nil {
			return nil, err
		}
		if !found {
			return types.Nil, nil
		}
		return z, nil

	case types.Indexable:
		n := x.Len()
		i, err := AsInt(y)
		if err != nil {
			return nil, fmt.Errorf("%s index: %s", x.Type(), err)
		}
		origI := i
		if i < 0 {
			i += n
		}
		if i < 0 || i >= n {
			return nil, fmt.Errorf("%s index %d out of range [%d:%d]", x.Type(), origI, -n, n-1)
		}
		return x.Index(i), nil
	}
	return nil, fmt.Errorf("unsupported index operation %s[%s]", x.Type(), y.Type())
}

// AsInt enforces the type conversion rules for a value to an integer. Only
// Int and Float may convert to Int, and Float conversion is valid only if its
// value can be exactly represented by an integer.
func AsInt(v types.Value) (int, error) {
	switch v := v.(type) {
	case types.Int:
		return int(v), nil
	case types.Float:
		i := int(v)
		if types.Float(i) == v {
			return i, nil
		}
		return 0, fmt.Errorf("no exact integer representation possible for %s value %v", v.Type(), v)
	default:
		return 0, fmt.Errorf("%s cannot be converted to integer", v.Type())
	}
}

// AsString enforces the type conversion rules for a value to a string, which
// is no conversion at all - only String values can be returned as a Go string.
func AsString(v types.Value) (string, bool) {
	s, ok := v.(types.String)
	return string(s), ok
}

// Binary applies a strict binary operator (not AND or OR) to its operands. For
// equality tests or ordered comparisons, use Compare instead.
func Binary(op token.Token, l, r types.Value) (types.Value, error) {
	switch op {
	case token.PLUS:
		// + concatenation: only works on strings, no implicit conversion
		//
		// + arithmetic addition: if both operands are integers, the operation is
		// performed over integers and the result is an integer. Otherwise, if both
		// operands are numbers, then they are converted to floats, the operation
		// is performed following Go's rules for floating-point arithmetic (IEEE
		// 754), and the result is a float.
		switch l := l.(type) {
		case types.String:
			if r, ok := r.(types.String); ok {
				return l + r, nil
			}
		case types.Int:
			switch r := r.(type) {
			case types.Int:
				return l + r, nil
			case types.Float:
				lf := types.Float(l)
				return lf + r, nil
			}
		case types.Float:
			switch r := r.(type) {
			case types.Float:
				return l + r, nil
			case types.Int:
				rf := types.Float(r)
				return l + rf, nil
			}
		}

	case token.MINUS:
		// - arithmetic subtraction: if both operands are integers, the operation is
		// performed over integers and the result is an integer. Otherwise, if both
		// operands are numbers, then they are converted to floats, the operation
		// is performed following Go's rules for floating-point arithmetic (IEEE
		// 754), and the result is a float.
		switch l := l.(type) {
		case types.Int:
			switch r := r.(type) {
			case types.Int:
				return l - r, nil
			case types.Float:
				lf := types.Float(l)
				return lf - r, nil
			}
		case types.Float:
			switch r := r.(type) {
			case types.Float:
				return l - r, nil
			case types.Int:
				rf := types.Float(r)
				return l - rf, nil
			}
		}

		/*
			case syntax.STAR:
				switch x := x.(type) {
				case Int:
					switch y := y.(type) {
					case Int:
						return x * y, nil
					case Float:
						xf := Float(x)
						return xf * y, nil
					case String:
						return stringRepeat(y, x)
					case Bytes:
						return bytesRepeat(y, x)
					case *List:
						elems, err := tupleRepeat(Tuple(y.elems), x)
						if err != nil {
							return nil, err
						}
						return NewList(elems), nil
					case Tuple:
						return tupleRepeat(y, x)
					}
				case Float:
					switch y := y.(type) {
					case Float:
						return x * y, nil
					case Int:
						yf := Float(y)
						return x * yf, nil
					}
				case String:
					if y, ok := y.(Int); ok {
						return stringRepeat(x, y)
					}
				case Bytes:
					if y, ok := y.(Int); ok {
						return bytesRepeat(x, y)
					}
				case *List:
					if y, ok := y.(Int); ok {
						elems, err := tupleRepeat(Tuple(x.elems), y)
						if err != nil {
							return nil, err
						}
						return NewList(elems), nil
					}
				case Tuple:
					if y, ok := y.(Int); ok {
						return tupleRepeat(x, y)
					}
				}

			case syntax.SLASH:
				switch x := x.(type) {
				case Int:
					xf := Float(x)
					switch y := y.(type) {
					case Int:
						yf := Float(y)
						if yf == 0.0 {
							return nil, fmt.Errorf("floating-point division by zero")
						}
						return xf / yf, nil
					case Float:
						if y == 0.0 {
							return nil, fmt.Errorf("floating-point division by zero")
						}
						return xf / y, nil
					}
				case Float:
					switch y := y.(type) {
					case Float:
						if y == 0.0 {
							return nil, fmt.Errorf("floating-point division by zero")
						}
						return x / y, nil
					case Int:
						yf := Float(y)
						if yf == 0.0 {
							return nil, fmt.Errorf("floating-point division by zero")
						}
						return x / yf, nil
					}
				}

			case syntax.SLASHSLASH:
				switch x := x.(type) {
				case Int:
					switch y := y.(type) {
					case Int:
						if y == 0 {
							return nil, fmt.Errorf("floored division by zero")
						}
						return x / y, nil
					case Float:
						xf := Float(x)
						if y == 0.0 {
							return nil, fmt.Errorf("floored division by zero")
						}
						return floor(xf / y), nil
					}
				case Float:
					switch y := y.(type) {
					case Float:
						if y == 0.0 {
							return nil, fmt.Errorf("floored division by zero")
						}
						return floor(x / y), nil
					case Int:
						yf := Float(y)
						if yf == 0.0 {
							return nil, fmt.Errorf("floored division by zero")
						}
						return floor(x / yf), nil
					}
				}

			case syntax.PERCENT:
				switch x := x.(type) {
				case Int:
					switch y := y.(type) {
					case Int:
						if y == 0 {
							return nil, fmt.Errorf("integer modulo by zero")
						}
						return x % y, nil
					case Float:
						xf := Float(x)
						if y == 0 {
							return nil, fmt.Errorf("floating-point modulo by zero")
						}
						return xf.Mod(y), nil
					}
				case Float:
					switch y := y.(type) {
					case Float:
						if y == 0.0 {
							return nil, fmt.Errorf("floating-point modulo by zero")
						}
						return x.Mod(y), nil
					case Int:
						if y == 0 {
							return nil, fmt.Errorf("floating-point modulo by zero")
						}
						yf := Float(y)
						return x.Mod(yf), nil
					}
				case String:
					return interpolate(string(x), y)
				}

			case syntax.NOT_IN:
				z, err := Binary(syntax.IN, x, y)
				if err != nil {
					return nil, err
				}
				return !z.Truth(), nil

			case syntax.IN:
				switch y := y.(type) {
				case *List:
					for _, elem := range y.elems {
						if eq, err := Equal(elem, x); err != nil {
							return nil, err
						} else if eq {
							return True, nil
						}
					}
					return False, nil
				case Tuple:
					for _, elem := range y {
						if eq, err := Equal(elem, x); err != nil {
							return nil, err
						} else if eq {
							return True, nil
						}
					}
					return False, nil
				case Mapping: // e.g. dict
					// Ignore error from Get as we cannot distinguish true
					// errors (value cycle, type error) from "key not found".
					_, found, _ := y.Get(x)
					return Bool(found), nil
				case *Set:
					ok, err := y.Has(x)
					return Bool(ok), err
				case String:
					needle, ok := x.(String)
					if !ok {
						return nil, fmt.Errorf("'in <string>' requires string as left operand, not %s", x.Type())
					}
					return Bool(strings.Contains(string(y), string(needle))), nil
				case Bytes:
					switch needle := x.(type) {
					case Bytes:
						return Bool(strings.Contains(string(y), string(needle))), nil
					case Int:
						var b byte
						if err := AsInt(needle, &b); err != nil {
							return nil, fmt.Errorf("int in bytes: %s", err)
						}
						return Bool(strings.IndexByte(string(y), b) >= 0), nil
					default:
						return nil, fmt.Errorf("'in bytes' requires bytes or int as left operand, not %s", x.Type())
					}
				case rangeValue:
					i, err := NumberToInt(x)
					if err != nil {
						return nil, fmt.Errorf("'in <range>' requires integer as left operand, not %s", x.Type())
					}
					return Bool(y.contains(i)), nil
				}

			case syntax.PIPE:
				switch x := x.(type) {
				case Int:
					if y, ok := y.(Int); ok {
						return x | y, nil
					}

				case *Dict: // union
					if y, ok := y.(*Dict); ok {
						return x.Union(y), nil
					}

				case *Set: // union
					if y, ok := y.(*Set); ok {
						iter := Iterate(y)
						defer iter.Done()
						return x.Union(iter)
					}
				}

			case syntax.AMP:
				switch x := x.(type) {
				case Int:
					if y, ok := y.(Int); ok {
						return x & y, nil
					}
				case *Set: // intersection
					if y, ok := y.(*Set); ok {
						iter := y.Iterate()
						defer iter.Done()
						return x.Intersection(iter)
					}
				}

			case syntax.CIRCUMFLEX:
				switch x := x.(type) {
				case Int:
					if y, ok := y.(Int); ok {
						return x ^ y, nil
					}
				case *Set: // symmetric difference
					if y, ok := y.(*Set); ok {
						iter := y.Iterate()
						defer iter.Done()
						return x.SymmetricDifference(iter)
					}
				}

			case syntax.LTLT, syntax.GTGT:
				if x, ok := x.(Int); ok {
					y, err := AsInt32(y)
					if err != nil {
						return nil, err
					}
					if y < 0 {
						return nil, fmt.Errorf("negative shift count: %v", y)
					}
					if op == syntax.LTLT {
						if y >= 512 {
							return nil, fmt.Errorf("shift count too large: %v", y)
						}
						return x << uint(y), nil
					}
					return x >> uint(y), nil
				}
		*/
	default:
		// unknown operator
		goto unknown
	}

	// user-defined types
	// (nil, nil) => unhandled
	if l, ok := l.(types.HasBinary); ok {
		res, err := l.Binary(op, r, types.Left)
		if res != nil || err != nil {
			return res, err
		}
	}
	if r, ok := r.(types.HasBinary); ok {
		res, err := r.Binary(op, l, types.Right)
		if res != nil || err != nil {
			return res, err
		}
	}

	// unsupported operator
unknown:
	return nil, fmt.Errorf("unknown binary op: %s %s %s", l.Type(), op, r.Type())
}
