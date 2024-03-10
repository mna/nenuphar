package machine

import "fmt"

// Increment this to force recompilation of saved bytecode files.
const Version = 0

type Opcode uint8

// "x DUP x x" is a "stack picture" that describes the state of the stack
// before and after execution of the instruction.
//
// OP<index> indicates an immediate operand that is an index into the specified
// table: locals, names, freevars, constants.
const ( //nolint:revive
	NOP Opcode = iota // - NOP -

	// stack operations
	DUP  //   x DUP x x
	DUP2 // x y DUP2 x y x y
	POP  //   x POP -
	EXCH // x y EXCH y x

	// binary comparisons
	// (order must match Token)
	LT
	LE
	GT
	GE
	EQL
	NEQ

	// binary arithmetic
	// (order must match Token)
	PLUS
	MINUS
	STAR
	SLASH
	SLASHSLASH
	CIRCUMFLEX
	PERCENT
	AMPERSAND
	TILDE
	PIPE
	GTGT
	LTLT

	IN

	// unary operators
	//   "try" and "must" compile to a block with a catch, and sets the value
	//   to "nil" for "try", converts the error to a critical one for "must"
	UPLUS  // x UPLUS  x
	UMINUS // x UMINUS -x
	UTILDE // x UTILDE ~x
	NOT    // x NOT    bool
	LEN    // x LEN    #x

	NIL   // - NIL Nil
	TRUE  // - TRUE True
	FALSE // - FALSE False

	ITERPUSH  //       iterable ITERPUSH     -    [pushes the iterator stack]
	ITERPOP   //              - ITERPOP      -    [pops the iterator stack]
	RETURN    //          value RETURN       -
	SETINDEX  //        a i new SETINDEX     -
	INDEX     //            a i INDEX        elem
	SETMAP    //  map key value SETMAP       -
	APPEND    //      list elem APPEND       -
	SLICE     //   x lo hi step SLICE        slice
	MAKEMAP   //              - MAKEMAP      map
	RUNDEFER  //              - RUNDEFER     -      next opcode must run deferred blocks
	DEFEREXIT //              - DEFEREXIT    -      run next deferred block or if no more deferred block to execute, resume

	// --- opcodes with an argument must go below this line ---

	// control flow
	JMP     //            - JMP<addr>     -
	CJMP    //         cond CJMP<addr>    -
	ITERJMP //            - ITERJMP<addr> elem   (and fall through) [acts on topmost iterator]
	//----> // or:        - ITERJMP<addr> -      (and jump)
	CATCHJMP //           - CATCHJMP<addr> -     (jump to addr on catch block exit)

	CONSTANT     //                 - CONSTANT<constant>  value
	MAKETUPLE    //         x1 ... xn MAKETUPLE<n>        tuple
	MAKEARRAY    //         x1 ... xn MAKEARRAY<n>        array
	MAKEFUNC     // defaults+freevars MAKEFUNC<func>      fn
	LOAD         //  from1..fromN mod LOAD<n>             v1 .. vN
	SETLOCAL     //             value SETLOCAL<local>     -
	SETGLOBAL    //             value SETGLOBAL<global>   -
	LOCAL        //                 - LOCAL<local>        value
	FREE         //                 - FREE<freevar>       cell
	FREECELL     //                 - FREECELL<freevar>   value       (content of FREE cell)
	LOCALCELL    //                 - LOCALCELL<local>    value       (content of LOCAL cell)
	SETLOCALCELL //             value SETLOCALCELL<local> -           (set content of LOCAL cell)
	GLOBAL       //                 - GLOBAL<global>      value
	PREDECLARED  //                 - PREDECLARED<name>   value       predeclared = additional bindings made available by the environment, immutable (so unlike globals)
	UNIVERSAL    //                 - UNIVERSAL<name>     value       universe = part of the language, all programs have access to those
	ATTR         //                 x ATTR<name>          y           y = x.name
	SETFIELD     //               x y SETFIELD<name>      -           x.name = y
	UNPACK       //          iterable UNPACK<n>           vn ... v1

	// n>>8 is #positional args and n&0xff is #named args (pairs).
	CALL        // fn positional named                CALL<n>        result
	CALL_VAR    // fn positional named *args          CALL_VAR<n>    result
	CALL_KW     // fn positional named       **kwargs CALL_KW<n>     result TODO: see if kwargs are required
	CALL_VAR_KW // fn positional named *args **kwargs CALL_VAR_KW<n> result TODO: see if kwargs are required

	OpcodeArgMin = JMP
	OpcodeMax    = CALL_VAR_KW
	opcodeJMPMin = JMP
	opcodeJMPMax = CATCHJMP
)

var opcodeNames = [...]string{
	AMPERSAND:    "ampersand",
	APPEND:       "append",
	ATTR:         "attr",
	CALL:         "call",
	CALL_KW:      "call_kw",
	CALL_VAR:     "call_var",
	CALL_VAR_KW:  "call_var_kw",
	CATCHJMP:     "catchjmp",
	CIRCUMFLEX:   "circumflex",
	CJMP:         "cjmp",
	CONSTANT:     "constant",
	DEFEREXIT:    "deferexit",
	DUP2:         "dup2",
	DUP:          "dup",
	EQL:          "eql",
	EXCH:         "exch",
	FALSE:        "false",
	FREE:         "free",
	FREECELL:     "freecell",
	GE:           "ge",
	GLOBAL:       "global",
	GT:           "gt",
	GTGT:         "gtgt",
	IN:           "in",
	INDEX:        "index",
	INPLACE_ADD:  "inplace_add",
	INPLACE_PIPE: "inplace_pipe",
	ITERJMP:      "iterjmp",
	ITERPOP:      "iterpop",
	ITERPUSH:     "iterpush",
	JMP:          "jmp",
	LE:           "le",
	LOAD:         "load",
	LOCAL:        "local",
	LOCALCELL:    "localcell",
	LT:           "lt",
	LTLT:         "ltlt",
	MAKEMAP:      "makemap",
	MAKEFUNC:     "makefunc",
	MAKELIST:     "makelist",
	MAKETUPLE:    "maketuple",
	MANDATORY:    "mandatory",
	MINUS:        "minus",
	NEQ:          "neq",
	NIL:          "nil",
	NOP:          "nop",
	NOT:          "not",
	PERCENT:      "percent",
	PIPE:         "pipe",
	PLUS:         "plus",
	POP:          "pop",
	PREDECLARED:  "predeclared",
	RETURN:       "return",
	RUNDEFER:     "rundefer",
	SETMAP:       "setmap",
	SETDICTUNIQ:  "setdictuniq",
	SETFIELD:     "setfield",
	SETGLOBAL:    "setglobal",
	SETINDEX:     "setindex",
	SETLOCAL:     "setlocal",
	SETLOCALCELL: "setlocalcell",
	SLASH:        "slash",
	SLASHSLASH:   "slashslash",
	SLICE:        "slice",
	STAR:         "star",
	TILDE:        "tilde",
	TRUE:         "true",
	UMINUS:       "uminus",
	UNIVERSAL:    "universal",
	UNPACK:       "unpack",
	UPLUS:        "uplus",
	UTILDE:       "utilde",
}

var reverseLookupOpcode = func() map[string]Opcode {
	m := make(map[string]Opcode, len(opcodeNames))
	for op, s := range opcodeNames {
		m[s] = Opcode(op)
	}
	return m
}()

func isJump(op Opcode) bool {
	// Jump op argument is always encoded with 4 bytes
	return opcodeJMPMin <= op && op <= opcodeJMPMax
}

// returns the number of bytes required to encode the Opcode with its argument
// (if it applies).
func encodedSize(op Opcode, arg uint32) int {
	if op >= OpcodeArgMin {
		if isJump(op) {
			// jumps are always encoded on 4 bytes, padded with NOPs if the jump
			// requires less.
			return 1 + 4
		}
		return 1 + varArgLen(arg)
	}
	return 1
}

// returns the number of bytes required to encode x as a VarInt.
func varArgLen(x uint32) int {
	n := 0
	for x >= 0x80 {
		n++
		x >>= 7
	}
	return n + 1
}

const variableStackEffect = 0x7f

// stackEffect records the effect on the size of the operand stack of
// each kind of instruction. For some instructions this requires computation.
var stackEffect = [...]int8{
	AMPERSAND:    -1,
	APPEND:       -2,
	ATTR:         0,
	CALL:         variableStackEffect,
	CALL_KW:      variableStackEffect,
	CALL_VAR:     variableStackEffect,
	CALL_VAR_KW:  variableStackEffect,
	CATCHJMP:     0,
	CIRCUMFLEX:   -1,
	CJMP:         -1,
	CONSTANT:     +1,
	DEFEREXIT:    0,
	DUP2:         +2,
	DUP:          +1,
	EQL:          -1,
	FALSE:        +1,
	FREE:         +1,
	FREECELL:     +1,
	GE:           -1,
	GLOBAL:       +1,
	GT:           -1,
	GTGT:         -1,
	IN:           -1,
	INDEX:        -1,
	INPLACE_ADD:  -1,
	INPLACE_PIPE: -1,
	ITERJMP:      variableStackEffect,
	ITERPOP:      0,
	ITERPUSH:     -1,
	JMP:          0,
	LE:           -1,
	LOAD:         -1,
	LOCAL:        +1,
	LOCALCELL:    +1,
	LT:           -1,
	LTLT:         -1,
	MAKEMAP:      +1,
	MAKEFUNC:     0,
	MAKELIST:     variableStackEffect,
	MAKETUPLE:    variableStackEffect,
	MANDATORY:    +1,
	MINUS:        -1,
	NEQ:          -1,
	NIL:          +1,
	NOP:          0,
	NOT:          0,
	PERCENT:      -1,
	PIPE:         -1,
	PLUS:         -1,
	POP:          -1,
	PREDECLARED:  +1,
	RETURN:       -1,
	RUNDEFER:     0,
	SETLOCALCELL: -1,
	SETMAP:       -3,
	SETDICTUNIQ:  -3,
	SETFIELD:     -2,
	SETGLOBAL:    -1,
	SETINDEX:     -3,
	SETLOCAL:     -1,
	SLASH:        -1,
	SLASHSLASH:   -1,
	SLICE:        -3,
	STAR:         -1,
	TRUE:         +1,
	UMINUS:       0,
	UNIVERSAL:    +1,
	UNPACK:       variableStackEffect,
	UPLUS:        0,
}

func (op Opcode) String() string {
	if op <= OpcodeMax {
		if name := opcodeNames[op]; name != "" {
			return name
		}
	}
	return fmt.Sprintf("illegal op (%d)", op)
}
