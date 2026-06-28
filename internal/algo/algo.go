// Package algo implements sift's fuzzy matching and scoring.
//
// A "fuzzy" match means every rune of the pattern appears in the target in the
// same order, but not necessarily contiguously. Among the (possibly many) ways
// a pattern can be aligned to a target, we want the one a human would consider
// "best", so each alignment is scored and the highest score wins.
//
// The scoring philosophy (closely following the lessons popularised by fzf):
//
//   - Matches at the start of a word are worth more than matches in the middle,
//     so "ff" prefers "fuzzy-finder" over "fuzzyfinder".
//   - The first pattern rune is the most intentional, so its position bonus is
//     amplified.
//   - Long stretches between matched runes are penalised (gap penalty), so the
//     tool stays a fuzzy finder rather than an acronym finder.
//   - Consecutive matches earn a small bonus so "foob" prefers "foobar" over
//     "foo-bar".
//
// The matcher here is a greedy two-pass aligner (forward scan to locate the
// match, backward scan to tighten it) followed by a single linear scoring pass.
// It runs in O(n) per target and produces both a score and the matched rune
// positions for highlighting.
package algo

import "unicode"

// Scoring constants. These were hand-tuned so that a word-boundary bonus is
// roughly cancelled out once the gap between two matched runes grows beyond a
// typical word length.
const (
	scoreMatch       = 16 // base reward for matching a pattern rune
	scoreGapStart    = -3 // penalty for opening a gap between matched runes
	scoreGapExtend   = -1 // penalty for each additional gap rune
	bonusBoundary    = 8  // match sits at the start of a word
	bonusCamel       = 7  // camelCase hump or letter->digit transition
	bonusConsecutive = 4  // minimum bonus for a run of consecutive matches
	firstCharMult    = 2  // the first pattern rune's bonus is multiplied by this
)

// Result holds the outcome of a successful match.
type Result struct {
	Score     int   // higher is a better match
	Positions []int // rune indices in the target that were matched (nil if not requested)
}

// charClass categorises a rune for the purpose of computing boundary bonuses.
type charClass int

const (
	classWhite   charClass = iota // whitespace (also the virtual char before the string)
	classNonWord                  // punctuation and symbols
	classDelim                    // path/field delimiters: / \ . , : ; | _ -
	classLower                    // lowercase letter
	classUpper                    // uppercase letter
	classDigit                    // 0-9
	classLetter                   // any other (non-ASCII) letter
)

const delimiters = "/\\.,:;|_-"

func classOf(r rune) charClass {
	switch {
	case r >= 'a' && r <= 'z':
		return classLower
	case r >= 'A' && r <= 'Z':
		return classUpper
	case r >= '0' && r <= '9':
		return classDigit
	case r == ' ' || r == '\t' || r == '\n' || r == '\v' || r == '\f' || r == '\r':
		return classWhite
	}
	for _, d := range delimiters {
		if r == d {
			return classDelim
		}
	}
	if r > unicode.MaxASCII {
		switch {
		case unicode.IsLower(r):
			return classLower
		case unicode.IsUpper(r):
			return classUpper
		case unicode.IsNumber(r):
			return classDigit
		case unicode.IsLetter(r):
			return classLetter
		case unicode.IsSpace(r):
			return classWhite
		}
	}
	return classNonWord
}

// bonusFor returns the position bonus earned by a rune of class cur that follows
// a rune of class prev.
func bonusFor(prev, cur charClass) int {
	// Transition into a "word" rune from a non-word/boundary rune: word start.
	if cur >= classLower {
		switch prev {
		case classWhite:
			return bonusBoundary + 2 // strongest: start of string or after whitespace
		case classDelim:
			return bonusBoundary + 1 // after a delimiter such as '/' or '_'
		case classNonWord:
			return bonusBoundary
		}
	}
	// camelCase hump (lower -> upper) or a letter followed by a digit.
	if prev == classLower && cur == classUpper {
		return bonusCamel
	}
	if prev != classDigit && cur == classDigit {
		return bonusCamel
	}
	return 0
}

func toLower(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + 32
	}
	if r > unicode.MaxASCII {
		return unicode.ToLower(r)
	}
	return r
}

// Match reports whether pattern fuzzy-matches text and, if so, returns its
// score and (when withPos is true) the matched rune positions.
//
// pattern must already be lowercased by the caller when caseSensitive is false.
// An empty pattern matches everything with a score of 0.
func Match(text string, pattern []rune, caseSensitive, withPos bool) (Result, bool) {
	if len(pattern) == 0 {
		return Result{}, true
	}

	target := []rune(text)
	n := len(target)
	m := len(pattern)
	if m > n {
		return Result{}, false
	}

	// Build a comparison copy (lowercased unless the search is case-sensitive).
	cmp := make([]rune, n)
	if caseSensitive {
		copy(cmp, target)
	} else {
		for i, r := range target {
			cmp[i] = toLower(r)
		}
	}

	// Pass 1: forward scan to find the earliest subsequence match.
	sidx, eidx, pidx := -1, -1, 0
	for i := 0; i < n; i++ {
		if cmp[i] == pattern[pidx] {
			if sidx < 0 {
				sidx = i
			}
			pidx++
			if pidx == m {
				eidx = i + 1
				break
			}
		}
	}
	if eidx < 0 {
		return Result{}, false // pattern is not a subsequence of text
	}

	// Pass 2: backward scan to pull the start forward, yielding the shortest
	// span that still contains the pattern (a tighter, higher-scoring match).
	pidx = m - 1
	for i := eidx - 1; i >= sidx; i-- {
		if cmp[i] == pattern[pidx] {
			pidx--
			if pidx < 0 {
				sidx = i
				break
			}
		}
	}

	return score(target, cmp, pattern, sidx, eidx, withPos), true
}

// score walks the matched span [sidx, eidx) once and accumulates the final
// score, also collecting matched positions when requested.
func score(target, cmp, pattern []rune, sidx, eidx int, withPos bool) Result {
	total := 0
	pidx := 0
	inGap := false
	consecutive := 0
	firstBonus := 0

	prevClass := classWhite
	if sidx > 0 {
		prevClass = classOf(target[sidx-1])
	}

	var positions []int
	if withPos {
		positions = make([]int, 0, len(pattern))
	}

	for i := sidx; i < eidx && pidx < len(pattern); i++ {
		cur := classOf(target[i])
		if cmp[i] == pattern[pidx] {
			if withPos {
				positions = append(positions, i)
			}
			total += scoreMatch

			b := bonusFor(prevClass, cur)
			if consecutive == 0 {
				firstBonus = b
			} else {
				// Inside a consecutive run, every rune earns at least the run's
				// opening bonus, but a strong boundary can raise the bar.
				if b >= bonusBoundary && b > firstBonus {
					firstBonus = b
				}
				b = max(b, firstBonus, bonusConsecutive)
			}

			if pidx == 0 {
				total += b * firstCharMult
			} else {
				total += b
			}

			inGap = false
			consecutive++
			pidx++
		} else {
			if inGap {
				total += scoreGapExtend
			} else {
				total += scoreGapStart
			}
			inGap = true
			consecutive = 0
			firstBonus = 0
		}
		prevClass = cur
	}

	return Result{Score: total, Positions: positions}
}
