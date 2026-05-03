package users

import (
	"fmt"
	"strings"

	"github.com/danielpadua/oad/internal/scim/parser"
)

// FilterToSQL translates a SCIM filter AST into a SQL fragment that can be
// embedded inside a WHERE clause for the SCIM /Users LIST query. The
// fragment uses positional placeholders ($1, $2, ...) tied to the returned
// args slice. Callers append it to a base WHERE that already constrains
// type='User' and the calling provider.
//
// Supported attributes (case-insensitive):
//
//	userName, displayName, active, emails.value, id, externalId
//
// Unsupported attributes return a translation error which the handler
// surfaces as 400 Bad Request with SCIM scimType=invalidFilter.
func FilterToSQL(expr parser.Expr) (sql string, args []any, err error) {
	b := &filterBuilder{}
	sql, err = b.build(expr)
	if err != nil {
		return "", nil, err
	}
	return sql, b.args, nil
}

type attrSpec struct {
	sql      string // SQL expression that yields the comparison column
	isString bool   // whether co/sw/ew apply
}

// userAttrs maps lowercased SCIM attribute paths to SQL expressions.
// `e` is the entity row alias, `ei` is the entity_external_identity alias —
// both are joined by the LIST query.
var userAttrs = map[string]attrSpec{
	"username":     {sql: "e.properties->>'userName'", isString: true},
	"displayname":  {sql: "e.properties->>'displayName'", isString: true},
	"active":       {sql: "(e.properties->>'active')::bool", isString: false},
	"emails.value": {sql: "e.properties->>'email'", isString: true},
	"id":           {sql: "e.id::text", isString: true},
	"externalid":   {sql: "ei.external_subject", isString: true},
}

type filterBuilder struct {
	args []any
}

func (b *filterBuilder) build(expr parser.Expr) (string, error) {
	switch e := expr.(type) {
	case *parser.AtomExpr:
		return b.buildAtom(e)
	case *parser.LogicExpr:
		left, err := b.build(e.Left)
		if err != nil {
			return "", err
		}
		right, err := b.build(e.Right)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("(%s %s %s)", left, strings.ToUpper(e.Op), right), nil
	}
	return "", fmt.Errorf("unsupported expression type %T", expr)
}

func (b *filterBuilder) buildAtom(a *parser.AtomExpr) (string, error) {
	spec, ok := userAttrs[a.Attr]
	if !ok {
		return "", fmt.Errorf("unsupported filter attribute %q (allowed: userName, displayName, active, emails.value, id, externalId)", a.Attr)
	}

	if a.Op == parser.OpPr {
		return spec.sql + " IS NOT NULL", nil
	}

	switch a.Op {
	case parser.OpEq, parser.OpNe:
		op := "="
		if a.Op == parser.OpNe {
			op = "<>"
		}
		b.args = append(b.args, a.Value)
		return fmt.Sprintf("%s %s $%d", spec.sql, op, len(b.args)), nil
	case parser.OpCo, parser.OpSw, parser.OpEw:
		if !spec.isString {
			return "", fmt.Errorf("operator %q only supports string attributes", a.Op)
		}
		s, ok := a.Value.(string)
		if !ok {
			return "", fmt.Errorf("operator %q expects a string value", a.Op)
		}
		pattern := likePattern(a.Op, s)
		b.args = append(b.args, pattern)
		return fmt.Sprintf("%s ILIKE $%d ESCAPE '\\'", spec.sql, len(b.args)), nil
	}
	return "", fmt.Errorf("unsupported operator %q", a.Op)
}

func likePattern(op, value string) string {
	escaped := escapeLike(value)
	switch op {
	case parser.OpCo:
		return "%" + escaped + "%"
	case parser.OpSw:
		return escaped + "%"
	case parser.OpEw:
		return "%" + escaped
	}
	return escaped
}

// escapeLike escapes the wildcard characters PostgreSQL ILIKE recognizes
// (% and _) and the escape character itself (\). The translated fragment
// uses ESCAPE '\' so that `\%` in the pattern matches a literal '%'.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	s = strings.ReplaceAll(s, "_", `\_`)
	return s
}
