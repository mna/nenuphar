package types

import (
	"strconv"
	"strings"
)

// String is the type of a text string. It encapsulates an immutable sequence
// of bytes.
type String string

var (
	_ Value   = String("")
	_ Ordered = String("")
)

func (s String) String() string { return strconv.Quote(string(s)) }
func (s String) Type() string   { return "string" }

func (s String) Cmp(y Value) (int, error) {
	sb := y.(String)
	return strings.Compare(string(s), string(sb)), nil
}
