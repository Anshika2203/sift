package algo

// MatchV2 is an optimal fuzzy matcher. Unlike the greedy Match (v1), which
// scores the first subsequence alignment it finds, MatchV2 runs a
// Smith-Waterman-style dynamic program over the match window and returns the
// highest-scoring alignment. It is a little slower but ranks better.
//
// The scoring mirrors Match's: a base reward per matched rune, position
// bonuses (word boundary / camelCase), gap penalties, and a consecutive-run
// bonus. The important subtlety is that runes inside a consecutive run inherit
// the bonus of the run's first rune, so e.g. "foob" ranks "foobar" above
// "foo-bar".
//
// pattern must be lowercased when caseSensitive is false.
func MatchV2(text string, pattern []rune, caseSensitive, withPos bool) (Result, bool) {
	if len(pattern) == 0 {
		return Result{}, true
	}
	target := []rune(text)
	n, m := len(target), len(pattern)
	if m > n {
		return Result{}, false
	}
	cmp := lowerRunes(target, caseSensitive)

	// Confirm the pattern is a subsequence at all.
	pidx := 0
	for i := 0; i < n && pidx < m; i++ {
		if cmp[i] == pattern[pidx] {
			pidx++
		}
	}
	if pidx != m {
		return Result{}, false
	}

	// The DP window runs from the first occurrence of the first pattern rune to
	// the last occurrence of the last pattern rune — wide enough to contain
	// every possible alignment, so the optimum is never missed.
	first := 0
	for first < n && cmp[first] != pattern[0] {
		first++
	}
	last := n - 1
	for last >= 0 && cmp[last] != pattern[m-1] {
		last--
	}
	if first > last {
		return Result{}, false
	}
	width := last - first + 1

	// Position bonus for every column in the window.
	bonus := make([]int, width)
	prev := classWhite
	if first > 0 {
		prev = classOf(target[first-1])
	}
	for off := 0; off < width; off++ {
		c := classOf(target[first+off])
		bonus[off] = int(bonusFor(prev, c))
		prev = c
	}

	// H = best score, C = length of the consecutive run ending at this cell.
	H := make([]int, m*width)
	C := make([]int, m*width)
	maxScore, maxCol := 0, 0

	for i := 0; i < m; i++ {
		row := i * width
		pc := pattern[i]
		inGap := false
		for off := 0; off < width; off++ {
			// Gap path: carry the value from the left, paying a gap penalty.
			gap := 0
			if off > 0 {
				if inGap {
					gap = H[row+off-1] + scoreGapExtend
				} else {
					gap = H[row+off-1] + scoreGapStart
				}
			}

			// Match path: come from the diagonal.
			s1, matched, cons := 0, false, 0
			if cmp[first+off] == pc {
				diag, valid := 0, true
				if i > 0 {
					if off > 0 {
						diag = H[row-width+off-1]
					} else {
						valid = false // pattern[i>0] cannot match the first column
					}
				}
				if valid {
					b := bonus[off]
					prevCons := 0
					if i > 0 && off > 0 {
						prevCons = C[row-width+off-1]
					}
					cons = prevCons + 1
					if cons > 1 {
						start := off - (cons - 1)
						if start < 0 {
							start = 0
						}
						fb := bonus[start] // bonus of the run's first rune
						if b >= bonusBoundary && b > fb {
							cons = 1 // a stronger boundary starts a fresh run
						} else {
							if b < bonusConsecutive {
								b = bonusConsecutive
							}
							if b < fb {
								b = fb
							}
						}
					}
					add := b
					if i == 0 {
						add = b * firstCharMult
					}
					s1 = diag + scoreMatch + add
					matched = true
				}
			}

			best := gap
			if matched && s1 >= gap {
				best = s1
				C[row+off] = cons
				inGap = false
			} else {
				if best < 0 {
					best = 0
				}
				inGap = best > 0
			}
			if best < 0 {
				best = 0
			}
			H[row+off] = best

			if i == m-1 && best > maxScore {
				maxScore, maxCol = best, off
			}
		}
	}

	res := Result{Score: maxScore}
	if withPos {
		pos := make([]int, 0, m)
		i, off := m-1, maxCol
		for i >= 0 && off >= 0 {
			if C[i*width+off] > 0 {
				pos = append(pos, first+off)
				i--
				off--
			} else {
				off--
			}
		}
		for l, r := 0, len(pos)-1; l < r; l, r = l+1, r-1 {
			pos[l], pos[r] = pos[r], pos[l]
		}
		res.Positions = pos
	}
	return res, true
}
