package pattern

import "testing"

var fuzzySmart = Options{Fuzzy: true, Case: CaseSmart}

func matchesWith(query, text string, opts Options) bool {
	_, _, ok := Parse(query, opts).Match(text, false)
	return ok
}

func matches(query, text string) bool {
	return matchesWith(query, text, fuzzySmart)
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
			t.Errorf("Match(%q, %q) = %v, want %v", c.query, c.text, got, c.want)
		}
	}
}

func TestBoundaryMatch(t *testing.T) {
	cases := []struct {
		query, text string
		want        bool
	}{
		{"'wild'", "a wild thing", true},
		{"'wild'", "the_wild_west", true}, // underscores are boundaries
		{"'wild'", "wildcard", false},     // no right boundary
		{"'wild'", "rewild", false},       // no left boundary
	}
	for _, c := range cases {
		if got := matches(c.query, c.text); got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.query, c.text, got, c.want)
		}
	}
}

func TestOrOperator(t *testing.T) {
	cases := []struct {
		query, text string
		want        bool
	}{
		{"go$ | rb$", "main.go", true},
		{"go$ | rb$", "main.rb", true},
		{"go$ | rb$", "main.py", false},
		{"^src | ^lib", "src/main.go", true},
		{"^src | ^lib", "lib/util.go", true},
		{"^src | ^lib", "cmd/main.go", false},
		{"foo bar | baz", "foo baz", true}, // AND(foo, (bar|baz))
		{"foo bar | baz", "foo qux", false},
	}
	for _, c := range cases {
		if got := matches(c.query, c.text); got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.query, c.text, got, c.want)
		}
	}
}

func TestExactMode(t *testing.T) {
	exact := Options{Fuzzy: false, Case: CaseSmart}
	// In exact mode a bare term is an exact substring, not a subsequence.
	if matchesWith("fbb", "FooBarBaz", exact) {
		t.Error("exact mode should not treat 'fbb' as a subsequence match")
	}
	if !matchesWith("Bar", "FooBarBaz", exact) {
		t.Error("exact mode should match the literal substring 'Bar'")
	}
	// A leading quote flips back to fuzzy in exact mode.
	if !matchesWith("'fbb", "FooBarBaz", exact) {
		t.Error("'fbb in exact mode should be fuzzy and match")
	}
}

func TestCaseModes(t *testing.T) {
	if !matchesWith("foo", "FOOBAR", Options{Fuzzy: true, Case: CaseSmart}) {
		t.Error("smart case: lowercase query should be case-insensitive")
	}
	if matchesWith("FOO", "foobar", Options{Fuzzy: true, Case: CaseSmart}) {
		t.Error("smart case: uppercase query should be case-sensitive")
	}
	if !matchesWith("FOO", "foobar", Options{Fuzzy: true, Case: CaseIgnore}) {
		t.Error("ignore case: should match regardless of case")
	}
	if matchesWith("foo", "FOOBAR", Options{Fuzzy: true, Case: CaseRespect}) {
		t.Error("respect case: lowercase query should not match uppercase text")
	}
}

func TestEmptyMatchesEverything(t *testing.T) {
	p := Parse("   ", fuzzySmart)
	if !p.IsEmpty() {
		t.Fatal("blank query should be empty")
	}
	if _, _, ok := p.Match("anything", false); !ok {
		t.Error("empty pattern should match any item")
	}
}

func TestSortable(t *testing.T) {
	if Parse("!foo", fuzzySmart).Sortable() {
		t.Error("a query of only inverse terms should not be sortable")
	}
	if !Parse("foo !bar", fuzzySmart).Sortable() {
		t.Error("a query with a positive term should be sortable")
	}
}
