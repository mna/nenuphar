package types

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// String is the type of a text string. It encapsulates an immutable sequence
// of bytes, but strings are not directly iterable. Instead, iterate over the
// result of calling one of these four methods: codepoints, codepoint_ords,
// elems, elem_ords.
// TODO: revisit this ^
//
// Strings typically contain text; use Bytes for binary data. The spec defines
// text strings as sequences of UTF-8 codes that encode Unicode code points.
type String string

var (
	_ Indexable = String("")
	_ Sliceable = String("")
	_ Ordered   = String("")
)

func (s String) String() string    { return strconv.Quote(string(s)) }
func (s String) Type() string      { return "string" }
func (s String) Freeze()           {} // immutable
func (s String) Truth() Bool       { return len(s) > 0 }
func (s String) Len() int          { return len(s) } // number of bytes
func (s String) Index(i int) Value { return s[i : i+1] }

func (s String) Slice(start, end, step int) Value {
	if step == 1 {
		return s[start:end]
	}

	sign := signum(step)
	var str []byte
	for i := start; signum(end-i) == sign; i += step {
		str = append(str, s[i])
	}
	return String(str)
}

// From Hacker's Delight, section 2.8. Returns +1, 0 or -1.
func signum64(x int64) int { return int(uint64(x>>63) | uint64(-x)>>63) }
func signum(x int) int     { return signum64(int64(x)) }

// TODO: string methods
//func (s String) Attr(name string) (Value, error) { return builtinAttr(s, name, stringMethods) }
//func (s String) AttrNames() []string             { return builtinAttrNames(stringMethods) }

func (s String) Cmp(y Value, depth int) (int, error) {
	sb := y.(String)
	return strings.Compare(string(s), string(sb)), nil
}

// A stringElems is an iterable whose iterator yields a sequence of elements
// (bytes), either numerically or as successive substrings. It is an indexable
// sequence.
type stringElems struct {
	s    String
	ords bool
}

var (
	_ Sequence  = (*stringElems)(nil)
	_ Indexable = (*stringElems)(nil)
)

func (si stringElems) String() string {
	if si.ords {
		return si.s.String() + ".elem_ords()"
	}
	return si.s.String() + ".elems()"
}
func (si stringElems) Type() string      { return "string.elems" }
func (si stringElems) Freeze()           {} // immutable
func (si stringElems) Truth() Bool       { return True }
func (si stringElems) Iterate() Iterator { return &stringElemsIterator{si, 0} }
func (si stringElems) Len() int          { return len(si.s) }
func (si stringElems) Index(i int) Value {
	if si.ords {
		return Int(int(si.s[i]))
	}
	// TODO(adonovan): opt: preallocate canonical 1-byte strings
	// to avoid interface allocation.
	return si.s[i : i+1]
}

type stringElemsIterator struct {
	si stringElems
	i  int
}

func (it *stringElemsIterator) Next(p *Value) bool {
	if it.i == len(it.si.s) {
		return false
	}
	*p = it.si.Index(it.i)
	it.i++
	return true
}

func (*stringElemsIterator) Done() {}

// A stringCodepoints is an iterable whose iterator yields a sequence of
// Unicode code points, either numerically or as successive substrings. It is
// not indexable.
type stringCodepoints struct {
	s    String
	ords bool
}

var _ Iterable = (*stringCodepoints)(nil)

func (si stringCodepoints) String() string {
	if si.ords {
		return si.s.String() + ".codepoint_ords()"
	}
	return si.s.String() + ".codepoints()"
}
func (si stringCodepoints) Type() string      { return "string.codepoints" }
func (si stringCodepoints) Freeze()           {} // immutable
func (si stringCodepoints) Truth() Bool       { return True }
func (si stringCodepoints) Iterate() Iterator { return &stringCodepointsIterator{si, 0} }

type stringCodepointsIterator struct {
	si stringCodepoints
	i  int
}

func (it *stringCodepointsIterator) Next(p *Value) bool {
	s := it.si.s[it.i:]
	if s == "" {
		return false
	}
	r, sz := utf8.DecodeRuneInString(string(s))
	if !it.si.ords {
		if r == utf8.RuneError {
			*p = String(r)
		} else {
			*p = s[:sz]
		}
	} else {
		*p = Int(int(r))
	}
	it.i += sz
	return true
}

func (*stringCodepointsIterator) Done() {}
