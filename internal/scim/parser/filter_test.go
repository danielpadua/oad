package parser_test

import (
	"errors"
	"testing"

	"github.com/danielpadua/oad/internal/scim/parser"
)

func atom(attr, op string, val any) *parser.AtomExpr {
	return &parser.AtomExpr{Attr: attr, Op: op, Value: val}
}

func and(l, r parser.Expr) *parser.LogicExpr {
	return &parser.LogicExpr{Op: parser.LogicAnd, Left: l, Right: r}
}

func or(l, r parser.Expr) *parser.LogicExpr {
	return &parser.LogicExpr{Op: parser.LogicOr, Left: l, Right: r}
}

func TestParse_Atoms(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  parser.Expr
	}{
		{"eq string", `userName eq "alice"`, atom("username", parser.OpEq, "alice")},
		{"ne string", `userName ne "bob"`, atom("username", parser.OpNe, "bob")},
		{"co", `displayName co "Smith"`, atom("displayname", parser.OpCo, "Smith")},
		{"sw", `userName sw "al"`, atom("username", parser.OpSw, "al")},
		{"ew", `userName ew "ce"`, atom("username", parser.OpEw, "ce")},
		{"pr", `displayName pr`, atom("displayname", parser.OpPr, nil)},
		{"eq bool", `active eq true`, atom("active", parser.OpEq, true)},
		{"eq false", `active eq false`, atom("active", parser.OpEq, false)},
		{"eq null", `displayName eq null`, atom("displayname", parser.OpEq, nil)},
		{"sub-attribute", `emails.value eq "x@y.z"`, atom("emails.value", parser.OpEq, "x@y.z")},
		{"case-insensitive operator", `userName EQ "alice"`, atom("username", parser.OpEq, "alice")},
		{
			"case-insensitive logic", `userName eq "a" AND active eq true`,
			and(atom("username", parser.OpEq, "a"), atom("active", parser.OpEq, true)),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parser.Parse(tc.input)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.input, err)
			}
			assertExprEqual(t, got, tc.want)
		})
	}
}

func TestParse_Logic(t *testing.T) {
	t.Parallel()

	// `and` binds tighter than `or` per RFC 7644 §3.4.2.2.
	got, err := parser.Parse(`userName eq "a" or active eq true and displayName pr`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := or(
		atom("username", parser.OpEq, "a"),
		and(
			atom("active", parser.OpEq, true),
			atom("displayname", parser.OpPr, nil),
		),
	)
	assertExprEqual(t, got, want)
}

func TestParse_Parens(t *testing.T) {
	t.Parallel()

	got, err := parser.Parse(`(userName eq "a" or userName eq "b") and active eq true`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := and(
		or(
			atom("username", parser.OpEq, "a"),
			atom("username", parser.OpEq, "b"),
		),
		atom("active", parser.OpEq, true),
	)
	assertExprEqual(t, got, want)
}

func TestParse_StringEscapes(t *testing.T) {
	t.Parallel()

	got, err := parser.Parse(`displayName eq "Quote: \"hello\""`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := atom("displayname", parser.OpEq, `Quote: "hello"`)
	assertExprEqual(t, got, want)
}

func TestParse_Errors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
	}{
		{"empty", ``},
		{"unknown operator", `userName foo "x"`},
		{"unknown gt operator", `score gt 10`},
		{"missing value", `userName eq`},
		{"unterminated string", `userName eq "alice`},
		{"unbalanced paren", `(userName eq "a"`},
		{"trailing garbage", `userName eq "a" extra`},
		{"bare attribute", `userName`},
		{"not operator unsupported", `not (userName eq "a")`},
		{"missing attribute", `eq "a"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := parser.Parse(tc.input)
			if err == nil {
				t.Fatalf("Parse(%q) error = nil, want InvalidFilterError", tc.input)
			}
			var ifErr *parser.InvalidFilterError
			if !errors.As(err, &ifErr) {
				t.Errorf("error type = %T, want *InvalidFilterError", err)
			}
		})
	}
}

// assertExprEqual structurally compares two ASTs. Atom values use loose
// equality (compatible types via fmt %v) which is enough for this test
// surface (string, bool, nil, float64).
func assertExprEqual(t *testing.T, got, want parser.Expr) {
	t.Helper()
	switch w := want.(type) {
	case *parser.AtomExpr:
		g, ok := got.(*parser.AtomExpr)
		if !ok {
			t.Fatalf("got %T, want *AtomExpr", got)
		}
		if g.Attr != w.Attr {
			t.Errorf("Attr = %q, want %q", g.Attr, w.Attr)
		}
		if g.Op != w.Op {
			t.Errorf("Op = %q, want %q", g.Op, w.Op)
		}
		if g.Value != w.Value {
			t.Errorf("Value = %v, want %v", g.Value, w.Value)
		}
	case *parser.LogicExpr:
		g, ok := got.(*parser.LogicExpr)
		if !ok {
			t.Fatalf("got %T, want *LogicExpr", got)
		}
		if g.Op != w.Op {
			t.Errorf("Op = %q, want %q", g.Op, w.Op)
		}
		assertExprEqual(t, g.Left, w.Left)
		assertExprEqual(t, g.Right, w.Right)
	default:
		t.Fatalf("unexpected want type %T", want)
	}
}
