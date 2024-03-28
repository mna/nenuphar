package scanner

import (
	"fmt"
	"unicode"
)

func (s *Scanner) shortString(opening rune) (lit, decoded string) {
	// '"' / "'" opening already consumed
	startOff, startLine, startCol := s.off-1, s.line, s.col-1

	for {
		cur := s.cur
		if cur == '\n' || cur < 0 {
			s.error(startOff, startLine, startCol, "string literal not terminated")
			break
		}
		s.advance()
		if cur == opening {
			break
		}
		if cur == '\\' {
			s.escape()
		}
	}

	panic("unimplemented")
	return string(s.src[startOff:s.off]), ""
}

// escape parses an escape sequence. In case of a syntax error, it stops at the
// offending character (without consuming it) and returns 0, false. Otherwise
// it returns the first escape character ('0' for a decimal escape) and true.
// It expects the leading backslash to be consumed.
func (s *Scanner) escape(start int) (rune, bool) {
	cur := s.cur
	if s.advanceIf('a', 'b', 'f', 'n', 'r', 't', 'v', 'z', '\\', '/', '"', '\'', '\n') {
		return cur, true
	}

	illegalOrIncomplete := func() {
		pos := s.off
		msg := fmt.Sprintf("illegal character %#U in escape sequence", s.cur)
		if s.cur < 0 {
			msg = "escape sequence not terminated"
			pos = start
		}
		s.error(pos, msg)
	}

	var max, rn uint32
	if isDecimal(s.cur) {
		cur = '0' // return '0' for a decimal escape

		// \ddd - up to 3 decimal digits, to encode a byte
		max = 255
		rn = uint32(digitVal(s.cur))
		s.advance()
		if isDecimal(s.cur) {
			rn = rn*10 + uint32(digitVal(s.cur))
			s.advance()
			if isDecimal(s.cur) {
				rn = rn*10 + uint32(digitVal(s.cur))
				s.advance()
			}
		}

	} else if s.advanceIf('x') {
		// \xhh - exactly 2 hexadecimal digits, to encode a byte
		max = 255
		for i := 0; i < 2; i++ {
			if !isHexadecimal(s.cur) {
				illegalOrIncomplete()
				return 0, false
			}
			rn = rn*16 + uint32(digitVal(s.cur))
			s.advance()
		}

	} else if s.advanceIf('u') {
		max = unicode.MaxRune
		if s.advanceIf('{') {
			// \u{h+} - at least one and up to 8 hexadecimal digits, to encode a Unicode code point

			var count int
			for isHexadecimal(s.cur) {
				rn = rn*16 + uint32(digitVal(s.cur))
				s.advance()
				count++
			}
			if !s.advanceIf('}') {
				illegalOrIncomplete()
				return 0, false
			}
			if count > 8 {
				s.error(start, "escape sequence has too many hexadecimal digits")
				return 0, false
			}
		} else {
			// \uhhhh - exactly 4 hexadecimal digits, compatible with JSON \u escape sequence
			for i := 0; i < 4; i++ {
				if !isHexadecimal(s.cur) {
					illegalOrIncomplete()
					return 0, false
				}
				rn = rn*16 + uint32(digitVal(s.cur))
				s.advance()
			}
		}

	} else {
		msg := "unknown escape sequence"
		if s.cur < 0 {
			msg = "escape sequence not terminated"
		}
		s.error(start, msg)
		return 0, false
	}

	if rn > max || 0xD800 <= rn && rn < 0xE000 {
		msg := "escape sequence is invalid Unicode code point"
		if max == 255 {
			msg = "escape sequence is invalid byte value"
		}
		s.error(start, msg)
		return 0, false
	}
	return cur, true
}
