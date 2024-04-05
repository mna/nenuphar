package parser

import (
	"errors"
	"os"
	"strings"

	"github.com/mna/nenuphar/lang/ast"
	"github.com/mna/nenuphar/lang/scanner"
	"github.com/mna/nenuphar/lang/token"
)

// Mode is a set of bit flags that configures the parsing. By default (0), the
// AST is parsed fully, all errors are reported and comments are ignored.
type Mode uint

// List of supported parsing modes, which can be combined with bitwise or.
const (
	Comments Mode = 1 << iota // parse and report comments, associate them with their AST node.
)

// ParseFiles is a helper function that parses the source files and returns the
// fileset along with the ASTs and any error encountered. The error, if
// non-nil, is guaranteed to be a scanner.ErrorList.
func ParseFiles(mode Mode, files ...string) (*token.FileSet, []*ast.Chunk, error) {
	if len(files) == 0 {
		return nil, nil, nil
	}

	var p parser
	p.parseComments = mode&Comments != 0

	res := make([]*ast.Chunk, 0, len(files))
	fs := token.NewFileSet()

	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			p.errors.Add(token.Position{Filename: file}, err.Error())
			continue
		}

		p.init(fs, file, b)
		ch := p.parseChunk()
		res = append(res, ch)
	}
	return fs, res, p.errors.Err()
}

// ParseChunk is a helper function that parses a single chunk from a slice of
// bytes and returns the AST and any error encountered. The chunk is added to
// the provided fset for position reporting under the name specified in
// filename. The error, if non-nil, is guaranteed to be a scanner.ErrorList.
func ParseChunk(mode Mode, fset *token.FileSet, filename string, src []byte) (*ast.Chunk, error) {
	var p parser
	p.parseComments = mode&Comments != 0
	p.init(fset, filename, src)
	ch := p.parseChunk()
	return ch, p.errors.Err()
}

// TODO: ParseChunkAt to set the initial position? Would allow e.g. REPL line number to be adequate.

// parser parses source files and generates an AST.
type parser struct {
	// those fields are immutable after p.init
	parseComments bool
	scanner       scanner.Scanner
	errors        scanner.ErrorList
	file          *token.File

	// current token
	tok token.Token
	val token.Value

	// this is set in p.advance to the position before skipping any comment,
	// which is then used to set the starting position of blocks, so that blocks
	// always encompass the comments.
	preCommentPos token.Pos

	// this field is only used when parseComments is true, pending comments are
	// those skipped over by p.advance, stored here until they are processed
	// post-parse.
	pendingComments []*ast.Comment
}

func (p *parser) init(fset *token.FileSet, filename string, src []byte) {
	p.file = fset.AddFile(filename, -1, len(src))
	p.scanner.Init(p.file, src, p.errors.Add)
	p.pendingComments = nil

	// advance to first token
	p.advance()
}

func (p *parser) advance() {
	p.tok = p.scanner.Scan(&p.val)
	p.preCommentPos = p.val.Pos
	for p.tok == token.COMMENT {
		if p.parseComments {
			p.pendingComments = append(p.pendingComments, &ast.Comment{
				Start: p.val.Pos,
				Raw:   p.val.Raw,
				Val:   p.val.String,
			})
		}
		p.tok = p.scanner.Scan(&p.val)
	}
}

var errPanicMode = errors.New("panic")

// expect returns the position of the current token and consumes it if it is
// one of the expected tokens, otherwise it reports an error and panics with
// errPanicMode which gets recovered at the statement level, resulting in a
// BadStmt.
func (p *parser) expect(toks ...token.Token) token.Pos {
	pos := p.val.Pos

	var buf strings.Builder
	var ok bool
	for i, tok := range toks {
		if p.tok == tok {
			ok = true
			break
		}
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(tok.GoString())
	}

	if !ok {
		var lbl string
		if len(toks) > 1 {
			lbl = "one of " + buf.String()
		} else {
			lbl = buf.String()
		}
		p.errorExpected(pos, lbl)
		panic(errPanicMode)
	}

	p.advance()
	return pos
}

func (p *parser) error(pos token.Pos, msg string) {
	lpos := p.file.Position(pos)
	p.errors.Add(lpos, msg)
}

func (p *parser) errorExpected(pos token.Pos, msg string) {
	msg = "expected " + msg
	if pos == p.val.Pos {
		// the error happened at the current position;
		// make the error message more specific
		switch lit := p.tok.Literal(p.val); lit {
		case "":
			msg += ", found " + p.tok.GoString()
		default:
			// print 123 rather than 'INT', etc.
			msg += ", found " + lit
		}
	}
	p.error(pos, msg)
}
