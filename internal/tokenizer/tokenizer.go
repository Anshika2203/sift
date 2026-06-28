// Package tokenizer splits lines into fields and selects field ranges, backing
// sift's --nth / --with-nth / --delimiter options.
package tokenizer

import "strings"

// Delimiter splits a line into fields. The zero value (and NewDelimiter(""))
// splits on runs of whitespace (AWK-style). A non-empty literal splits on each
// occurrence of that string.
type Delimiter struct {
	literal string
}

// NewDelimiter returns a Delimiter. An empty string means AWK whitespace.
func NewDelimiter(s string) Delimiter { return Delimiter{literal: s} }

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\v' || r == '\f' || r == '\r'
}

// Token is a field together with its starting rune offset in the source line.
type Token struct {
	Text  string
	Start int
}

// Tokenize splits s into fields according to d.
func Tokenize(s string, d Delimiter) []Token {
	runes := []rune(s)
	var toks []Token

	if d.literal == "" {
		i := 0
		for i < len(runes) {
			for i < len(runes) && isSpace(runes[i]) {
				i++
			}
			if i >= len(runes) {
				break
			}
			start := i
			for i < len(runes) && !isSpace(runes[i]) {
				i++
			}
			toks = append(toks, Token{string(runes[start:i]), start})
		}
		return toks
	}

	dl := []rune(d.literal)
	start, i := 0, 0
	for i+len(dl) <= len(runes) {
		if matchAt(runes, dl, i) {
			toks = append(toks, Token{string(runes[start:i]), start})
			i += len(dl)
			start = i
		} else {
			i++
		}
	}
	toks = append(toks, Token{string(runes[start:]), start})
	return toks
}

func matchAt(runes, sub []rune, at int) bool {
	for k := range sub {
		if runes[at+k] != sub[k] {
			return false
		}
	}
	return true
}

// Range is a 1-based field selector. Negative numbers count from the end; an
// unset bound (loSet/hiSet false) is open.
type Range struct {
	lo, hi       int
	loSet, hiSet bool
}

// ParseRanges parses a comma-separated spec like "1,2,-1,2..3,..2,3.." and
// returns the ranges (ok=false on a malformed spec).
func ParseRanges(spec string) ([]Range, bool) {
	var out []Range
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var r Range
		if i := strings.Index(part, ".."); i >= 0 {
			lo, hi := part[:i], part[i+2:]
			if lo != "" {
				n, ok := atoi(lo)
				if !ok {
					return nil, false
				}
				r.lo, r.loSet = n, true
			}
			if hi != "" {
				n, ok := atoi(hi)
				if !ok {
					return nil, false
				}
				r.hi, r.hiSet = n, true
			}
		} else {
			n, ok := atoi(part)
			if !ok {
				return nil, false
			}
			r.lo, r.hi, r.loSet, r.hiSet = n, n, true, true
		}
		out = append(out, r)
	}
	return out, true
}

func atoi(s string) (int, bool) {
	neg := false
	if strings.HasPrefix(s, "-") {
		neg, s = true, s[1:]
	}
	if s == "" {
		return 0, false
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	if neg {
		n = -n
	}
	return n, true
}

func selected(nFields int, ranges []Range) []bool {
	sel := make([]bool, nFields)
	to0 := func(v int) int {
		if v < 0 {
			return nFields + v // -1 -> last
		}
		return v - 1 // 1-based -> 0-based
	}
	for _, r := range ranges {
		lo := 0
		if r.loSet {
			lo = to0(r.lo)
		}
		hi := nFields - 1
		if r.hiSet {
			hi = to0(r.hi)
		}
		if lo < 0 {
			lo = 0
		}
		if hi > nFields-1 {
			hi = nFields - 1
		}
		for i := lo; i <= hi; i++ {
			if i >= 0 {
				sel[i] = true
			}
		}
	}
	return sel
}

// Select returns the space-joined text of the fields picked by ranges, plus a
// map from each rune index in the result to its rune offset in src (-1 for the
// spaces inserted between fields).
func Select(src string, d Delimiter, ranges []Range) (string, []int) {
	toks := Tokenize(src, d)
	sel := selected(len(toks), ranges)

	var b []rune
	var pm []int
	firstOut := true
	for i, tok := range toks {
		if !sel[i] {
			continue
		}
		if !firstOut {
			b = append(b, ' ')
			pm = append(pm, -1)
		}
		firstOut = false
		for k, r := range []rune(tok.Text) {
			b = append(b, r)
			pm = append(pm, tok.Start+k)
		}
	}
	return string(b), pm
}

// Join is Select without the position map, for --with-nth display transforms.
func Join(src string, d Delimiter, ranges []Range) string {
	s, _ := Select(src, d, ranges)
	return s
}
