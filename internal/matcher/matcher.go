// Package matcher runs a parsed query across a whole list of items and returns
// the matches ranked best-first.
package matcher

import (
	"runtime"
	"sort"
	"sync"

	"github.com/Anshika2203/sift/internal/pattern"
)

// Match is a single item that matched the query.
type Match struct {
	Index     int    // position of the item in the original input
	Text      string // the item's text
	Score     int    // match score (higher is better)
	Positions []int  // matched rune indices, for highlighting
}

// Options configures a filter pass.
type Options struct {
	Fuzzy   bool         // default term type fuzzy (true) or exact (--exact)
	Case    pattern.Case // case-sensitivity policy
	Sort    bool         // rank by score (false keeps input order, i.e. --no-sort)
	WithPos bool         // compute matched positions (for UI highlighting)
}

// Filter ranks items against query and returns the matches. With Sort enabled
// (and a sortable query) the best score comes first; otherwise input order is
// preserved. An empty query returns every item in input order.
func Filter(items []string, query string, opts Options) []Match {
	p := pattern.Parse(query, pattern.Options{Fuzzy: opts.Fuzzy, Case: opts.Case})

	if p.IsEmpty() {
		out := make([]Match, len(items))
		for i, t := range items {
			out[i] = Match{Index: i, Text: t}
		}
		return out
	}

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
				if sc, pos, ok := p.Match(items[i], opts.WithPos); ok {
					local = append(local, Match{
						Index:     i,
						Text:      items[i],
						Score:     sc,
						Positions: pos,
					})
				}
			}
			partials[w] = local
		}(w, start, end)
	}
	wg.Wait()

	var matches []Match
	for _, pr := range partials {
		matches = append(matches, pr...)
	}

	// Rank by score (then shorter item, then input order). Skipped when sorting
	// is disabled or the query is all-inverse; the worker layout already yields
	// ascending input order.
	if opts.Sort && p.Sortable() {
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
	}

	return matches
}
