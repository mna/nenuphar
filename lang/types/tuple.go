package types

import "fmt"

// A Tuple represents an immutable list of values. Iteration over a Tuple
// yields each of the tuple's values in order.
type Tuple []Value // TODO: this means a Tuple can't be a Map's key...

var (
	_ Value    = Tuple(nil)
	_ Iterable = Tuple(nil)
)

func (t Tuple) String() string    { return fmt.Sprintf("tuple(%p)", t) }
func (t Tuple) Type() string      { return "tuple" }
func (t Tuple) Iterate() Iterator { return &tupleIterator{elems: t} }

type tupleIterator struct{ elems Tuple }

func (it *tupleIterator) Next(p *Value) bool {
	if len(it.elems) > 0 {
		*p = it.elems[0]
		it.elems = it.elems[1:]
		return true
	}
	return false
}

func (it *tupleIterator) Done() {}
