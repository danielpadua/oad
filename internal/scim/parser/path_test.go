package parser_test

import (
	"strings"
	"testing"

	"github.com/danielpadua/oad/internal/scim/parser"
)

func TestParsePath_Empty(t *testing.T) {
	t.Parallel()
	p, err := parser.ParsePath("")
	if err != nil {
		t.Fatalf("ParsePath(\"\"): unexpected error: %v", err)
	}
	if p.Attr != "" || p.SubAttr != "" || p.Filter != nil {
		t.Errorf("empty input: got %+v, want zero Path", p)
	}
}

func TestParsePath_SimpleAttr(t *testing.T) {
	t.Parallel()
	p, err := parser.ParsePath("displayName")
	if err != nil {
		t.Fatalf("ParsePath: %v", err)
	}
	if p.Attr != "displayname" {
		t.Errorf("Attr = %q, want displayname", p.Attr)
	}
	if p.SubAttr != "" || p.Filter != nil {
		t.Errorf("unexpected fields: %+v", p)
	}
}

func TestParsePath_BracketFilterAndSubAttr(t *testing.T) {
	t.Parallel()
	p, err := parser.ParsePath(`emails[primary eq true].value`)
	if err != nil {
		t.Fatalf("ParsePath: %v", err)
	}
	if p.Attr != "emails" {
		t.Errorf("Attr = %q, want emails", p.Attr)
	}
	if p.SubAttr != "value" {
		t.Errorf("SubAttr = %q, want value", p.SubAttr)
	}
	atom, ok := p.Filter.(*parser.AtomExpr)
	if !ok {
		t.Fatalf("Filter type = %T, want *AtomExpr", p.Filter)
	}
	if atom.Attr != "primary" || atom.Op != parser.OpEq || atom.Value != true {
		t.Errorf("filter = %+v, want primary eq true", atom)
	}
}

func TestParsePath_MembersFilter(t *testing.T) {
	t.Parallel()
	p, err := parser.ParsePath(`members[value eq "abc-123"]`)
	if err != nil {
		t.Fatalf("ParsePath: %v", err)
	}
	if p.Attr != "members" {
		t.Errorf("Attr = %q, want members", p.Attr)
	}
	if p.SubAttr != "" {
		t.Errorf("SubAttr = %q, want empty", p.SubAttr)
	}
	atom, ok := p.Filter.(*parser.AtomExpr)
	if !ok {
		t.Fatalf("Filter type = %T, want *AtomExpr", p.Filter)
	}
	if atom.Attr != "value" || atom.Value != "abc-123" {
		t.Errorf("filter = %+v, want value eq \"abc-123\"", atom)
	}
}

func TestParsePath_QuotedBracketContent(t *testing.T) {
	t.Parallel()
	// A `]` inside the quoted value must not terminate the bracket.
	p, err := parser.ParsePath(`members[value eq "ab]cd"]`)
	if err != nil {
		t.Fatalf("ParsePath: %v", err)
	}
	atom, _ := p.Filter.(*parser.AtomExpr)
	if atom == nil || atom.Value != "ab]cd" {
		t.Errorf("filter value = %v, want 'ab]cd'", atom)
	}
}

func TestParsePath_CaseInsensitive(t *testing.T) {
	t.Parallel()
	p, err := parser.ParsePath("DisplayName")
	if err != nil {
		t.Fatalf("ParsePath: %v", err)
	}
	if p.Attr != "displayname" {
		t.Errorf("Attr = %q, want lowercased displayname", p.Attr)
	}
}

func TestParsePath_Errors(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"":            "", // empty handled separately
		"[":           "expected attribute name",
		"emails[":     "unterminated bracket filter",
		"emails[]":    "invalid bracket filter",
		"emails.":     "expected sub-attribute name",
		"emails.123":  "", // 123 is identchar — actually valid ident, won't error
		"emails junk": "unexpected trailing tokens",
	}
	for input, wantSubstr := range cases {
		if input == "" || input == "emails.123" {
			continue
		}
		_, err := parser.ParsePath(input)
		if err == nil {
			t.Errorf("ParsePath(%q): expected error", input)
			continue
		}
		if wantSubstr != "" && !strings.Contains(err.Error(), wantSubstr) {
			t.Errorf("ParsePath(%q): error = %v, want substring %q", input, err, wantSubstr)
		}
	}
}
