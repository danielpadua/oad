package groups_test

import (
	"strings"
	"testing"

	"github.com/danielpadua/oad/internal/scim/groups"
	"github.com/danielpadua/oad/internal/scim/parser"
)

func mustParse(t *testing.T, in string) parser.Expr {
	t.Helper()
	expr, err := parser.Parse(in)
	if err != nil {
		t.Fatalf("parser.Parse(%q): %v", in, err)
	}
	return expr
}

func TestFilterToSQL_DisplayNameEq(t *testing.T) {
	t.Parallel()
	sql, args, err := groups.FilterToSQL(mustParse(t, `displayName eq "Engineering"`))
	if err != nil {
		t.Fatalf("FilterToSQL: %v", err)
	}
	if !strings.Contains(sql, `e.properties->>'displayName'`) || !strings.Contains(sql, "= $1") {
		t.Errorf("sql = %q", sql)
	}
	if len(args) != 1 || args[0] != "Engineering" {
		t.Errorf("args = %v, want [Engineering]", args)
	}
}

func TestFilterToSQL_DisplayNameContains(t *testing.T) {
	t.Parallel()
	sql, args, err := groups.FilterToSQL(mustParse(t, `displayName co "eng"`))
	if err != nil {
		t.Fatalf("FilterToSQL: %v", err)
	}
	if !strings.Contains(sql, "ILIKE") || !strings.Contains(sql, "ESCAPE '\\'") {
		t.Errorf("sql = %q, want ILIKE pattern with ESCAPE", sql)
	}
	if got, want := args[0], "%eng%"; got != want {
		t.Errorf("args[0] = %q, want %q", got, want)
	}
}

func TestFilterToSQL_DisplayNameStartsAndEnds(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		`displayName sw "Eng"`: "Eng%",
		`displayName ew "ing"`: "%ing",
	}
	for input, wantArg := range cases {
		_, args, err := groups.FilterToSQL(mustParse(t, input))
		if err != nil {
			t.Errorf("%s: %v", input, err)
			continue
		}
		if args[0] != wantArg {
			t.Errorf("%s: args[0] = %q, want %q", input, args[0], wantArg)
		}
	}
}

func TestFilterToSQL_ExternalIDEq(t *testing.T) {
	t.Parallel()
	sql, args, err := groups.FilterToSQL(mustParse(t, `externalId eq "eng-1"`))
	if err != nil {
		t.Fatalf("FilterToSQL: %v", err)
	}
	if !strings.Contains(sql, "ei.external_subject = $1") {
		t.Errorf("sql = %q", sql)
	}
	if args[0] != "eng-1" {
		t.Errorf("args[0] = %q, want eng-1", args[0])
	}
}

func TestFilterToSQL_AndCompound(t *testing.T) {
	t.Parallel()
	sql, args, err := groups.FilterToSQL(mustParse(t, `displayName eq "Eng" and externalId pr`))
	if err != nil {
		t.Fatalf("FilterToSQL: %v", err)
	}
	if !strings.Contains(sql, " AND ") || !strings.Contains(sql, "IS NOT NULL") {
		t.Errorf("sql = %q", sql)
	}
	if len(args) != 1 {
		t.Errorf("args = %v, want 1 placeholder bound", args)
	}
}

func TestFilterToSQL_UnsupportedAttribute(t *testing.T) {
	t.Parallel()
	_, _, err := groups.FilterToSQL(mustParse(t, `members eq "anything"`))
	if err == nil {
		t.Fatalf("FilterToSQL: expected error for unsupported attribute")
	}
}

func TestFilterToSQL_NumericOpOnStringAttr(t *testing.T) {
	t.Parallel()
	// `co` against `id` is allowed (both string-typed in our spec); the
	// rejection path is operator-mismatch on a non-string. For Group the
	// spec exposes only string attributes, so the rejection path is the
	// unsupported-attribute case above. This test simply confirms id/co
	// translates without error.
	if _, _, err := groups.FilterToSQL(mustParse(t, `id co "abc"`)); err != nil {
		t.Fatalf("FilterToSQL(id co): %v", err)
	}
}

func TestEscapeLike_HandlesSpecials(t *testing.T) {
	t.Parallel()
	// `\\` in the SCIM filter source escapes to a single literal backslash
	// after parsing; escapeLike must double it again for the LIKE engine.
	_, args, err := groups.FilterToSQL(mustParse(t, `displayName co "10%_\\path"`))
	if err != nil {
		t.Fatalf("FilterToSQL: %v", err)
	}
	got := args[0].(string)
	for _, want := range []string{`\%`, `\_`, `\\`} {
		if !strings.Contains(got, want) {
			t.Errorf("escaped pattern missing %q: got %q", want, got)
		}
	}
}
