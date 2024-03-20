package types

import "fmt"

// A Map represents a map or dictionary. If you know the exact final number of
// entries, it is more efficient to call NewMap.
type Map struct {
	m         map[Value]Value
	itercount uint32 // number of active iterators
}

var (
	_ Value     = (*Map)(nil)
	_ Mapping   = (*Map)(nil)
	_ HasSetKey = (*Map)(nil)
	_ Iterable  = (*Map)(nil)
)

// NewMap returns a map with initial capacity for at least size items.
func NewMap(size int) *Map {
	m := make(map[Value]Value, size)
	return &Map{m: m}
}

func (m *Map) String() string { return fmt.Sprintf("map(%p)", m) }
func (m *Map) Type() string   { return "map" }
func (m *Map) Get(k Value) (Value, bool, error) {
	v, ok := m.m[k]
	return v, ok, nil
}
func (m *Map) SetKey(k, v Value) error {
	if err := m.checkMutable("insert into"); err != nil {
		return err
	}

	m.m[k] = v
	return nil
}

func (m *Map) Iterate() Iterator {
	// TODO: use https://github.com/dolthub/swiss/blob/v0.2.1/map.go#L214 and modify to return iterator?
	panic("unimplemented")
}

// checkMutable reports an error if the map should not be mutated. verb+" map"
// should describe the operation.
func (m *Map) checkMutable(verb string) error {
	if m.itercount > 0 {
		return fmt.Errorf("cannot %s map during iteration", verb)
	}
	return nil
}
