package compiler

type insn struct {
	op        Opcode
	arg       uint32
	line, col int32
}

func encodeInsn(code []byte, op Opcode, arg uint32) []byte {
	code = append(code, byte(op))
	if op >= OpcodeArgMin {
		if isJump(op) {
			code = addUint32(code, arg, 4) // pad arg to 4 bytes
		} else {
			code = addUint32(code, arg, 0)
		}
	}
	return code
}

// addUint32 encodes x as 7-bit little-endian varint.
func addUint32(code []byte, x uint32, min int) []byte {
	end := len(code) + min
	for x >= 0x80 {
		code = append(code, byte(x)|0x80)
		x >>= 7
	}
	code = append(code, byte(x))
	// Pad the operand with NOPs to exactly min bytes.
	for len(code) < end {
		code = append(code, byte(NOP))
	}
	return code
}
