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
	_ Value = (*Array)(nil)
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

func (a *Array) String() string { return "TODO(array)" }
func (a *Array) Type() string   { return "array" }
