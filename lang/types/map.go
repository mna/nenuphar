package types

// A Map represents a map or dictionary. If you know the exact final number of
// entries, it is more efficient to call NewMap.
type Map struct {
	m map[Value]Value
	// TODO: itercount? While Go's map can be modified during iteration, that
	// would not play well with the custom iterators needed by the machine. And
	// how will those iterators work without access to Go's internal map
	// iteration?
}

var (
	_ Value = (*Map)(nil)
)

// NewMap returns a map with initial capacity for at least size items.
func NewMap(size int) *Map {
	m := make(map[Value]Value, size)
	return &Map{m: m}
}

func (m *Map) String() string { return "TODO(map)" }
func (m *Map) Type() string   { return "map" }
