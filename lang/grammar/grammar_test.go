package grammar

import (
	"os"
	"testing"

	"golang.org/x/exp/ebnf"
)

func TestEBNF(t *testing.T) {
	files := []string{
		"grammar.ebnf",
		"grammar_lua.ebnf",
	}
	for _, filename := range files {
		t.Run(filename, func(t *testing.T) {
			f, err := os.Open(filename)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			g, err := ebnf.Parse(filename, f)
			if err != nil {
				t.Fatal(err)
			}
			if err := ebnf.Verify(g, "Chunk"); err != nil {
				t.Fatal(err)
			}
		})
	}
}
