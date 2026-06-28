// Package matcher runs the fuzzy algorithm across a whole list of items and
// returns the matches ranked best-first.
package matcher

import (
	"runtime"
	"sort"
	"sync"
	"unicode"

	"github.com/Anshika2203/sift/internal/algo"
)

// Match is a single item that matched the query.
type Match struct {
	Index     int    // position of the item in the original input
	Text      string // the item's text
	Score     int    // match score (higher is better)
	Positions []int  // matched rune indices, for highlighting
}

// smartCase reports whether the query should be treated case-sensitively.
// Like fzf, the search is case-insensitive unless the query contains an
// uppercase letter ("smart case").
func smartCase(query string) bool {
	for _, r := range query {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

// Filter ranks items against query and returns the matches, best score first.
// An empty query returns every item in its original order.
//
// withPos controls whether matched positions are computed (needed for the
// interactive UI's highlighting, skippable for plain filter output).
func Filter(items []string, query string, withPos bool) []Match {
	caseSensitive := smartCase(query)

	pattern := []rune(query)
	if !caseSensitive {
		for i, r := range pattern {
			if r >= 'A' && r <= 'Z' {
				pattern[i] = r + 32
			} else if r > unicode.MaxASCII {
				pattern[i] = unicode.ToLower(r)
			}
		}
	}

	// Empty query: pass everything through unranked.
	if len(pattern) == 0 {
		out := make([]Match, len(items))
		for i, t := range items {
			out[i] = Match{Index: i, Text: t}
		}
		return out
	}

	// Fan the work out across CPUs: each worker scans a contiguous slice of the
	// input and produces its own list of matches, which we merge afterwards.
	workers := runtime.NumCPU()
	if workers > len(items) {
		workers = len(items)
	}
	if workers < 1 {
		workers = 1
	}

	partials := make([][]Match, workers)
	chunk := (len(items) + workers - 1) / workers

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		start := w * chunk
		if start >= len(items) {
			break
		}
		end := start + chunk
		if end > len(items) {
			end = len(items)
		}
		wg.Add(1)
		go func(w, start, end int) {
			defer wg.Done()
			var local []Match
			for i := start; i < end; i++ {
				if res, ok := algo.Match(items[i], pattern, caseSensitive, withPos); ok {
					local = append(local, Match{
						Index:     i,
						Text:      items[i],
						Score:     res.Score,
						Positions: res.Positions,
					})
				}
			}
			partials[w] = local
		}(w, start, end)
	}
	wg.Wait()

	var matches []Match
	for _, p := range partials {
		matches = append(matches, p...)
	}

	// Rank: highest score first; ties go to the shorter item (a tighter match),
	// and remaining ties preserve the original input order for stability.
	sort.SliceStable(matches, func(a, b int) bool {
		ma, mb := matches[a], matches[b]
		if ma.Score != mb.Score {
			return ma.Score > mb.Score
		}
		if len(ma.Text) != len(mb.Text) {
			return len(ma.Text) < len(mb.Text)
		}
		return ma.Index < mb.Index
	})

	return matches
}
