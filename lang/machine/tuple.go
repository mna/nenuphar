package machine

import (
	"fmt"

	"github.com/mna/nenuphar/lang/token"
)

// A Tuple represents an immutable list of values (only the list is immutable,
// the values themselves are not). Iteration over a Tuple yields each of the
// tuple's values in order.
type Tuple struct {
	elems []Value
}

// NilaryTuple is the value of an empty tuple.
var NilaryTuple = NewTuple(nil)

var (
	_ Value     = (*Tuple)(nil)
	_ Indexable = (*Tuple)(nil)
	_ Iterable  = (*Tuple)(nil)
	_ HasEqual  = (*Tuple)(nil)
	_ Sequence  = (*Tuple)(nil)
)

// NewTuple returns a tuple containing the specified elements. Callers should
// not subsequently modify elems.
func NewTuple(elems []Value) *Tuple { return &Tuple{elems: elems} }

func (t *Tuple) String() string    { return fmt.Sprintf("tuple(%p)", t) }
func (t *Tuple) Type() string      { return "tuple" }
func (t *Tuple) Iterate() Iterator { return &tupleIterator{elems: t.elems} }
func (t *Tuple) Len() int          { return len(t.elems) }
func (t *Tuple) Index(i int) Value { return t.elems[i] }
func (t *Tuple) Equals(y Value) (bool, error) {
	yt := y.(*Tuple)
	if len(t.elems) != len(yt.elems) {
		return false, nil
	}
	for i, xv := range t.elems {
		yv := yt.elems[i]
		eq, err := Compare(token.EQL, xv, yv)
		if !eq || err != nil {
			return eq, err
		}
	}
	return true, nil
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
