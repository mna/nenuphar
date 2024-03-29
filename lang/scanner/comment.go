package scanner

func (s *Scanner) comment() (lit, val string) {
	// '--' opening already consumed, hence the -2
	startOff := s.off - 2

	// this is a long comment only if there is a valid opening long bracket
	// sequence.
	var level int
	if s.advanceIf('[') {
		for s.advanceIf('=') {
			level++
		}
		if !s.advanceIf('[') {
			// this was not a long comment opening
			level = -1
		}
		if level >= 0 {
			return s.longComment(level)
		}
	}

	for s.cur != '\n' && s.cur != -1 {
		s.advance()
	}
	return string(s.src[startOff:s.off]), string(s.src[startOff+2 : s.off])
}

func (s *Scanner) longComment(level int) (lit, val string) {
	// '--[=[' opening already consumed, hence the -(4+level)
	startOff, startLine, startCol := s.off-(4+level), s.line, s.col-(4+level)
	s.sb.Reset()

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
			if closeLevel > -1 /* valid close sequence */ && closeLevel == level /* matching opening level */ {
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
		s.error(startOff, startLine, startCol, "long comment not terminated")
	}
	return string(s.src[startOff:s.off]), s.sb.String()
}
