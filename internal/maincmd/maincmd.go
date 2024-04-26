package maincmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/mna/mainer"
)

const binName = "nenuphar"

var (
	shortUsage = fmt.Sprintf(`
usage: %s [<option>...] <command> [<path>...]
Run '%[1]s --help' for details.
`, binName)

	longUsage = fmt.Sprintf(`usage: %s [<option>...] <command> [<path>...] [-- <arg>...]
       %[1]s -h|--help
       %[1]s -v|--version

Compiler and all-in-one tool for the %[1]s programming language.

The <command> can be one of:
       parse                     Execute the parser phase of the
                                 compilation and print the resulting
                                 abstract syntax tree (AST).
       resolve                   Execute the resolver phase of the
                                 compilation and print the resulting
                                 abstract syntax tree (AST) with symbol
                                 resolution information.
       tokenize                  Execute the scanner phase of the
                                 compilation and print the resulting
                                 tokens.

Valid flag options are:
       -h --help                 Show this help and exit.
       -v --version              Print version and exit.

Valid flag options for the <parse> command are:
       --with-comments           Include comments in the AST (excluded
                                 by default).

More information on the %[1]s repository:
       https://github.com/mna/nenuphar
`, binName)
)

type Cmd struct {
	BuildVersion string
	BuildDate    string

	Help    bool `flag:"h,help"`
	Version bool `flag:"v,version"`

	WithComments bool `flag:"with-comments"`

	args  []string
	flags map[string]bool
	cmdFn func(context.Context, mainer.Stdio, []string) error
}

func (c *Cmd) SetArgs(args []string) {
	c.args = args
}

func (c *Cmd) SetFlags(flags map[string]bool) {
	c.flags = flags
}

func (c *Cmd) Validate() error {
	if c.Help || c.Version {
		return nil
	}

	if len(c.args) == 0 {
		return errors.New("no command specified")
	}

	cmdName := c.args[0]

	commands := buildCmds(c)
	c.cmdFn = commands[cmdName]
	if c.cmdFn == nil {
		return fmt.Errorf("unknown command: %s", c.args[0])
	}

	if cmdName == "tokenize" || cmdName == "parse" {
		// at least one file is required, or TODO: read from stdin
		if len(c.args[1:]) == 0 {
			return fmt.Errorf("%s: at least one file must be provided", cmdName)
		}
	}

	if c.flags["with-comments"] && cmdName != "parse" && cmdName != "resolve" {
		return fmt.Errorf("%s: invalid flag 'with-comments'", cmdName)
	}

	return nil
}

func printError(stdio mainer.Stdio, err error) error {
	if err != nil {
		fmt.Fprintf(stdio.Stderr, "%s\n", err)
	}
	return err
}

func (c *Cmd) Main(args []string, stdio mainer.Stdio) mainer.ExitCode {
	p := mainer.Parser{
		EnvVars:   false, // leaving this here for now in case some flags can use this
		EnvPrefix: binName + "_",
	}
	if err := p.Parse(args, c); err != nil {
		fmt.Fprintf(stdio.Stderr, "invalid arguments: %s\n%s", err, shortUsage)
		return mainer.InvalidArgs
	}

	switch {
	case c.Help:
		fmt.Fprint(stdio.Stdout, longUsage)
		return mainer.Success

	case c.Version:
		fmt.Fprintf(stdio.Stdout, "%s %s %s\n", binName, c.BuildVersion, c.BuildDate)
		return mainer.Success
	}

	ctx := mainer.CancelOnSignal(context.Background(), os.Interrupt)
	if err := c.cmdFn(ctx, stdio, c.args[1:]); err != nil {
		// each command takes care of printing its errors, just return with an error code
		return mainer.Failure
	}
	return mainer.Success
}

// valid commands are those that take a mainer.Stdio and a slice of strings as
// input, and return an error as output.
func buildCmds(v interface{}) map[string]func(context.Context, mainer.Stdio, []string) error {
	cmds := make(map[string]func(context.Context, mainer.Stdio, []string) error)

	vv := reflect.ValueOf(v)
	vt := vv.Type()
	for i := 0; i < vt.NumMethod(); i++ {
		m := vt.Method(i)
		mt := m.Type

		// must take 4 parameters (including receiver) and return 1
		if mt.NumIn() != 4 || mt.NumOut() != 1 {
			continue
		}

		if rt := mt.Out(0); rt.Kind() != reflect.Interface || rt.Name() != "error" {
			continue
		}
		if p0 := mt.In(0); p0.Kind() != reflect.Ptr || p0.Elem().Name() != "Cmd" {
			continue
		}
		if p1 := mt.In(1); p1.Kind() != reflect.Interface || p1.Name() != "Context" {
			continue
		}
		if p2 := mt.In(2); p2.Kind() != reflect.Struct || p2.Name() != "Stdio" {
			continue
		}
		if p3 := mt.In(3); p3.Kind() != reflect.Slice || p3.Elem().Name() != "string" {
			continue
		}
		cmds[strings.ToLower(m.Name)] = vv.Method(i).Interface().(func(context.Context, mainer.Stdio, []string) error)
	}
	return cmds
}
