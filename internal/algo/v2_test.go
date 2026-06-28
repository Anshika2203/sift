package algo

import "testing"

func TestV2BasicMatch(t *testing.T) {
	cases := []struct {
		text, pat string
		want      bool
	}{
		{"report_2024_final.txt", "rprt", true},
		{"report_2024_final.txt", "xyz", false},
		{"foobar", "foob", true},
		{"foobar", "fbz", false},
	}
	for _, c := range cases {
		_, ok := MatchV2(c.text, lower(c.pat), false, false)
		if ok != c.want {
			t.Errorf("MatchV2(%q, %q) ok=%v want %v", c.text, c.pat, ok, c.want)
		}
	}
}

func v2Score(t *testing.T, text, pat string) int {
	t.Helper()
	r, ok := MatchV2(text, lower(pat), false, false)
	if !ok {
		t.Fatalf("expected %q to match %q", pat, text)
	}
	return r.Score
}

func TestV2PrefersWordBoundary(t *testing.T) {
	if v2Score(t, "fuzzy-finder", "ff") <= v2Score(t, "fuzzyfinder", "ff") {
		t.Error("v2: 'fuzzy-finder' should outrank 'fuzzyfinder' for 'ff'")
	}
}

func TestV2PrefersConsecutive(t *testing.T) {
	if v2Score(t, "foobar", "foob") <= v2Score(t, "foo-bar", "foob") {
		t.Error("v2: 'foobar' should outrank 'foo-bar' for 'foob'")
	}
}

// V2 finds the optimal alignment where the greedy V1 might not. For " ab" style
// inputs with a closer second occurrence, the score should reflect the best.
func TestV2FindsBetterAlignment(t *testing.T) {
	// "ab" appears spread out early and tightly later; v2 should pick the tight one.
	r1, ok := MatchV2("a___b___ab", lower("ab"), false, true)
	if !ok {
		t.Fatal("expected match")
	}
	// The optimal alignment is the trailing consecutive "ab" (positions 8,9).
	if len(r1.Positions) != 2 || r1.Positions[0] != 8 || r1.Positions[1] != 9 {
		t.Errorf("v2 positions = %v, want [8 9]", r1.Positions)
	}
}

func TestV2Positions(t *testing.T) {
	r, ok := MatchV2("foobar", lower("fb"), false, true)
	if !ok {
		t.Fatal("expected match")
	}
	if len(r.Positions) != 2 || r.Positions[0] != 0 || r.Positions[1] != 3 {
		t.Errorf("positions = %v, want [0 3]", r.Positions)
	}
}
