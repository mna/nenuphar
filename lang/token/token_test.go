package token

import (
	"testing"
)

func TestTokenString(t *testing.T) {
	for tok := Token(0); tok < maxToken; tok++ {
		if tok.String() == "" {
			t.Errorf("missing string representation of token %d", tok)
		}
	}
}
