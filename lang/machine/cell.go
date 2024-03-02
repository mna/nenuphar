package machine

import "github.com/mna/nenuphar/lang/types"

// A cell is a box containing a Value. Local variables marked as cells hold
// their value indirectly so that they may be shared by outer and inner nested
// functions. Cells are always accessed using indirect
// {FREE,LOCAL,SETLOCAL}CELL instructions. The FreeVars tuple contains only
// cells. The FREE instruction always yields a cell.
type cell struct{ v types.Value }

func (c *cell) String() string        { return "cell" }
func (c *cell) Type() string          { return "cell" }
func (c *cell) Truth() types.Bool     { panic("unreachable") }
func (c *cell) Hash() (uint32, error) { panic("unreachable") }
func (c *cell) Freeze() {
	if c.v != nil {
		c.v.Freeze()
	}
}
