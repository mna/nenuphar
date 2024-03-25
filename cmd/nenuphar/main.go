package main

import (
	"os"

	"github.com/mna/mainer"
	"github.com/mna/nenuphar/internal/maincmd"
)

var (
	// placeholder values, replaced on build
	version   = "{v}" // must be N.N[.N]
	buildDate = "{d}" // must be YYYY-mm-DD
)

func main() {
	c := maincmd.Cmd{BuildVersion: version, BuildDate: buildDate}
	os.Exit(int(c.Main(os.Args, mainer.CurrentStdio())))
}
