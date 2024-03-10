package token

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPos(t *testing.T) {
	for l := 1; l <= MaxLines; l++ {
		for c := 1; c <= MaxCols; c++ {
			p := MakePos(l, c)
			if p.Unknown() {
				t.Fatalf("p.Unknown() true for %d, %d", l, c)
			}
			gotl, gotc := p.LineCol()
			if gotl != l || gotc != c {
				t.Fatalf("p.LineCol() returned %d, %d for %d, %d", gotl, gotc, l, c)
			}
		}
	}
}

func TestPosUnknown(t *testing.T) {
	p := MakePos(0, 0)
	require.True(t, p.Unknown())
	p = MakePos(1, 0)
	require.True(t, p.Unknown())
	p = MakePos(0, 1)
	require.True(t, p.Unknown())
	p = MakePos(1, 1)
	require.False(t, p.Unknown())
}
