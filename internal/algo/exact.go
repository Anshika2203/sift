package algo

// This file adds the non-fuzzy match modes used by sift's extended search
// syntax: exact substring, prefix, suffix, and exact-equality. Each one reuses
// the same scoring pass as the fuzzy matcher so results from different term
// types can be ranked on a comparable scale.
//
// As with Match, callers must pass an already-lowercased pattern when
// caseSensitive is false.

// lowerRunes returns a comparison copy of target, lowercased unless the match
// is case-sensitive.
func lowerRunes(target []rune, caseSensitive bool) []rune {
	cmp := make([]rune, len(target))
	if caseSensitive {
		copy(cmp, target)
	} else {
		for i, r := range target {
			cmp[i] = toLower(r)
		}
	}
	return cmp
}

func equalAt(cmp, pattern []rune, start int) bool {
	for k := range pattern {
		if cmp[start+k] != pattern[k] {
			return false
		}
	}
	return true
}

// ExactMatch finds pattern as a contiguous substring of text, returning the
// highest-scoring occurrence (e.g. one that begins at a word boundary).
func ExactMatch(text string, pattern []rune, caseSensitive, withPos bool) (Result, bool) {
	if len(pattern) == 0 {
		return Result{}, true
	}
	target := []rune(text)
	n, m := len(target), len(pattern)
	if m > n {
		return Result{}, false
	}
	cmp := lowerRunes(target, caseSensitive)

	best, bestScore := -1, 0
	for start := 0; start+m <= n; start++ {
		if !equalAt(cmp, pattern, start) {
			continue
		}
		r := score(target, cmp, pattern, start, start+m, false)
		if best < 0 || r.Score > bestScore {
			best, bestScore = start, r.Score
		}
	}
	if best < 0 {
		return Result{}, false
	}
	return score(target, cmp, pattern, best, best+m, withPos), true
}

// ExactBoundaryMatch finds pattern as a contiguous substring that sits at word
// boundaries on both sides (e.g. 'wild' matches "a wild thing" but not
// "wildcard"). Returns the highest-scoring such occurrence.
func ExactBoundaryMatch(text string, pattern []rune, caseSensitive, withPos bool) (Result, bool) {
	if len(pattern) == 0 {
		return Result{}, true
	}
	target := []rune(text)
	n, m := len(target), len(pattern)
	if m > n {
		return Result{}, false
	}
	cmp := lowerRunes(target, caseSensitive)

	best, bestScore := -1, 0
	for start := 0; start+m <= n; start++ {
		if !equalAt(cmp, pattern, start) {
			continue
		}
		// A boundary exists at the string edge or next to a non-word rune.
		leftOK := start == 0 || classOf(target[start-1]) <= classDelim
		rightOK := start+m == n || classOf(target[start+m]) <= classDelim
		if !leftOK || !rightOK {
			continue
		}
		r := score(target, cmp, pattern, start, start+m, false)
		if best < 0 || r.Score > bestScore {
			best, bestScore = start, r.Score
		}
	}
	if best < 0 {
		return Result{}, false
	}
	return score(target, cmp, pattern, best, best+m, withPos), true
}

// PrefixMatch reports whether text begins with pattern.
func PrefixMatch(text string, pattern []rune, caseSensitive, withPos bool) (Result, bool) {
	if len(pattern) == 0 {
		return Result{}, true
	}
	target := []rune(text)
	if len(pattern) > len(target) {
		return Result{}, false
	}
	cmp := lowerRunes(target, caseSensitive)
	if !equalAt(cmp, pattern, 0) {
		return Result{}, false
	}
	return score(target, cmp, pattern, 0, len(pattern), withPos), true
}

// SuffixMatch reports whether text ends with pattern.
func SuffixMatch(text string, pattern []rune, caseSensitive, withPos bool) (Result, bool) {
	if len(pattern) == 0 {
		return Result{}, true
	}
	target := []rune(text)
	n, m := len(target), len(pattern)
	if m > n {
		return Result{}, false
	}
	cmp := lowerRunes(target, caseSensitive)
	start := n - m
	if !equalAt(cmp, pattern, start) {
		return Result{}, false
	}
	return score(target, cmp, pattern, start, n, withPos), true
}

// EqualMatch reports whether text equals pattern exactly.
func EqualMatch(text string, pattern []rune, caseSensitive, withPos bool) (Result, bool) {
	target := []rune(text)
	if len(target) != len(pattern) {
		return Result{}, false
	}
	if len(pattern) == 0 {
		return Result{}, true
	}
	cmp := lowerRunes(target, caseSensitive)
	if !equalAt(cmp, pattern, 0) {
		return Result{}, false
	}
	return score(target, cmp, pattern, 0, len(pattern), withPos), true
}
