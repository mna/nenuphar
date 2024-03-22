package machine

// A cell is a box containing a Value. Local variables marked as cells hold
// their value indirectly so that they may be shared by outer and inner nested
// functions. Cells are always accessed using indirect
// {FREE,LOCAL,SETLOCAL}CELL instructions. The FreeVars tuple contains only
// cells. The FREE instruction always yields a cell.
type cell struct{ v Value }

var _ Value = (*cell)(nil)

func (c *cell) String() string { return "cell" }
func (c *cell) Type() string   { return "cell" }
