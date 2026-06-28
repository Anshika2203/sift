package tokenizer

import "testing"

func ranges(t *testing.T, spec string) []Range {
	t.Helper()
	r, ok := ParseRanges(spec)
	if !ok {
		t.Fatalf("ParseRanges(%q) failed", spec)
	}
	return r
}

func TestTokenizeWhitespace(t *testing.T) {
	toks := Tokenize("  foo   bar baz ", NewDelimiter(""))
	if len(toks) != 3 {
		t.Fatalf("got %d tokens, want 3: %v", len(toks), toks)
	}
	if toks[0].Text != "foo" || toks[0].Start != 2 {
		t.Errorf("token0 = %+v, want {foo 2}", toks[0])
	}
	if toks[2].Text != "baz" {
		t.Errorf("token2 = %q, want baz", toks[2].Text)
	}
}

func TestTokenizeLiteral(t *testing.T) {
	toks := Tokenize("a,bb,,c", NewDelimiter(","))
	want := []string{"a", "bb", "", "c"}
	if len(toks) != len(want) {
		t.Fatalf("got %d tokens, want %d", len(toks), len(want))
	}
	for i, w := range want {
		if toks[i].Text != w {
			t.Errorf("token%d = %q, want %q", i, toks[i].Text, w)
		}
	}
	// "c" starts after "a,bb,," = 6 runes
	if toks[3].Start != 6 {
		t.Errorf("token3.Start = %d, want 6", toks[3].Start)
	}
}

func TestSelectAndMap(t *testing.T) {
	// Fields: "alpha"(0) "beta"(6) "gamma"(11)
	src := "alpha beta gamma"
	out, pm := Select(src, NewDelimiter(""), ranges(t, "2"))
	if out != "beta" {
		t.Errorf("Select field 2 = %q, want beta", out)
	}
	// position 0 of "beta" maps to offset 6 in src
	if pm[0] != 6 {
		t.Errorf("posMap[0] = %d, want 6", pm[0])
	}

	out2, _ := Select(src, NewDelimiter(""), ranges(t, "2.."))
	if out2 != "beta gamma" {
		t.Errorf("Select 2.. = %q, want 'beta gamma'", out2)
	}

	out3, _ := Select(src, NewDelimiter(""), ranges(t, "-1"))
	if out3 != "gamma" {
		t.Errorf("Select -1 = %q, want gamma", out3)
	}

	out4, _ := Select(src, NewDelimiter(""), ranges(t, "..2"))
	if out4 != "alpha beta" {
		t.Errorf("Select ..2 = %q, want 'alpha beta'", out4)
	}
}

func TestParseRangesInvalid(t *testing.T) {
	if _, ok := ParseRanges("1,abc"); ok {
		t.Error("expected ParseRanges to reject 'abc'")
	}
}
