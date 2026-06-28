package algo

import "testing"

func lower(s string) []rune {
	r := []rune(s)
	for i := range r {
		r[i] = toLower(r[i])
	}
	return r
}

func TestMatchBasic(t *testing.T) {
	cases := []struct {
		text    string
		pattern string
		want    bool
	}{
		{"report_2024_final.txt", "rprt", true},
		{"report_2024_final.txt", "rprt24", true},
		{"report_2024_final.txt", "xyz", false},
		{"foobar", "foob", true},
		{"foobar", "fbz", false},
		{"", "a", false},
		{"anything", "", true},
	}
	for _, c := range cases {
		_, ok := Match(c.text, lower(c.pattern), false, false)
		if ok != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.text, c.pattern, ok, c.want)
		}
	}
}

func mustScore(t *testing.T, text, pattern string) int {
	t.Helper()
	r, ok := Match(text, lower(pattern), false, false)
	if !ok {
		t.Fatalf("expected %q to match %q", pattern, text)
	}
	return r.Score
}

// Word-boundary matches should outrank crammed-together matches.
func TestPrefersWordBoundary(t *testing.T) {
	hyphen := mustScore(t, "fuzzy-finder", "ff")
	crammed := mustScore(t, "fuzzyfinder", "ff")
	if hyphen <= crammed {
		t.Errorf("expected 'fuzzy-finder' (%d) to outrank 'fuzzyfinder' (%d)", hyphen, crammed)
	}
}

// Consecutive matches should outrank gapped matches.
func TestPrefersConsecutive(t *testing.T) {
	consec := mustScore(t, "foobar", "foob")
	gapped := mustScore(t, "foo-bar", "foob")
	if consec <= gapped {
		t.Errorf("expected 'foobar' (%d) to outrank 'foo-bar' (%d)", consec, gapped)
	}
}

// camelCase humps should be treated as boundaries.
func TestCamelCase(t *testing.T) {
	camel := mustScore(t, "FooBarBaz", "fbb")
	flat := mustScore(t, "foobarbaz", "fbb")
	if camel <= flat {
		t.Errorf("expected 'FooBarBaz' (%d) to outrank 'foobarbaz' (%d)", camel, flat)
	}
}

func TestPositions(t *testing.T) {
	r, ok := Match("foobar", lower("fb"), false, true)
	if !ok {
		t.Fatal("expected match")
	}
	// 'f' at 0, 'b' at 3
	if len(r.Positions) != 2 || r.Positions[0] != 0 || r.Positions[1] != 3 {
		t.Errorf("positions = %v, want [0 3]", r.Positions)
	}
}

func TestCaseSensitive(t *testing.T) {
	if _, ok := Match("Foobar", []rune("foo"), true, false); ok {
		t.Error("case-sensitive match should fail on case mismatch")
	}
	if _, ok := Match("Foobar", []rune("Foo"), true, false); !ok {
		t.Error("case-sensitive match should succeed on exact case")
	}
}
