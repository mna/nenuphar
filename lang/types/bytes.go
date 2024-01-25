package types

import (
	"strconv"
	"strings"
)

// Bytes is the type of binary data. A Bytes encapsulates an immutable sequence
// of bytes. It is comparable, indexable, and sliceable, but not directly
// iterable; use bytes.elems() for an iterable view.
type Bytes string

var (
	_ Ordered   = Bytes("")
	_ Sliceable = Bytes("")
	_ Indexable = Bytes("")
)

func (b Bytes) String() string    { return strconv.Quote(string(b)) }
func (b Bytes) Type() string      { return "bytes" }
func (b Bytes) Freeze()           {} // immutable
func (b Bytes) Truth() Bool       { return len(b) > 0 }
func (b Bytes) Len() int          { return len(b) }
func (b Bytes) Index(i int) Value { return b[i : i+1] }

// TODO: bytes methods
//func (b Bytes) Attr(name string) (Value, error) { return builtinAttr(b, name, bytesMethods) }
//func (b Bytes) AttrNames() []string             { return builtinAttrNames(bytesMethods) }

func (b Bytes) Slice(start, end, step int) Value {
	if step == 1 {
		return b[start:end]
	}

	sign := signum(step)
	var str []byte
	for i := start; signum(end-i) == sign; i += step {
		str = append(str, b[i])
	}
	return Bytes(str)
}

func (b Bytes) Cmp(y Value, depth int) (int, error) {
	bb := y.(Bytes)
	return strings.Compare(string(b), string(bb)), nil
}
