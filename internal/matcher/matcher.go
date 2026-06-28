// Package matcher runs a parsed query across a whole list of items and returns
// the matches ranked best-first.
package matcher

import (
	"runtime"
	"sort"
	"sync"

	"github.com/Anshika2203/sift/internal/pattern"
	"github.com/Anshika2203/sift/internal/tokenizer"
)

// Match is a single item that matched the query.
type Match struct {
	Index     int    // position of the item in the original input
	Text      string // text shown in the UI (after --with-nth), and used for length
	Output    string // original line, printed on accept
	Score     int    // match score (higher is better)
	Positions []int  // matched rune indices into Text, for highlighting
}

// Tiebreak selects how equally-scored matches are ordered.
type Tiebreak int

const (
	TieLength Tiebreak = iota // shorter item first
	TieBegin                  // earlier first matched position first
	TieEnd                    // later last matched position first
	TieIndex                  // earlier in the input first
)

// Options configures a filter pass.
type Options struct {
	Fuzzy   bool         // default term type fuzzy (true) or exact (--exact)
	Case    pattern.Case // case-sensitivity policy
	Sort    bool         // rank by score (false keeps input order, i.e. --no-sort)
	WithPos bool         // compute matched positions (for UI highlighting)
	AlgoV2  bool         // use the optimal v2 fuzzy scorer

	Tiebreak []Tiebreak // tie-break order (default {TieLength})

	Delimiter  tokenizer.Delimiter
	Nth        []tokenizer.Range // --nth: limit search to these fields
	WithNth    []tokenizer.Range // --with-nth: display only these fields
	HasNth     bool
	HasWithNth bool
}

// display computes the shown text for a line (applying --with-nth).
func (o Options) display(line string) string {
	if o.HasWithNth {
		return tokenizer.Join(line, o.Delimiter, o.WithNth)
	}
	return line
}

// Filter ranks items against query and returns the matches. With Sort enabled
// (and a sortable query) the best score comes first; otherwise input order is
// preserved. An empty query returns every item (display-transformed) in order.
func Filter(items []string, query string, opts Options) []Match {
	p := pattern.Parse(query, pattern.Options{Fuzzy: opts.Fuzzy, Case: opts.Case, AlgoV2: opts.AlgoV2})

	tie := opts.Tiebreak
	if len(tie) == 0 {
		tie = []Tiebreak{TieLength}
	}
	needPos := opts.WithPos
	for _, tb := range tie {
		if tb == TieBegin || tb == TieEnd {
			needPos = true
		}
	}

	if p.IsEmpty() {
		out := make([]Match, len(items))
		for i, t := range items {
			out[i] = Match{Index: i, Text: opts.display(t), Output: t}
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
		go func(start, end int) {
			defer wg.Done()
			var local []Match
			for i := start; i < end; i++ {
				raw := items[i]
				disp := opts.display(raw)

				search := disp
				var posMap []int
				if opts.HasNth {
					search, posMap = tokenizer.Select(disp, opts.Delimiter, opts.Nth)
				}

				sc, pos, ok := p.Match(search, needPos)
				if !ok {
					continue
				}

				var mapped []int
				if needPos && len(pos) > 0 {
					if posMap == nil {
						mapped = pos
					} else {
						mapped = make([]int, 0, len(pos))
						for _, pp := range pos {
							if pp < len(posMap) && posMap[pp] >= 0 {
								mapped = append(mapped, posMap[pp])
							}
						}
					}
				}
				local = append(local, Match{Index: i, Text: disp, Output: raw, Score: sc, Positions: mapped})
			}
			partials[start/chunk] = local
		}(start, end)
	}
	wg.Wait()

	var matches []Match
	for _, pr := range partials {
		matches = append(matches, pr...)
	}

	if opts.Sort && p.Sortable() {
		sort.SliceStable(matches, func(a, b int) bool {
			return less(matches[a], matches[b], tie)
		})
	}
	return matches
}

func firstPos(m Match) int {
	if len(m.Positions) == 0 {
		return 1 << 30
	}
	lo := m.Positions[0]
	for _, p := range m.Positions[1:] {
		if p < lo {
			lo = p
		}
	}
	return lo
}

func lastPos(m Match) int {
	hi := -1
	for _, p := range m.Positions {
		if p > hi {
			hi = p
		}
	}
	return hi
}

func less(a, b Match, tie []Tiebreak) bool {
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	for _, tb := range tie {
		switch tb {
		case TieLength:
			if la, lb := len([]rune(a.Text)), len([]rune(b.Text)); la != lb {
				return la < lb
			}
		case TieBegin:
			if fa, fb := firstPos(a), firstPos(b); fa != fb {
				return fa < fb
			}
		case TieEnd:
			if ea, eb := lastPos(a), lastPos(b); ea != eb {
				return ea > eb
			}
		case TieIndex:
			if a.Index != b.Index {
				return a.Index < b.Index
			}
		}
	}
	return a.Index < b.Index
}
