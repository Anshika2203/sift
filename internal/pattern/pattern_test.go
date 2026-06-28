package pattern

import "testing"

func matches(query, text string) bool {
	_, _, ok := Parse(query).Match(text, false)
	return ok
}

func TestExtendedSyntax(t *testing.T) {
	cases := []struct {
		query, text string
		want        bool
	}{
		{"foo", "barfoobaz", true},       // fuzzy substring
		{"fbb", "FooBarBaz", true},       // fuzzy subsequence
		{"'bar", "foobarbaz", true},      // exact substring present
		{"'xyz", "foobarbaz", false},     // exact substring absent
		{"^foo", "foobar", true},         // prefix
		{"^bar", "foobar", false},        // prefix mismatch
		{"bar$", "foobar", true},         // suffix
		{"foo$", "foobar", false},        // suffix mismatch
		{"^foo$", "foo", true},           // equal
		{"^foo$", "foobar", false},       // equal mismatch
		{"!foo", "barbaz", true},         // inverse: absent -> match
		{"!foo", "foobar", false},        // inverse: present -> no match
		{"foo !bar", "foobaz", true},     // AND with inverse, ok
		{"foo !bar", "foobar", false},    // inverse term rejects
		{"foo bar", "foo and bar", true}, // two ANDed fuzzy terms
		{"foo bar", "only foo", false},   // one term missing
		{"!^foo", "barfoo", true},        // inverse prefix: not a prefix -> match
		{"!^foo", "foobar", false},       // inverse prefix: is a prefix -> no match
	}
	for _, c := range cases {
		if got := matches(c.query, c.text); got != c.want {
			t.Errorf("Parse(%q).Match(%q) = %v, want %v", c.query, c.text, got, c.want)
		}
	}
}

func TestSmartCase(t *testing.T) {
	if !matches("foo", "FOOBAR") {
		t.Error("lowercase query should match case-insensitively")
	}
	if matches("FOO", "foobar") {
		t.Error("uppercase query should be case-sensitive")
	}
}

func TestEmptyMatchesEverything(t *testing.T) {
	p := Parse("   ")
	if !p.IsEmpty() {
		t.Fatal("blank query should be empty")
	}
	if _, _, ok := p.Match("anything", false); !ok {
		t.Error("empty pattern should match any item")
	}
}

func TestSortable(t *testing.T) {
	if Parse("!foo").Sortable() {
		t.Error("a query of only inverse terms should not be sortable")
	}
	if !Parse("foo !bar").Sortable() {
		t.Error("a query with a positive term should be sortable")
	}
}
