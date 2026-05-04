package parser

import (
	"fmt"
	"strings"
)

// Path is the parsed shape of a SCIM PATCH path expression
// (RFC 7644 §3.5.2). The grammar OAD supports is the subset
// necessary for the documented PATCH paths in scim-ingest §7.1:
//
//	path     = attr ( "[" filter "]" )? ( "." subAttr )?
//	attr     = ident
//	subAttr  = ident
//
// Examples:
//
//	displayName                       → {Attr:"displayname"}
//	emails                            → {Attr:"emails"}
//	emails[primary eq true].value     → {Attr:"emails", Filter:<atom>, SubAttr:"value"}
//	members[value eq "<uuid>"]        → {Attr:"members", Filter:<atom>}
//
// Nested filters (e.g. `members[a and b]`) reuse the filter parser for
// the bracket content, so all logical/comparison operators supported in
// filters are equally supported here.
type Path struct {
	Attr    string
	SubAttr string
	Filter  Expr
}

// InvalidPathError indicates a PATCH path expression that the parser
// could not parse. The Pos field is the byte offset within the input
// where the error was detected.
type InvalidPathError struct {
	Pos int
	Msg string
}

func (e *InvalidPathError) Error() string {
	return fmt.Sprintf("invalid path at position %d: %s", e.Pos, e.Msg)
}

// ParsePath parses input as a SCIM PATCH path expression and returns
// the parsed Path. Attr and SubAttr are lowercased (SCIM attribute
// names are case-insensitive per RFC 7643 §2.1). Returns an
// *InvalidPathError on any grammar violation.
//
// Empty input returns an empty Path with no error — many PATCH
// operations omit the path entirely (operating on the resource root),
// and callers handle that case by inspecting Path.Attr == "".
func ParsePath(input string) (Path, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return Path{}, nil
	}

	// Read the leading attribute name.
	attrEnd := 0
	for attrEnd < len(s) {
		c := s[attrEnd]
		if isIdentChar(c) {
			attrEnd++
			continue
		}
		break
	}
	if attrEnd == 0 {
		return Path{}, &InvalidPathError{Pos: 0, Msg: "expected attribute name"}
	}
	out := Path{Attr: strings.ToLower(s[:attrEnd])}
	rest := s[attrEnd:]
	consumed := attrEnd

	// Optional bracket filter: `[<filter>]`.
	if strings.HasPrefix(rest, "[") {
		closeIdx := indexUnquoted(rest, ']')
		if closeIdx < 0 {
			return Path{}, &InvalidPathError{Pos: consumed, Msg: "unterminated bracket filter"}
		}
		inner := rest[1:closeIdx]
		expr, err := Parse(inner)
		if err != nil {
			return Path{}, &InvalidPathError{Pos: consumed + 1, Msg: "invalid bracket filter: " + err.Error()}
		}
		out.Filter = expr
		rest = rest[closeIdx+1:]
		consumed += closeIdx + 1
	}

	// Optional sub-attribute: `.<ident>`.
	if strings.HasPrefix(rest, ".") {
		sub := rest[1:]
		end := 0
		for end < len(sub) {
			if isIdentChar(sub[end]) {
				end++
				continue
			}
			break
		}
		if end == 0 {
			return Path{}, &InvalidPathError{Pos: consumed + 1, Msg: "expected sub-attribute name after '.'"}
		}
		out.SubAttr = strings.ToLower(sub[:end])
		rest = sub[end:]
		consumed += 1 + end
	}

	if strings.TrimSpace(rest) != "" {
		return Path{}, &InvalidPathError{Pos: consumed, Msg: "unexpected trailing tokens"}
	}
	return out, nil
}

// isIdentChar reports whether c is allowed inside a SCIM identifier
// (RFC 7643 §2.1: ALPHA / DIGIT / "_" / "-"). The "$" character is
// allowed in $ref expressions but never appears at the path top
// level we need to support.
func isIdentChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z':
		return true
	case c >= 'A' && c <= 'Z':
		return true
	case c >= '0' && c <= '9':
		return true
	case c == '_' || c == '-':
		return true
	}
	return false
}

// indexUnquoted returns the index of the first occurrence of ch outside
// of a double-quoted string within s, or -1 if absent. Backslash escapes
// are honored inside the string. This lets us locate the closing `]`
// of a bracket filter without confusing a `]` that lives inside a quoted
// value (e.g. `members[value eq "abc]def"]`).
func indexUnquoted(s string, ch byte) int {
	inStr := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			if c == '\\' && i+1 < len(s) {
				i++
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
			continue
		}
		if c == ch {
			return i
		}
	}
	return -1
}
