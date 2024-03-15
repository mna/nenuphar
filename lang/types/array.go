package types

import (
	"fmt"
)

// An *Array represents a list of values.
type Array struct {
	elems     []Value
	itercount uint32 // number of active iterators
}

var (
	_ Value       = (*Array)(nil)
	_ Indexable   = (*Array)(nil)
	_ HasSetIndex = (*Array)(nil)
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

func (a *Array) String() string    { return "TODO(array)" }
func (a *Array) Type() string      { return "array" }
func (a *Array) Len() int          { return len(a.elems) }
func (a *Array) Index(i int) Value { return a.elems[i] }
func (a *Array) SetIndex(i int, v Value) error {
	if err := a.checkMutable("assign to element of"); err != nil {
		return err
	}
	// TODO: return catchable error on out of bounds
	a.elems[i] = v
	return nil
}
