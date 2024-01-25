package types

import (
	"fmt"

	"github.com/mna/nenuphar-wip/syntax"
)

// A Map represents a map or dictionary. If you know the exact final number of
// entries, it is more efficient to call NewMap.
type Map struct {
	m      map[Value]Value
	frozen bool
	// TODO: itercount? While Go's map can be modified during iteration, that
	// would not play well with the custom iterators needed by the machine. And
	// how will those iterators work without access to Go's internal map
	// iteration?
}

// NewMap returns a map with initial capacity for at least size items.
func NewMap(size int) *Map {
	m := make(map[Value]Value, size)
	return &Map{m: m}
}

func (m *Map) Clear() error {
	for k := range m.m {
		delete(m.m, k)
	}
	return nil
}
func (m *Map) Delete(k Value) (v Value, found bool, err error) { return d.ht.delete(k) }
func (m *Map) Get(k Value) (v Value, found bool, err error)    { return d.ht.lookup(k) }
func (m *Map) Items() []Tuple                                  { return d.ht.items() }
func (m *Map) Keys() []Value                                   { return d.ht.keys() }
func (m *Map) Len() int                                        { return int(d.ht.len) }
func (m *Map) Iterate() Iterator                               { return d.ht.iterate() }
func (m *Map) SetKey(k, v Value) error                         { return d.ht.insert(k, v) }
func (m *Map) String() string                                  { return toString(d) }
func (m *Map) Type() string                                    { return "dict" }
func (m *Map) Freeze()                                         { d.ht.freeze() }
func (m *Map) Truth() Bool                                     { return d.Len() > 0 }
func (m *Map) Hash() (uint32, error)                           { return 0, fmt.Errorf("unhashable type: dict") }

func (m *Map) Union(y *Map) *Map {
	z := new(Dict)
	z.ht.init(d.Len()) // a lower bound
	z.ht.addAll(&d.ht) // can't fail
	z.ht.addAll(&y.ht) // can't fail
	return z
}

// TODO: map methods
//func (m *Map) Attr(name string) (Value, error) { return builtinAttr(d, name, dictMethods) }
//func (m *Map) AttrNames() []string             { return builtinAttrNames(dictMethods) }

func (d *Dict) CompareSameType(op syntax.Token, y_ Value, depth int) (bool, error) {
	y := y_.(*Dict)
	switch op {
	case syntax.EQL:
		ok, err := dictsEqual(d, y, depth)
		return ok, err
	case syntax.NEQ:
		ok, err := dictsEqual(d, y, depth)
		return !ok, err
	default:
		return false, fmt.Errorf("%s %s %s not implemented", d.Type(), op, y.Type())
	}
}

func dictsEqual(x, y *Dict, depth int) (bool, error) {
	if x.Len() != y.Len() {
		return false, nil
	}
	for e := x.ht.head; e != nil; e = e.next {
		key, xval := e.key, e.value

		if yval, found, _ := y.Get(key); !found {
			return false, nil
		} else if eq, err := EqualDepth(xval, yval, depth-1); err != nil {
			return false, err
		} else if !eq {
			return false, nil
		}
	}
	return true, nil
}
