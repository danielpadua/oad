// Package parser implements a hand-rolled subset of the SCIM 2.0 filter
// grammar (RFC 7644 §3.4.2.2). The supported subset is enough for the
// LIST endpoints OAD exposes:
//
//	filter      = orExpr
//	orExpr      = andExpr ( "or" andExpr )*
//	andExpr     = atomExpr ( "and" atomExpr )*
//	atomExpr    = "(" filter ")" | comparison
//	comparison  = attrPath ( "pr" | compareOp value )
//	attrPath    = ident ( "." ident )?
//	compareOp   = "eq" / "ne" / "co" / "sw" / "ew"
//	value       = string / number / boolean / null
//
// Operators not in this subset (gt/ge/lt/le, "not", complex paths with
// embedded filters) return InvalidFilterError.
package parser

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// InvalidFilterError indicates a filter that the parser could not parse.
// The Pos field is the byte offset within the input where the error was
// detected; useful for client-side highlighting.
type InvalidFilterError struct {
	Pos int
	Msg string
}

func (e *InvalidFilterError) Error() string {
	return fmt.Sprintf("invalid filter at position %d: %s", e.Pos, e.Msg)
}

// Comparison and logical operators. Stored as lowercase strings so the
// SQL translator can switch on them directly.
const (
	OpEq = "eq"
	OpNe = "ne"
	OpCo = "co"
	OpSw = "sw"
	OpEw = "ew"
	OpPr = "pr"

	LogicAnd = "and"
	LogicOr  = "or"
)

// Expr is the root AST node interface.
type Expr interface{ exprNode() }

// AtomExpr represents a single comparison or presence check.
// For Op == OpPr, Value is nil.
type AtomExpr struct {
	Attr  string
	Op    string
	Value any // string | float64 | bool | nil (for null or pr)
}

// LogicExpr combines two sub-expressions with "and" or "or".
type LogicExpr struct {
	Op    string // LogicAnd or LogicOr
	Left  Expr
	Right Expr
}

func (*AtomExpr) exprNode()  {}
func (*LogicExpr) exprNode() {}

// Parse parses input as a SCIM filter expression and returns the AST.
// Whitespace is skipped between tokens. Returns an *InvalidFilterError on
// any grammar or lexical violation.
func Parse(input string) (Expr, error) {
	p := &parser{l: newLexer(input)}
	if err := p.l.next(); err != nil {
		return nil, err
	}
	expr, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.l.tok.kind != tokEOF {
		return nil, &InvalidFilterError{Pos: p.l.tokPos, Msg: "unexpected trailing tokens"}
	}
	return expr, nil
}

// ─── Lexer ──────────────────────────────────────────────────────────────────

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokIdent
	tokString
	tokNumber
	tokTrue
	tokFalse
	tokNull
	tokLParen
	tokRParen
	tokOp    // eq/ne/co/sw/ew
	tokPr    // pr (unary)
	tokLogic // and/or
	tokDot   // sub-attribute separator
)

type token struct {
	kind  tokenKind
	value string
}

type lexer struct {
	src    []rune
	pos    int
	tok    token
	tokPos int
}

func newLexer(input string) *lexer { return &lexer{src: []rune(input)} }

// next advances the lexer to the next token (stored in l.tok). EOF is
// represented by a tok of kind tokEOF.
func (l *lexer) next() error {
	l.skipWS()
	l.tokPos = l.pos
	if l.pos >= len(l.src) {
		l.tok = token{kind: tokEOF}
		return nil
	}
	c := l.src[l.pos]
	switch {
	case c == '(':
		l.tok = token{kind: tokLParen, value: "("}
		l.pos++
	case c == ')':
		l.tok = token{kind: tokRParen, value: ")"}
		l.pos++
	case c == '.':
		l.tok = token{kind: tokDot, value: "."}
		l.pos++
	case c == '"':
		s, err := l.readString()
		if err != nil {
			return err
		}
		l.tok = token{kind: tokString, value: s}
	case isLetter(c):
		l.tok = l.readIdentOrKeyword()
	case isDigit(c) || c == '-':
		s, err := l.readNumber()
		if err != nil {
			return err
		}
		l.tok = token{kind: tokNumber, value: s}
	default:
		return &InvalidFilterError{Pos: l.pos, Msg: fmt.Sprintf("unexpected character %q", c)}
	}
	return nil
}

func (l *lexer) skipWS() {
	for l.pos < len(l.src) && unicode.IsSpace(l.src[l.pos]) {
		l.pos++
	}
}

// readString reads a double-quoted string with backslash escapes \"  \\  \/
// \b \f \n \r \t \uXXXX (per JSON / RFC 7159).
func (l *lexer) readString() (string, error) {
	start := l.pos
	l.pos++ // consume opening "
	var sb strings.Builder
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c == '"' {
			l.pos++
			return sb.String(), nil
		}
		if c == '\\' {
			l.pos++
			if l.pos >= len(l.src) {
				return "", &InvalidFilterError{Pos: l.pos, Msg: "unterminated escape"}
			}
			esc, err := l.unescape(l.src[l.pos])
			if err != nil {
				return "", err
			}
			sb.WriteRune(esc)
			l.pos++
			continue
		}
		sb.WriteRune(c)
		l.pos++
	}
	return "", &InvalidFilterError{Pos: start, Msg: "unterminated string"}
}

func (l *lexer) unescape(c rune) (rune, error) {
	switch c {
	case '"', '\\', '/':
		return c, nil
	case 'b':
		return '\b', nil
	case 'f':
		return '\f', nil
	case 'n':
		return '\n', nil
	case 'r':
		return '\r', nil
	case 't':
		return '\t', nil
	}
	return 0, &InvalidFilterError{Pos: l.pos, Msg: fmt.Sprintf("invalid escape \\%c", c)}
}

func (l *lexer) readNumber() (string, error) {
	start := l.pos
	if l.src[l.pos] == '-' {
		l.pos++
	}
	for l.pos < len(l.src) && (isDigit(l.src[l.pos]) || l.src[l.pos] == '.') {
		l.pos++
	}
	if l.pos == start || (l.pos == start+1 && l.src[start] == '-') {
		return "", &InvalidFilterError{Pos: start, Msg: "invalid number"}
	}
	return string(l.src[start:l.pos]), nil
}

// readIdentOrKeyword reads an identifier and decides whether it's a literal
// (true/false/null), an operator (eq/ne/co/sw/ew/pr) or a logic keyword
// (and/or). Comparison is case-insensitive per RFC 7644 §3.4.2.2.
func (l *lexer) readIdentOrKeyword() token {
	start := l.pos
	for l.pos < len(l.src) && (isLetter(l.src[l.pos]) || isDigit(l.src[l.pos]) || l.src[l.pos] == '_') {
		l.pos++
	}
	raw := string(l.src[start:l.pos])
	lower := strings.ToLower(raw)
	switch lower {
	case "true":
		return token{kind: tokTrue, value: lower}
	case "false":
		return token{kind: tokFalse, value: lower}
	case "null":
		return token{kind: tokNull, value: lower}
	case OpEq, OpNe, OpCo, OpSw, OpEw:
		return token{kind: tokOp, value: lower}
	case OpPr:
		return token{kind: tokPr, value: lower}
	case LogicAnd, LogicOr:
		return token{kind: tokLogic, value: lower}
	}
	return token{kind: tokIdent, value: raw}
}

func isLetter(r rune) bool { return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' }
func isDigit(r rune) bool  { return r >= '0' && r <= '9' }

// ─── Parser ────────────────────────────────────────────────────────────────

type parser struct {
	l *lexer
}

func (p *parser) parseOr() (Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.l.tok.kind == tokLogic && p.l.tok.value == LogicOr {
		if err := p.l.next(); err != nil {
			return nil, err
		}
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &LogicExpr{Op: LogicOr, Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseAnd() (Expr, error) {
	left, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	for p.l.tok.kind == tokLogic && p.l.tok.value == LogicAnd {
		if err := p.l.next(); err != nil {
			return nil, err
		}
		right, err := p.parseAtom()
		if err != nil {
			return nil, err
		}
		left = &LogicExpr{Op: LogicAnd, Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseAtom() (Expr, error) {
	if p.l.tok.kind == tokLParen {
		if err := p.l.next(); err != nil {
			return nil, err
		}
		expr, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.l.tok.kind != tokRParen {
			return nil, &InvalidFilterError{Pos: p.l.tokPos, Msg: "expected ')'"}
		}
		if err := p.l.next(); err != nil {
			return nil, err
		}
		return expr, nil
	}
	return p.parseComparison()
}

func (p *parser) parseComparison() (Expr, error) {
	if p.l.tok.kind != tokIdent {
		return nil, &InvalidFilterError{Pos: p.l.tokPos, Msg: "expected attribute name"}
	}
	attr := strings.ToLower(p.l.tok.value)
	if err := p.l.next(); err != nil {
		return nil, err
	}
	// Optional sub-attribute path (single dot).
	if p.l.tok.kind == tokDot {
		if err := p.l.next(); err != nil {
			return nil, err
		}
		if p.l.tok.kind != tokIdent {
			return nil, &InvalidFilterError{Pos: p.l.tokPos, Msg: "expected sub-attribute name after '.'"}
		}
		attr = attr + "." + strings.ToLower(p.l.tok.value)
		if err := p.l.next(); err != nil {
			return nil, err
		}
	}

	switch p.l.tok.kind { //nolint:exhaustive // unhandled token kinds fall through to the error return below
	case tokPr:
		if err := p.l.next(); err != nil {
			return nil, err
		}
		return &AtomExpr{Attr: attr, Op: OpPr}, nil
	case tokOp:
		op := p.l.tok.value
		if err := p.l.next(); err != nil {
			return nil, err
		}
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		return &AtomExpr{Attr: attr, Op: op, Value: val}, nil
	}
	return nil, &InvalidFilterError{Pos: p.l.tokPos, Msg: "expected operator (eq, ne, co, sw, ew, pr)"}
}

func (p *parser) parseValue() (any, error) {
	t := p.l.tok
	switch t.kind { //nolint:exhaustive // unhandled token kinds fall through to the error return below
	case tokString:
		if err := p.l.next(); err != nil {
			return nil, err
		}
		return t.value, nil
	case tokNumber:
		f, err := strconv.ParseFloat(t.value, 64)
		if err != nil {
			return nil, &InvalidFilterError{Pos: p.l.tokPos, Msg: "invalid number: " + err.Error()}
		}
		if err := p.l.next(); err != nil {
			return nil, err
		}
		return f, nil
	case tokTrue:
		if err := p.l.next(); err != nil {
			return nil, err
		}
		return true, nil
	case tokFalse:
		if err := p.l.next(); err != nil {
			return nil, err
		}
		return false, nil
	case tokNull:
		if err := p.l.next(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	return nil, &InvalidFilterError{Pos: p.l.tokPos, Msg: "expected literal value (string, number, true, false, null)"}
}
