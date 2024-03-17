package machine

import (
	"errors"
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

// Compare compares two values for the specified relational operator. The
// comparison operation must be one of EQL, NEQ, LT, LE, GT, or GE. Compare
// returns an error if an ordered comparison was requested for a pair of values
// that do not support it.
//
// Equality first compares the type of its operands. For values of same type,
// the values of the operands are compared. Strings are equal if they have the
// same byte content. Numbers are equal if they denote the same mathematical
// value, NaN values are greater than any other. Other values of the same type
// are compared by identity.
//
// The ordered operators work as follows. For numbers, if one value is a float
// then the other is converted to a float if necessary and they are compared
// according to their mathematical values, . Otherwise, if both arguments are
// strings, then their values are compared lexicographically. Boolean true is
// after false. Other types must implement the Ordered interface or rely on
// metamethods for comparison.
//
// Metamethods can be used to customize comparison for a value that supports
// it. The != operator is the negation of equality and cannot be customized.
func Compare(op token.Token, x, y types.Value) (bool, error) {
	if sameType(x, y) {
		if xcomp, ok := x.(types.Ordered); ok {
			t, err := xcomp.Cmp(y)
			if err != nil {
				return false, err
			}
			return threeway(op, t), nil
		}

		if x, ok := x.(types.HasMetamap); ok {
			if meta := x.Metamap(); meta != nil {
				// TODO: translate >= to <=, > to < with operands swapped, or just use a __cmp metamethod?
				//res, err := CallMetamethod(meta, op, x, y, types.Left) // TODO: translate op to metamethod name
				//if res != nil || err != nil {
				//	return res, err
				//}
			}
		}
		if y, ok := y.(types.HasMetamap); ok {
			if meta := y.Metamap(); meta != nil {
				// TODO: translate >= to <=, > to < with operands swapped, or just use a __cmp metamethod?
				//res, err := CallMetamethod(meta, op, x, y, types.Right) // TODO: translate op to metamethod name
				//if res != nil || err != nil {
				//	return res, err
				//}
			}
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

	if x, ok := x.(types.HasMetamap); ok {
		if meta := x.Metamap(); meta != nil {
			// TODO: translate >= to <=, > to < with operands swapped, or just use a __cmp metamethod?
			//res, err := CallMetamethod(meta, op, x, y, types.Left) // TODO: translate op to metamethod name
			//if res != nil || err != nil {
			//	return res, err
			//}
		}
	}
	if y, ok := y.(types.HasMetamap); ok {
		if meta := y.Metamap(); meta != nil {
			// TODO: translate >= to <=, > to < with operands swapped, or just use a __cmp metamethod?
			//res, err := CallMetamethod(meta, op, x, y, types.Right) // TODO: translate op to metamethod name
			//if res != nil || err != nil {
			//	return res, err
			//}
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
	// TODO: add support for metamap, see how Lua does it.
	switch x := x.(type) {
	case types.HasSetKey:
		if err := x.SetKey(y, z); err != nil {
			return err
		}

	case types.HasSetIndex:
		n := x.Len()
		i, err := AsExactInt(y)
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
		return fmt.Errorf("%s value does not support indexed assignment", x.Type())
	}
	return nil
}

// getIndex implements x[y].
func getIndex(x, y types.Value) (types.Value, error) {
	fail := true

	switch x := x.(type) {
	case types.Mapping:
		z, found, err := x.Get(y)
		if err != nil {
			return nil, err
		}
		if found {
			return z, nil
		}
		// continue in case a metamethod is possible
		fail = false

	case types.Indexable:
		// TODO: support metamethod to index a non-integer field for Indexable? I think not.
		n := x.Len()
		i, err := AsExactInt(y)
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

	if x, ok := x.(types.HasMetamap); ok {
		if meta := x.Metamap(); meta != nil {
			//res, err := CallMetamethod(meta, op, x, y) // TODO: translate op to metamethod name
			//if res != nil || err != nil {
			//	return res, err
			//}
		}
	}
	if !fail {
		return types.Nil, nil
	}
	return nil, fmt.Errorf("unsupported index operation %s[%s]", x.Type(), y.Type())
}

// getAttr implements x.dot.
func getAttr(x types.Value, name string) (types.Value, error) {
	hasAttr, ok := x.(types.HasAttrs)
	if !ok {
		// fallback to getIndex, which will use metamap if available.
		return getIndex(x, types.String(name))
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

	//// TODO: add spelling hint
	//if n := spell.Nearest(name, hasAttr.AttrNames()); n != "" {
	//	errmsg = fmt.Sprintf("%s (did you mean .%s?)", errmsg, n)
	//}

	return nil, errors.New(errmsg)
}

// setField implements x.name = y.
func setField(x types.Value, name string, y types.Value) error {
	if x, ok := x.(types.HasSetField); ok {
		err := x.SetField(name, y)
		if _, ok := err.(types.NoSuchAttrError); ok {
			//// TODO: No such field: check spelling.
			//if n := spell.Nearest(name, x.AttrNames()); n != "" {
			//	err = fmt.Errorf("%s (did you mean .%s?)", err, n)
			//}
		}
		return err
	}

	// fallback to setIndex
	return setIndex(x, types.String(name), y)
}

// AsExactInt enforces the type conversion rules for a value to an integer.
// Only Int and Float may convert to Int, and Float conversion is valid only if
// its value can be exactly represented by an integer.
func AsExactInt(v types.Value) (int, error) {
	switch v := v.(type) {
	case types.Int:
		return int(v), nil
	case types.Float:
		i, err := floatToInt(v)
		if err != nil {
			return 0, err
		}
		return int(i), nil
	default:
		return 0, fmt.Errorf("%s cannot be converted to integer", v.Type())
	}
}

func floatToInt(f types.Float) (types.Int, error) {
	i := types.Int(f)
	if types.Float(i) == f {
		return i, nil
	}
	return 0, fmt.Errorf("no exact integer representation possible for %s value %v", f.Type(), f)
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
	// first try to perform the binary operations supported as built-ins.
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

	case token.STAR:
		// * arithmetic multiplication: if both operands are integers, the
		// operation is performed over integers and the result is an integer.
		// Otherwise, if both operands are numbers, then they are converted to
		// floats, the operation is performed following Go's rules for
		// floating-point arithmetic (IEEE 754), and the result is a float.
		switch l := l.(type) {
		case types.Int:
			switch r := r.(type) {
			case types.Int:
				return l * r, nil
			case types.Float:
				lf := types.Float(l)
				return lf * r, nil
			}
		case types.Float:
			switch r := r.(type) {
			case types.Float:
				return l * r, nil
			case types.Int:
				rf := types.Float(r)
				return l * rf, nil
			}
		}

	case token.SLASH:
		// / float division: the operation is performed by converting the operands
		// to floats and the result is always a float.
		switch l := l.(type) {
		case types.Int:
			lf := types.Float(l)
			switch r := r.(type) {
			case types.Int:
				rf := types.Float(r)
				if rf == 0.0 {
					return nil, fmt.Errorf("floating-point division by zero")
				}
				return lf / rf, nil
			case types.Float:
				if r == 0.0 {
					return nil, fmt.Errorf("floating-point division by zero")
				}
				return lf / r, nil
			}
		case types.Float:
			switch r := r.(type) {
			case types.Float:
				if r == 0.0 {
					return nil, fmt.Errorf("floating-point division by zero")
				}
				return l / r, nil
			case types.Int:
				rf := types.Float(r)
				if rf == 0.0 {
					return nil, fmt.Errorf("floating-point division by zero")
				}
				return l / rf, nil
			}
		}

	case token.SLASHSLASH:
		// // floor division: returns the greatest integer value less than or equal
		// to the result. If both operands are integers, the operation is performed
		// over integers and the result is an integer. Otherwise, if both operands
		// are numbers, then they are converted to floats, the operation is
		// performed following Go's rules for floating-point arithmetic (IEEE 754)
		// and the result is obtained using Go's math.Floor.
		switch l := l.(type) {
		case types.Int:
			switch r := r.(type) {
			case types.Int:
				if r == 0 {
					return nil, fmt.Errorf("floored division by zero")
				}
				return floorDiv(l, r), nil
			case types.Float:
				lf := types.Float(l)
				if r == 0.0 {
					return nil, fmt.Errorf("floored division by zero")
				}
				return types.Float(math.Floor(float64(lf) / float64(r))), nil
			}
		case types.Float:
			switch r := r.(type) {
			case types.Float:
				if r == 0.0 {
					return nil, fmt.Errorf("floored division by zero")
				}
				return types.Float(math.Floor(float64(l) / float64(r))), nil
			case types.Int:
				rf := types.Float(r)
				if rf == 0.0 {
					return nil, fmt.Errorf("floored division by zero")
				}
				return types.Float(math.Floor(float64(l) / float64(rf))), nil
			}
		}

	case token.PERCENT: // TODO: test and compare with Lua/Python for correctness
		// % modulo division: returns the remainder of a division that rounds the
		// quotient towards minus infinity (floor division). If both operands are
		// integers, the operation is performed over integers and the result is an
		// integer. Otherwise, if both operands are numbers, then they are
		// converted to floats.
		switch l := l.(type) {
		case types.Int:
			switch r := r.(type) {
			case types.Int:
				if r == 0 {
					return nil, fmt.Errorf("integer modulo by zero")
				}
				return modInt(l, r), nil
			case types.Float:
				lf := types.Float(l)
				if r == 0 {
					return nil, fmt.Errorf("floating-point modulo by zero")
				}
				return modFloat(lf, r), nil
			}
		case types.Float:
			switch r := r.(type) {
			case types.Float:
				if r == 0.0 {
					return nil, fmt.Errorf("floating-point modulo by zero")
				}
				return modFloat(l, r), nil
			case types.Int:
				if r == 0 {
					return nil, fmt.Errorf("floating-point modulo by zero")
				}
				rf := types.Float(r)
				return modFloat(l, rf), nil
			}
		}

	case token.CIRCUMFLEX:
		// ^ arithmetic exponentiation: the operation is performed by converting
		// the operands to floats and the result is always a float, as returned by
		// Go's math.Pow.
		switch l := l.(type) {
		case types.Int:
			lf := types.Float(l)
			switch r := r.(type) {
			case types.Int:
				rf := types.Float(r)
				return types.Float(math.Pow(float64(lf), float64(rf))), nil
			case types.Float:
				return types.Float(math.Pow(float64(lf), float64(r))), nil
			}
		case types.Float:
			switch r := r.(type) {
			case types.Float:
				return types.Float(math.Pow(float64(l), float64(r))), nil
			case types.Int:
				rf := types.Float(r)
				return types.Float(math.Pow(float64(l), float64(rf))), nil
			}
		}

	case token.AMPERSAND:
		// & bitwise AND: the operation is performed by converting its operands to
		// integers and operating on all bits of those integers. The result is an
		// integer. The operation fails if a float is not representable as an
		// integer.
		switch l := l.(type) {
		case types.Int:
			switch r := r.(type) {
			case types.Int:
				return l & r, nil
			case types.Float:
				ri, err := floatToInt(r)
				if err != nil {
					return nil, err
				}
				return l & ri, nil
			}
		case types.Float:
			switch r := r.(type) {
			case types.Int:
				li, err := floatToInt(l)
				if err != nil {
					return nil, err
				}
				return li & r, nil
			case types.Float:
				li, err := floatToInt(l)
				if err != nil {
					return nil, err
				}
				ri, err := floatToInt(r)
				if err != nil {
					return nil, err
				}
				return li & ri, nil
			}
		}

	case token.PIPE:
		// | bitwise OR: the operation is performed by converting its operands to
		// integers and operating on all bits of those integers. The result is an
		// integer. The operation fails if a float is not representable as an
		// integer.
		switch l := l.(type) {
		case types.Int:
			switch r := r.(type) {
			case types.Int:
				return l | r, nil
			case types.Float:
				ri, err := floatToInt(r)
				if err != nil {
					return nil, err
				}
				return l | ri, nil
			}
		case types.Float:
			switch r := r.(type) {
			case types.Int:
				li, err := floatToInt(l)
				if err != nil {
					return nil, err
				}
				return li | r, nil
			case types.Float:
				li, err := floatToInt(l)
				if err != nil {
					return nil, err
				}
				ri, err := floatToInt(r)
				if err != nil {
					return nil, err
				}
				return li | ri, nil
			}
		}

	case token.TILDE:
		// ~ bitwise exclusive OR: the operation is performed by converting its
		// operands to integers and operating on all bits of those integers. The
		// result is an integer. The operation fails if a float is not
		// representable as an integer.
		switch l := l.(type) {
		case types.Int:
			switch r := r.(type) {
			case types.Int:
				return l ^ r, nil
			case types.Float:
				ri, err := floatToInt(r)
				if err != nil {
					return nil, err
				}
				return l ^ ri, nil
			}
		case types.Float:
			switch r := r.(type) {
			case types.Int:
				li, err := floatToInt(l)
				if err != nil {
					return nil, err
				}
				return li ^ r, nil
			case types.Float:
				li, err := floatToInt(l)
				if err != nil {
					return nil, err
				}
				ri, err := floatToInt(r)
				if err != nil {
					return nil, err
				}
				return li ^ ri, nil
			}
		}

	case token.LTLT:
		// << bitwise left shift: the operation is performed by converting its
		// operands to integers and operating on all bits of those integers. The
		// result is an integer. The operation fails if a float is not
		// representable as an integer. It fills the vacant bits with zeros.
		// Negative displacements shift to the other direction.
		switch l := l.(type) {
		case types.Int:
			switch r := r.(type) {
			case types.Int:
				if r < 0 {
					return types.Int(uint(l) >> -r), nil
				}
				return l << r, nil
			case types.Float:
				ri, err := floatToInt(r)
				if err != nil {
					return nil, err
				}
				if ri < 0 {
					return types.Int(uint(l) >> -ri), nil
				}
				return l << ri, nil
			}
		case types.Float:
			switch r := r.(type) {
			case types.Int:
				li, err := floatToInt(l)
				if err != nil {
					return nil, err
				}
				if r < 0 {
					return types.Int(uint(li) >> -r), nil
				}
				return li << r, nil
			case types.Float:
				li, err := floatToInt(l)
				if err != nil {
					return nil, err
				}
				ri, err := floatToInt(r)
				if err != nil {
					return nil, err
				}
				if ri < 0 {
					return types.Int(uint(li) >> -ri), nil
				}
				return li << ri, nil
			}
		}

	case token.GTGT:
		// >> bitwise right shift: the operation is performed by converting its
		// operands to integers and operating on all bits of those integers. The
		// result is an integer. The operation fails if a float is not
		// representable as an integer. It fills the vacant bits with zeros.
		// Negative displacements shift to the other direction.
		switch l := l.(type) {
		case types.Int:
			switch r := r.(type) {
			case types.Int:
				if r < 0 {
					return l << -r, nil
				}
				return types.Int(uint(l) >> r), nil
			case types.Float:
				ri, err := floatToInt(r)
				if err != nil {
					return nil, err
				}
				if ri < 0 {
					return l << -ri, nil
				}
				return types.Int(uint(l) >> ri), nil
			}
		case types.Float:
			switch r := r.(type) {
			case types.Int:
				li, err := floatToInt(l)
				if err != nil {
					return nil, err
				}
				if r < 0 {
					return li << -r, nil
				}
				return types.Int(uint(li) >> r), nil
			case types.Float:
				li, err := floatToInt(l)
				if err != nil {
					return nil, err
				}
				ri, err := floatToInt(r)
				if err != nil {
					return nil, err
				}
				if ri < 0 {
					return li << -ri, nil
				}
				return types.Int(uint(li) >> ri), nil
			}
		}

	default:
		// unknown operator
		goto unknown
	}

	// user-defined types with direct binary operators support
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

	// user-defined types with metatable support
	// (nil, nil) => no metamethod found
	if l, ok := l.(types.HasMetamap); ok {
		if meta := l.Metamap(); meta != nil {
			//res, err := CallMetamethod(meta, op, l, r, types.Left) // TODO: translate op to metamethod name
			//if res != nil || err != nil {
			//	return res, err
			//}
		}
	}
	if r, ok := r.(types.HasMetamap); ok {
		if meta := r.Metamap(); meta != nil {
			//res, err := CallMetamethod(meta, op, l, r, types.Right) // TODO: translate op to metamethod name
			//if res != nil || err != nil {
			//	return res, err
			//}
		}
	}

unknown:
	return nil, fmt.Errorf("unsupported binary op: %s %s %s", l.Type(), op, r.Type())
}

func floorDiv(l, r types.Int) types.Int {
	if r < 0 {
		l, r = -l, -r
	}
	m := l % r
	if m < 0 {
		m += r
	}
	return (l - m) / r
}

func modInt(l, r types.Int) types.Int {
	return (l%r + r) % r
}

func modFloat(l, r types.Float) types.Float {
	v := types.Float(math.Mod(float64(l), float64(r)))
	if v < 0 {
		v += r
	}
	return v
}

// Unary applies a unary operator (only +, -, ~, # and "not" as the others -
// "try" and "must" - are compiled to catch statements) to its operand.
func Unary(op token.Token, x types.Value) (types.Value, error) {
	// The NOT operator is not customizable.
	if op == token.NOT {
		return !Truth(x), nil
	}

	switch op {
	case token.PLUS:
		// + unary addition: returns the integer or float unchanged.
		switch x := x.(type) {
		case types.Int:
			return +x, nil
		case types.Float:
			return +x, nil
		}

	case token.MINUS:
		// - unary subtraction: switches the sign of the integer or float,
		// returning the same type.
		switch x := x.(type) {
		case types.Int:
			return -x, nil
		case types.Float:
			return -x, nil
		}

	case token.TILDE:
		// ~ unary bitwise NOT: converts the operand to int and switches all bits,
		// as if the integer was unsigned. The result is an integer. The operation
		// fails if the float is not representable as an integer.
		switch x := x.(type) {
		case types.Int:
			return types.Int(^uint(x)), nil
		case types.Float:
			xi, err := floatToInt(x)
			if err != nil {
				return nil, err
			}
			return types.Int(^uint(xi)), nil
		}

	case token.POUND:
		// # len operator: the length of a string is its number of bytes, as an
		// integer.
		switch x := x.(type) {
		case types.String:
			return types.Int(len(x)), nil
		}

	default:
		goto unknown
	}

	if x, ok := x.(types.HasUnary); ok {
		// (nil, nil) => unhandled
		y, err := x.Unary(op)
		if y != nil || err != nil {
			return y, err
		}
	}

unknown:
	return nil, fmt.Errorf("unsupported unary op: %s %s", op, x.Type())
}
