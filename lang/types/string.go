package types

import (
	"strconv"
	"strings"
)

// String is the type of a text string. It encapsulates an immutable sequence
// of bytes. Iteration on a string yields each byte.
type String string

var (
	_ Value    = String("")
	_ Ordered  = String("")
	_ Iterable = String("")
)

func (s String) String() string { return strconv.Quote(string(s)) }
func (s String) Type() string   { return "string" }

func (s String) Cmp(y Value) (int, error) {
	sb := y.(String)
	return strings.Compare(string(s), string(sb)), nil
}

func (s String) Iterate() Iterator {
	return &stringIterator{s: string(s)}
}

type stringIterator struct {
	s string
}

func (it *stringIterator) Next(p *Value) bool {
	if len(it.s) > 0 {
		*p = String(it.s[0])
		it.s = it.s[1:]
		return true
	}
	return false
}

func (it *stringIterator) Done() {}
