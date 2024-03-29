package token

import (
	"fmt"
	"testing"
)

type startEnd struct {
	s, e Pos
}

func (se startEnd) Span() (start, end Pos) { return se.s, se.e }

func TestPosInside(t *testing.T) {
	cases := []struct {
		ref, test startEnd
		want      bool
	}{
		{startEnd{1, 2}, startEnd{3, 4}, false},
		{startEnd{1, 3}, startEnd{3, 4}, false},
		{startEnd{1, 4}, startEnd{3, 4}, true},
		{startEnd{2, 4}, startEnd{3, 4}, true},
		{startEnd{3, 4}, startEnd{3, 4}, true},
		{startEnd{4, 5}, startEnd{3, 4}, false},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%v-%v", c.ref, c.test), func(t *testing.T) {
			got := PosInside(c.ref, c.test)
			if c.want != got {
				t.Errorf("want %t, got %t", c.want, got)
			}
		})
	}
}

func TestPosAdjacent(t *testing.T) {
	fset := NewFileSet()
	f := fset.AddFile("test", -1, 10)
	f.AddLine(3) // note that those newlines are in raw byte offsets;
	f.AddLine(5) // translated to Pos values, you must + 1!
	f.AddLine(8)

	// In Pos values:
	// | 1  2  3  4  5  6  7  8  9  10  11 |
	//   _  _  _  \n _  \n _  _  \n _   EOF

	cases := []struct {
		ref, test startEnd
		want      bool
	}{
		// both empty, ref before test
		{startEnd{1, 1}, startEnd{1, 1}, true},
		{startEnd{1, 1}, startEnd{2, 2}, true},
		{startEnd{1, 1}, startEnd{3, 3}, true},
		{startEnd{1, 1}, startEnd{4, 4}, false}, // test is on next line
		{startEnd{4, 4}, startEnd{4, 4}, true},
		{startEnd{4, 4}, startEnd{5, 5}, true},
		{startEnd{4, 4}, startEnd{6, 6}, false}, // test is on next line
		{startEnd{9, 9}, startEnd{9, 9}, true},
		{startEnd{9, 9}, startEnd{10, 10}, true},
		{startEnd{9, 9}, startEnd{11, 11}, true},

		// both empty, ref after test
		{startEnd{2, 2}, startEnd{1, 1}, true},
		{startEnd{3, 3}, startEnd{1, 1}, true},
		{startEnd{4, 4}, startEnd{1, 1}, true},
		{startEnd{5, 5}, startEnd{1, 1}, true},
		{startEnd{6, 6}, startEnd{1, 1}, false}, // 2 lines after
		{startEnd{6, 6}, startEnd{2, 2}, false}, // 2 lines after
		{startEnd{6, 6}, startEnd{3, 3}, false}, // 2 lines after
		{startEnd{6, 6}, startEnd{4, 4}, true},
		{startEnd{7, 7}, startEnd{4, 4}, true},
		{startEnd{8, 8}, startEnd{4, 4}, true},
		{startEnd{9, 9}, startEnd{4, 4}, false},   // 2 lines after
		{startEnd{10, 10}, startEnd{4, 4}, false}, // 2 lines after
		{startEnd{11, 11}, startEnd{4, 4}, false}, // 2 lines after
		{startEnd{9, 9}, startEnd{6, 6}, true},
		{startEnd{10, 10}, startEnd{6, 6}, true},
		{startEnd{11, 11}, startEnd{6, 6}, true},

		// non-empty
		{startEnd{4, 8}, startEnd{1, 2}, true},  // same line
		{startEnd{5, 8}, startEnd{1, 2}, true},  // next line
		{startEnd{1, 3}, startEnd{3, 4}, true},  // same line
		{startEnd{1, 3}, startEnd{5, 7}, false}, // after, not same line
		{startEnd{2, 8}, startEnd{4, 6}, true},  // within
		{startEnd{2, 8}, startEnd{1, 3}, true},  // partly within
		{startEnd{2, 8}, startEnd{6, 10}, true}, // partly within
		{startEnd{3, 4}, startEnd{1, 2}, true},  // same line
		{startEnd{5, 6}, startEnd{1, 2}, true},  // next line
		{startEnd{7, 9}, startEnd{1, 2}, false}, // two lines after
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%v-%v", c.ref, c.test), func(t *testing.T) {
			got := PosAdjacent(c.ref, c.test, f)
			if c.want != got {
				t.Errorf("want %t, got %t", c.want, got)
			}
		})
	}
}

func TestFormatPos(t *testing.T) {
	fset := NewFileSet()
	f0 := fset.AddFile("test", -1, 10)
	f1 := fset.AddFile("test_next", -1, 10)

	cases := []struct {
		pos  Pos
		mode PosMode
		file *File
		want string
	}{
		{NoPos, PosLong, f0, "test:-:-"},
		{NoPos, PosOffsets, f0, "-"},
		{NoPos, PosRaw, f0, "0"},
		{NoPos, PosNone, f0, ""},
		{1, PosLong, f0, "test:1:1"},
		{1, PosOffsets, f0, "0"},
		{1, PosRaw, f0, "1"},
		{1, PosNone, f0, ""},
		{2, PosLong, f0, "test:1:2"},
		{2, PosOffsets, f0, "1"},
		{2, PosRaw, f0, "2"},
		{2, PosNone, f0, ""},
		{10, PosLong, f0, "test:1:10"},
		{10, PosOffsets, f0, "9"},
		{10, PosRaw, f0, "10"},
		{10, PosNone, f0, ""},
		{11, PosLong, f0, "test:1:11"},
		{11, PosOffsets, f0, "10"},
		{11, PosRaw, f0, "11"},
		{11, PosNone, f0, ""},
		{12, PosLong, f1, "test_next:1:1"},
		{12, PosOffsets, f1, "0"},
		{12, PosRaw, f1, "12"},
		{12, PosNone, f1, ""},
		{13, PosLong, f1, "test_next:1:2"},
		{13, PosOffsets, f1, "1"},
		{13, PosRaw, f1, "13"},
		{13, PosNone, f1, ""},
		{-14, PosLong, f1, ":1:3"},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%d:%s", c.pos, c.mode), func(t *testing.T) {
			// negative pos means to set filename to false
			pos := c.pos
			fname := true
			if pos < 0 {
				pos = -pos
				fname = false
			}
			got := FormatPos(c.mode, c.file, pos, fname)
			if got != c.want {
				t.Errorf("want %q, got %q", c.want, got)
			}
		})
	}
}
