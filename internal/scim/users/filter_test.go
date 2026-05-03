package users_test

import (
	"strings"
	"testing"

	"github.com/danielpadua/oad/internal/scim/parser"
	"github.com/danielpadua/oad/internal/scim/users"
)

func mustParse(t *testing.T, input string) parser.Expr {
	t.Helper()
	expr, err := parser.Parse(input)
	if err != nil {
		t.Fatalf("Parse(%q): %v", input, err)
	}
	return expr
}

func TestFilterToSQL_BasicComparisons(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		filter   string
		wantSQL  string
		wantArgs []any
	}{
		{
			name:     "eq string",
			filter:   `userName eq "alice"`,
			wantSQL:  `e.properties->>'userName' = $1`,
			wantArgs: []any{"alice"},
		},
		{
			name:     "ne string",
			filter:   `userName ne "bob"`,
			wantSQL:  `e.properties->>'userName' <> $1`,
			wantArgs: []any{"bob"},
		},
		{
			name:     "eq bool",
			filter:   `active eq true`,
			wantSQL:  `(e.properties->>'active')::bool = $1`,
			wantArgs: []any{true},
		},
		{
			name:     "presence",
			filter:   `displayName pr`,
			wantSQL:  `e.properties->>'displayName' IS NOT NULL`,
			wantArgs: nil,
		},
		{
			name:     "externalId",
			filter:   `externalId eq "alice-kc"`,
			wantSQL:  `ei.external_subject = $1`,
			wantArgs: []any{"alice-kc"},
		},
		{
			name:     "id eq",
			filter:   `id eq "550e8400-e29b-41d4-a716-446655440000"`,
			wantSQL:  `e.id::text = $1`,
			wantArgs: []any{"550e8400-e29b-41d4-a716-446655440000"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotSQL, gotArgs, err := users.FilterToSQL(mustParse(t, tc.filter))
			if err != nil {
				t.Fatalf("FilterToSQL: %v", err)
			}
			if gotSQL != tc.wantSQL {
				t.Errorf("sql = %q, want %q", gotSQL, tc.wantSQL)
			}
			if !equalArgs(gotArgs, tc.wantArgs) {
				t.Errorf("args = %v, want %v", gotArgs, tc.wantArgs)
			}
		})
	}
}

func TestFilterToSQL_StringPatterns(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		filter      string
		wantPattern string
	}{
		{"contains", `userName co "ali"`, "%ali%"},
		{"startsWith", `userName sw "al"`, "al%"},
		{"endsWith", `userName ew "ce"`, "%ce"},
		{"escapes percent", `userName co "50%"`, `%50\%%`},
		{"escapes underscore", `userName co "alice_smith"`, `%alice\_smith%`},
		{"escapes backslash", `userName co "a\\b"`, `%a\\b%`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotSQL, gotArgs, err := users.FilterToSQL(mustParse(t, tc.filter))
			if err != nil {
				t.Fatalf("FilterToSQL: %v", err)
			}
			if !strings.Contains(gotSQL, "ILIKE") || !strings.Contains(gotSQL, "ESCAPE") {
				t.Errorf("expected ILIKE ... ESCAPE in sql, got %q", gotSQL)
			}
			if len(gotArgs) != 1 || gotArgs[0] != tc.wantPattern {
				t.Errorf("pattern = %v, want %q", gotArgs, tc.wantPattern)
			}
		})
	}
}

func TestFilterToSQL_Compound(t *testing.T) {
	t.Parallel()

	gotSQL, gotArgs, err := users.FilterToSQL(mustParse(t, `userName eq "alice" and active eq true`))
	if err != nil {
		t.Fatalf("FilterToSQL: %v", err)
	}
	wantSQL := `(e.properties->>'userName' = $1 AND (e.properties->>'active')::bool = $2)`
	if gotSQL != wantSQL {
		t.Errorf("sql = %q, want %q", gotSQL, wantSQL)
	}
	if !equalArgs(gotArgs, []any{"alice", true}) {
		t.Errorf("args = %v, want [alice true]", gotArgs)
	}
}

func TestFilterToSQL_OrPrecedence(t *testing.T) {
	t.Parallel()

	// "or" is the outer combinator because "and" binds tighter.
	gotSQL, _, err := users.FilterToSQL(mustParse(t, `userName eq "a" or userName eq "b" and active eq true`))
	if err != nil {
		t.Fatalf("FilterToSQL: %v", err)
	}
	if !strings.HasPrefix(gotSQL, "(") || !strings.Contains(gotSQL, " OR ") || !strings.Contains(gotSQL, " AND ") {
		t.Errorf("unexpected sql layout: %q", gotSQL)
	}
}

func TestFilterToSQL_Errors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		filter string
	}{
		{"unknown attribute", `unknownField eq "x"`},
		{"co on bool", `active co "tr"`},
		{"sw on bool", `active sw "tr"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := users.FilterToSQL(mustParse(t, tc.filter))
			if err == nil {
				t.Fatalf("expected error for %q", tc.filter)
			}
		})
	}
}

func equalArgs(got, want []any) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
