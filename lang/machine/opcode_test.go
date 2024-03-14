package machine

import (
	"strings"
	"testing"
)

func TestOpcodeString(t *testing.T) {
	for op := Opcode(0); op <= OpcodeMax; op++ {
		if opcodeNames[op] == "" {
			t.Errorf("missing string representation of opcode %d", op)
		}
		if s := op.String(); strings.Contains(s, "illegal") {
			t.Errorf("invalid string representation of opcode %d", op)
		}
	}
}
