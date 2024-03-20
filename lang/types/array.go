package types

import (
	"fmt"
)

// An *Array represents a list of values. Iteration over an array yields each
// of the array's values in order.
type Array struct {
	elems     []Value
	itercount uint32 // number of active iterators
}

var (
	_ Value       = (*Array)(nil)
	_ Indexable   = (*Array)(nil)
	_ HasSetIndex = (*Array)(nil)
	_ Iterable    = (*Array)(nil)
)

// NewArray returns an array containing the specified elements. Callers should
// not subsequently modify elems.
func NewArray(elems []Value) *Array { return &Array{elems: elems} }

// checkMutable reports an error if the array should not be mutated.
// verb+" array" should describe the operation.
func (a *Array) checkMutable(verb string) error {
	// TODO: TBD if this needs to be maintained as constraint
	if a.itercount > 0 {
		return fmt.Errorf("cannot %s array during iteration", verb)
	}
	return nil
}

func (a *Array) String() string    { return fmt.Sprintf("array(%p)", a) }
func (a *Array) Type() string      { return "array" }
func (a *Array) Len() int          { return len(a.elems) }
func (a *Array) Index(i int) Value { return a.elems[i] }

func (a *Array) Iterate() Iterator {
	a.itercount++
	return &arrayIterator{a: a}
}

func (a *Array) SetIndex(i int, v Value) error {
	if err := a.checkMutable("assign to element of"); err != nil {
		return err
	}
	a.elems[i] = v
	return nil
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
