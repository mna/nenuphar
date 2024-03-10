package types

import (
	"fmt"

	"github.com/mna/nenuphar/lang/token"
)

// An *Array represents a list of values.
type Array struct {
	elems     []Value
	itercount uint32 // number of active iterators
}

var (
	_ Value     = (*Array)(nil)
	_ Indexable = (*Array)(nil)
	_ Sequence  = (*Array)(nil)
)

// NewArray returns an array containing the specified elements. Callers should
// not subsequently modify elems.
func NewArray(elems []Value) *Array { return &Array{elems: elems} }

// checkMutable reports an error if the array should not be mutated.
// verb+" array" should describe the operation.
func (a *Array) checkMutable(verb string) error {
	if a.itercount > 0 {
		return fmt.Errorf("cannot %s array during iteration", verb)
	}
	return nil
}

func (a *Array) String() string    { return toString(a) }
func (a *Array) Type() string      { return "array" }
func (a *Array) Truth() Bool       { return true }
func (a *Array) Len() int          { return len(a.elems) }
func (a *Array) Index(i int) Value { return a.elems[i] }

func (a *Array) Slice(start, end, step int) Value {
	if step == 1 {
		elems := append([]Value{}, a.elems[start:end]...)
		return NewArray(elems)
	}

	sign := signum(step)
	var list []Value
	for i := start; signum(end-i) == sign; i += step {
		list = append(list, a.elems[i])
	}
	return NewArray(list)
}

func (a *Array) Attr(name string) (Value, error) { return builtinAttr(a, name, listMethods) }
func (a *Array) AttrNames() []string             { return builtinAttrNames(listMethods) }

func (a *Array) Iterate() Iterator {
	a.itercount++
	return &arrayIterator{a: a}
}

func (a *Array) CompareSameType(op token.Token, y_ Value, depth int) (bool, error) {
	y := y_.(*Array)
	// It's tempting to check x == y as an optimization here,
	// but wrong because a list containing NaN is not equal to itself.
	return sliceCompare(op, a.elems, y.elems, depth)
}

func sliceCompare(op token.Token, x, y []Value, depth int) (bool, error) {
	// Fast path: check length.
	if len(x) != len(y) && (op == token.EQL || op == token.NEQ) {
		return op == token.NEQ, nil
	}

	// Find first element that is not equal in both lists.
	for i := 0; i < len(x) && i < len(y); i++ {
		if eq, err := EqualDepth(x[i], y[i], depth-1); err != nil {
			return false, err
		} else if !eq {
			switch op {
			case token.EQL:
				return false, nil
			case token.NEQ:
				return true, nil
			default:
				return CompareDepth(op, x[i], y[i], depth-1)
			}
		}
	}

	return threeway(op, len(x)-len(y)), nil
}

type arrayIterator struct {
	a *Array
	i int
}

func (it *arrayIterator) Next(p *Value) bool {
	if it.i < it.a.Len() {
		*p = it.a.elems[it.i]
		it.i++
		return true
	}
	return false
}

func (it *arrayIterator) Done() {
	it.a.itercount--
}

func (a *Array) SetIndex(i int, v Value) error {
	if err := a.checkMutable("assign to element of"); err != nil {
		return err
	}
	a.elems[i] = v
	return nil
}

func (a *Array) Append(v Value) error {
	if err := a.checkMutable("append to"); err != nil {
		return err
	}
	a.elems = append(a.elems, v)
	return nil
}

func (a *Array) Clear() error {
	if err := a.checkMutable("clear"); err != nil {
		return err
	}
	for i := range a.elems {
		a.elems[i] = nil // aid GC
	}
	a.elems = a.elems[:0]
	return nil
}
