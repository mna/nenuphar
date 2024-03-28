package scanner

import (
	"fmt"
	"unicode"
)

func (s *Scanner) shortString(opening rune) (lit, decoded string) {
	// '"' / "'" opening already consumed, hence the -1
	startOff, startLine, startCol := s.off-1, s.line, s.col-1
	s.sb.Reset()

	var skipws bool
	for {
		cur := s.cur
		if (cur == '\n' && !skipws) || cur < 0 {
			s.error(startOff, startLine, startCol, "string literal not terminated")
			break
		}
		s.advance()
		if cur == opening {
			break
		}
		if cur == '\\' {
			skipws = s.escape()
		} else if !skipws || !isWhitespace(cur) {
			skipws = false
			s.sb.WriteRune(cur)
		}
	}
	return string(s.src[startOff:s.off]), s.sb.String()
}

var simpleEscapes = [...]byte{
	'a':  '\a',
	'b':  '\b',
	'f':  '\f',
	'n':  '\n',
	'r':  '\r',
	't':  '\t',
	'v':  '\v',
	'\\': '\\',
	'/':  '/',
	'\'': '\'',
	'"':  '"',
	'\n': '\n',
}

// escape parses an escape sequence. In case of a syntax error, it stops at
// the offending character (without consuming it). Otherwise it consumes and
// writes the value of the escape sequence. It expects the leading backslash
// to be consumed. If the escape is \z, it returns true for skipws, indicating
// the the following whitespace characters in the string literal should be
// skipped for the string value.
func (s *Scanner) escape() (skipws bool) {
	// initial backslash already consumed, hence the -1
	startOff, startLine, startCol := s.off-1, s.line, s.col-1

	if cur := s.cur; s.advanceIf('a', 'b', 'f', 'n', 'r', 't', 'v', 'z', '\\', '/', '"', '\'', '\n') {
		if cur != 'z' {
			s.sb.WriteByte(simpleEscapes[cur])
		}
		return cur == 'z'
	}

	// emits an error either at the current char (if it is invalid as part of
	// this escape sequence) or at the start of the escape sequence (if the
	// error applies to the escape as a whole).
	illegalOrIncomplete := func() {
		pos, line, col := s.off, s.line, s.col
		msg := fmt.Sprintf("illegal character %#U in escape sequence", s.cur)
		if s.cur < 0 {
			msg = "escape sequence not terminated"
			pos, line, col = startOff, startLine, startCol
		}
		s.error(pos, line, col, msg)
	}

	var max, rn uint32
	if isDecimal(s.cur) {
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
				return false
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
				return false
			}
			if count > 8 {
				s.error(startOff, startLine, startCol, "escape sequence has too many hexadecimal digits")
				return false
			}
		} else {
			// \uhhhh - exactly 4 hexadecimal digits, compatible with JSON \u escape sequence
			for i := 0; i < 4; i++ {
				if !isHexadecimal(s.cur) {
					illegalOrIncomplete()
					return false
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
		s.error(startOff, startLine, startCol, msg)
		return false
	}

	if rn > max || 0xD800 <= rn && rn < 0xE000 {
		// TODO: if surrogate, try to decode following rune otherwise replace with invalid rune
		msg := "escape sequence is invalid Unicode code point"
		if max == 255 {
			msg = "escape sequence is invalid byte value"
		}
		s.error(startOff, startLine, startCol, msg)
		return false
	}
	s.sb.WriteRune(rune(rn))
	return false
}

func digitVal(rn rune) int {
	switch {
	case '0' <= rn && rn <= '9':
		return int(rn - '0')
	case 'a' <= rn && rn <= 'f':
		return int(rn - 'a' + 10)
	case 'A' <= rn && rn <= 'F':
		return int(rn - 'A' + 10)
	}
	return 16 // larger than any legal digit val
}
