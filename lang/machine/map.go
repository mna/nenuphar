package machine

import (
	"fmt"

	"github.com/dolthub/swiss"
)

// A Map represents a map or dictionary. If you know the exact final number of
// entries, it is more efficient to call NewMap.
type Map struct {
	m *swiss.Map[Value, Value]
}

var (
	_ Value     = (*Map)(nil)
	_ Mapping   = (*Map)(nil)
	_ HasSetKey = (*Map)(nil)
	_ Iterable  = (*Map)(nil)
)

// NewMap returns a map with initial capacity for at least size items.
func NewMap(size int) *Map {
	m := swiss.NewMap[Value, Value](uint32(size))
	return &Map{m: m}
}

func (m *Map) String() string { return fmt.Sprintf("map(%p)", m) }
func (m *Map) Type() string   { return "map" }
func (m *Map) Get(k Value) (Value, bool, error) {
	v, ok := m.m.Get(k)
	return v, ok, nil
}
func (m *Map) SetKey(k, v Value) error {
	m.m.Put(k, v)
	return nil
}

func (m *Map) Iterate() Iterator {
	panic("unimplemented")
}

type mapIterator struct {
	it *swiss.Iterator[Value, Value]
}

func (it *mapIterator) Next(p *Value) bool {
	if !it.it.Next() {
		return false
	}

	k, v := it.it.Pair()
	*p = NewTuple([]Value{k, v})
	return true
}

func (it *mapIterator) Done() {}
