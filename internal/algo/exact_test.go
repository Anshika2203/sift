package algo

import "testing"

func TestExactMatch(t *testing.T) {
	if _, ok := ExactMatch("foobarbaz", lower("bar"), false, false); !ok {
		t.Error("expected 'bar' to match as a substring")
	}
	if _, ok := ExactMatch("foobarbaz", lower("xyz"), false, false); ok {
		t.Error("'xyz' should not match")
	}
	// Not a subsequence matcher: gaps are not allowed.
	if _, ok := ExactMatch("f-o-o", lower("foo"), false, false); ok {
		t.Error("exact match should not allow gaps")
	}
}

func TestPrefixSuffixEqual(t *testing.T) {
	if _, ok := PrefixMatch("main.go", lower("main"), false, false); !ok {
		t.Error("prefix should match")
	}
	if _, ok := PrefixMatch("main.go", lower("go"), false, false); ok {
		t.Error("non-prefix should not match")
	}
	if _, ok := SuffixMatch("main.go", lower("go"), false, false); !ok {
		t.Error("suffix should match")
	}
	if _, ok := SuffixMatch("main.go", lower("main"), false, false); ok {
		t.Error("non-suffix should not match")
	}
	if _, ok := EqualMatch("go", lower("go"), false, false); !ok {
		t.Error("equal should match")
	}
	if _, ok := EqualMatch("golang", lower("go"), false, false); ok {
		t.Error("non-equal should not match")
	}
}

func TestExactBoundaryMatch(t *testing.T) {
	cases := []struct {
		text, pat string
		want      bool
	}{
		{"a wild thing", "wild", true},
		{"the_wild_west", "wild", true},
		{"wildcard", "wild", false},
		{"rewild", "wild", false},
		{"wild", "wild", true},
	}
	for _, c := range cases {
		_, ok := ExactBoundaryMatch(c.text, lower(c.pat), false, false)
		if ok != c.want {
			t.Errorf("ExactBoundaryMatch(%q, %q) = %v, want %v", c.text, c.pat, ok, c.want)
		}
	}
}
