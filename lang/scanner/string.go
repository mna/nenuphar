package scanner

import (
	"fmt"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

func (s *Scanner) longString() (lit, decoded string) {
	// '[' opening already consumed, hence the -1
	startOff, startLine, startCol := s.off-1, s.line, s.col-1
	s.sb.Reset()

	var level int
	for s.advanceIf('=') {
		level++
	}
	if !s.advanceIf('[') {
		s.error(startOff, startLine, startCol, "invalid long string literal opening sequence")
		return string(s.src[startOff:s.off]), ""
	}

	closeLevel := -1
	closeStartOff := 0
	for s.cur != -1 {
		if s.advanceIf(']') {
			// maybe a closing sequence, keep start index in case it ends up not being it
			closeStartOff = s.off - 1 // -1 since we're past the initial ']' now

			// calculate the close level
			closeLevel = 0
			for s.advanceIf('=') {
				closeLevel++
			}
			if !s.advanceIf(']') {
				closeLevel = -1
			}
			if closeLevel > -1 /* a valid close sequence */ && closeLevel == level /* matching the opening level */ {
				break
			}
			closeLevel = -1
			s.sb.Write(s.src[closeStartOff:s.off])
			continue
		}
		s.sb.WriteRune(s.cur)
		s.advance()
	}

	if closeLevel == -1 {
		s.error(startOff, startLine, startCol, "long string literal not terminated")
	}
	return string(s.src[startOff:s.off]), s.sb.String()
}

func (s *Scanner) shortString(opening rune) (lit, decoded string) {
	// '"' / "'" opening already consumed, hence the -1
	startOff, startLine, startCol := s.off-1, s.line, s.col-1
	s.sb.Reset()
	s.pendingSurrogate = 0

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
			s.writeStringLitRune(cur)
		}
	}
	if s.pendingSurrogate != 0 {
		s.sb.WriteRune(utf8.RuneError)
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
			s.writeStringLitRune(rune(simpleEscapes[cur]))
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

	if rn > max {
		msg := "escape sequence is invalid Unicode code point"
		if max == 255 {
			msg = "escape sequence is invalid byte value"
		}
		s.error(startOff, startLine, startCol, msg)
		return false
	}
	if utf16.IsSurrogate(rune(rn)) {
		s.writeStringLitSurrogate(rune(rn))
		return false
	}
	s.writeStringLitRune(rune(rn))
	return false
}

// writes a rune that is _not_ a surrogate
func (s *Scanner) writeStringLitRune(rn rune) {
	if s.pendingSurrogate != 0 {
		s.sb.WriteRune(utf8.RuneError)
		s.pendingSurrogate = 0
	}
	s.sb.WriteRune(rn)
}

// writes a rune that is a surrogate (could be first or second half)
func (s *Scanner) writeStringLitSurrogate(rn rune) {
	if s.pendingSurrogate == 0 {
		s.pendingSurrogate = rn
	} else {
		s.sb.WriteRune(utf16.DecodeRune(s.pendingSurrogate, rn))
		s.pendingSurrogate = 0
	}
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
