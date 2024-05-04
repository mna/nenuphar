package machine_test

//import (
//	"context"
//	"fmt"
//	"os"
//	"path/filepath"
//	"regexp"
//	"strconv"
//	"strings"
//	"testing"
//
//	"github.com/mna/nenuphar/lang/compiler"
//	"github.com/mna/nenuphar/lang/machine"
//	"github.com/stretchr/testify/assert"
//	"github.com/stretchr/testify/require"
//)
//
//var rxAssertGlobal = regexp.MustCompile(`(?m)^\s*###\s*([a-zA-Z][a-zA-Z0-9_]*):\s*(.+)$`)
//
//// TestExecAsm loads the assembly files in testdata/asm/*.asm and runs the resulting program.
//// Expected results are provided as comments in the asm file in the form of:
////   - ### fail: <error message>
////   - ### global_name: <value>
////   - ### nofail: <value>
////
//// Values can be 'nil', a number, a quoted string or 'true' and 'false'. Global
//// names are provided and retrieved in a predeclared 'G' map, and are nil by
//// default.
////
//// It is possible to combine those expected results, nofail: is the default if
//// neither fail nor nofail is specified. The nofail value is the value returned
//// by the program.
//func TestExecAsm(t *testing.T) {
//	dir := filepath.Join("testdata", "asm")
//	des, err := os.ReadDir(dir)
//	require.NoError(t, err)
//
//	for _, de := range des {
//		if de.IsDir() || !de.Type().IsRegular() || filepath.Ext(de.Name()) != ".asm" {
//			continue
//		}
//		t.Run(de.Name(), func(t *testing.T) {
//			filename := filepath.Join(dir, de.Name())
//			b, err := os.ReadFile(filename)
//			require.NoError(t, err)
//
//			cprog, err := compiler.Asm(b)
//			require.NoError(t, err)
//
//			var thread machine.Thread
//			gmap := machine.NewMap(0)
//			thread.Predeclared = map[string]machine.Value{"G": gmap}
//			res, err := thread.RunProgram(context.Background(), cprog)
//
//			ms := rxAssertGlobal.FindAllStringSubmatch(string(b), -1)
//			require.NotNil(t, ms, "no assertion provided")
//			var errAsserted bool
//			for _, m := range ms {
//				want := strings.TrimSpace(m[2])
//				switch global := m[1]; global {
//				case "fail":
//					errAsserted = true
//					assert.ErrorContains(t, err, want, "result: %v", res)
//				case "nofail":
//					errAsserted = true
//					if assert.NoError(t, err, "result: %v", res) {
//						assertValue(t, "", want, res)
//					}
//				default:
//					// assert the provided global
//					gval, _, _ := gmap.Get(machine.String(global))
//					if assert.NotNil(t, gval, "global %s does not exist", global) {
//						assertValue(t, global, want, gval)
//					}
//				}
//			}
//			if !errAsserted {
//				// default to no error expected
//				require.NoError(t, err)
//			}
//		})
//	}
//}
//
//func assertValue(t *testing.T, name, want string, got machine.Value) bool {
//	msg := "result"
//	if name != "" {
//		msg = fmt.Sprintf("global %s", name)
//	}
//	if want == "nil" {
//		return assert.Equal(t, machine.Nil, got, msg)
//	} else if want == "true" || want == "false" {
//		wantVal := machine.True
//		if want != "true" {
//			wantVal = machine.False
//		}
//		return assert.Equal(t, wantVal, got, msg)
//	} else if qs, err := strconv.Unquote(want); err == nil {
//		got, ok := machine.AsString(got)
//		if assert.True(t, ok, msg) {
//			return assert.Equal(t, qs, got, msg)
//		}
//	} else if n, err := strconv.ParseInt(want, 10, 64); err == nil {
//		got, err := machine.AsExactInt(got)
//		if assert.NoError(t, err, msg) {
//			return assert.Equal(t, n, int64(got), msg)
//		}
//	} else {
//		return assert.Failf(t, "unexpected result", "%s: want %s, got %v (%[2]T)", msg, want, got)
//	}
//	return false
//}
