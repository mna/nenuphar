package types

import (
	"fmt"
)

// A Tuple represents an immutable list of values (only the list is immutable,
// the values themselves are not). Iteration over a Tuple yields each of the
// tuple's values in order.
type Tuple struct {
	elems []Value
}

var (
	_ Value    = (*Tuple)(nil)
	_ Iterable = (*Tuple)(nil)
	_ HasEqual = (*Tuple)(nil)
)

func (t *Tuple) String() string    { return fmt.Sprintf("tuple(%p)", t) }
func (t *Tuple) Type() string      { return "tuple" }
func (t *Tuple) Iterate() Iterator { return &tupleIterator{elems: t.elems} }
func (t *Tuple) Equals(y Value) (bool, error) {
	yt := y.(*Tuple)
	if len(t.elems) != len(yt.elems) {
		return false, nil
	}
	for i, xv := range t.elems {
		yv := yt.elems[i]
		// TODO: need to use machine.Compare, but import cycle...
		_ = yv
	}
	panic("unimplemented")
}

type tupleIterator struct{ elems []Value }

func (it *tupleIterator) Next(p *Value) bool {
	if len(it.elems) > 0 {
		*p = it.elems[0]
		it.elems = it.elems[1:]
		return true
	}
	return false
}

func (it *tupleIterator) Done() {}
