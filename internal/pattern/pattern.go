// Package pattern parses sift's extended search syntax and evaluates it against
// items.
//
// A query is split on spaces into terms. Terms are ANDed together (an item must
// satisfy every term), except that consecutive terms joined by a bare "|" form
// an OR group ("term set") — the set is satisfied if any of its terms match.
// Markers change how a term matches:
//
//	foo      fuzzy match (the default)
//	'foo     exact substring match
//	'foo'    exact match at word boundaries
//	^foo     prefix match
//	foo$     suffix match
//	^foo$    exact equality
//	!foo     inverse: the item must NOT contain foo
//	a | b    OR: matches a or b
//
// The inverse marker combines with the others: !^foo, !foo$, !'foo. With
// --exact the default flips: bare terms become exact and 'foo becomes fuzzy.
// Case sensitivity follows the Case option (smart case by default).
package pattern

import (
	"strings"
	"unicode"

	"github.com/Anshika2203/sift/internal/algo"
)

// Case controls case sensitivity.
type Case int

const (
	CaseSmart   Case = iota // case-insensitive unless the term has an uppercase letter
	CaseIgnore              // always case-insensitive
	CaseRespect             // always case-sensitive
)

// Options configures how a query string is parsed.
type Options struct {
	Fuzzy bool // default term type is fuzzy (true) or exact (false, i.e. --exact)
	Case  Case
}

type termType int

const (
	termFuzzy termType = iota
	termExact
	termBoundary
	termPrefix
	termSuffix
	termEqual
)

type matchFunc func(text string, pattern []rune, caseSensitive, withPos bool) (algo.Result, bool)

type term struct {
	inv           bool
	caseSensitive bool
	text          []rune
	fn            matchFunc
}

// Pattern is a parsed query: an AND of term sets, each an OR of terms.
type Pattern struct {
	termSets [][]term
	sortable bool
	empty    bool
}

func fnFor(t termType) matchFunc {
	switch t {
	case termExact:
		return algo.ExactMatch
	case termBoundary:
		return algo.ExactBoundaryMatch
	case termPrefix:
		return algo.PrefixMatch
	case termSuffix:
		return algo.SuffixMatch
	case termEqual:
		return algo.EqualMatch
	default:
		return algo.Match
	}
}

// Parse builds a Pattern from a raw query string.
func Parse(query string, opts Options) *Pattern {
	tokens := strings.Fields(query)
	p := &Pattern{}

	var set []term
	switchSet := false // next term starts a new set
	afterBar := false  // previous token was "|", so the next term joins this set

	for _, tok := range tokens {
		if len(set) > 0 && !afterBar && tok == "|" {
			switchSet = false
			afterBar = true
			continue
		}
		afterBar = false

		t, ok := parseTerm(tok, opts)
		if !ok {
			continue
		}
		if switchSet {
			p.termSets = append(p.termSets, set)
			set = nil
		}
		set = append(set, t)
		switchSet = true
		if !t.inv {
			p.sortable = true
		}
	}
	if len(set) > 0 {
		p.termSets = append(p.termSets, set)
	}
	if len(p.termSets) == 0 {
		p.empty = true
	}
	return p
}

func parseTerm(tok string, opts Options) (term, bool) {
	typ := termFuzzy
	if !opts.Fuzzy {
		typ = termExact
	}
	inv := false
	text := tok

	if strings.HasPrefix(text, "!") {
		inv = true
		typ = termExact
		text = text[1:]
	}
	if text == "" {
		return term{}, false
	}

	// Suffix marker (but a lone "$" is a literal).
	if text != "$" && strings.HasSuffix(text, "$") {
		typ = termSuffix
		text = text[:len(text)-1]
	}

	switch {
	case len(text) >= 2 && strings.HasPrefix(text, "'") && strings.HasSuffix(text, "'"):
		typ = termBoundary
		text = text[1 : len(text)-1]
	case strings.HasPrefix(text, "'"):
		// A leading quote flips exactness.
		if opts.Fuzzy && !inv {
			typ = termExact
		} else {
			typ = termFuzzy
		}
		text = text[1:]
	case strings.HasPrefix(text, "^"):
		if typ == termSuffix {
			typ = termEqual
		} else {
			typ = termPrefix
		}
		text = text[1:]
	}

	if text == "" {
		return term{}, false
	}

	caseSensitive := false
	switch opts.Case {
	case CaseRespect:
		caseSensitive = true
	case CaseIgnore:
		caseSensitive = false
	default:
		caseSensitive = hasUpper(text)
	}

	runes := []rune(text)
	if !caseSensitive {
		for i, r := range runes {
			runes[i] = toLower(r)
		}
	}
	return term{inv: inv, caseSensitive: caseSensitive, text: runes, fn: fnFor(typ)}, true
}

// IsEmpty reports whether the pattern has no terms (and so matches everything).
func (p *Pattern) IsEmpty() bool { return p.empty }

// Sortable reports whether matches should be ranked by score. It is false when
// the query consists solely of inverse terms, where there is nothing to rank.
func (p *Pattern) Sortable() bool { return p.sortable }

// Match evaluates the pattern against text. On success it returns the summed
// score and, when withPos is set, the union of matched rune positions.
func (p *Pattern) Match(text string, withPos bool) (int, []int, bool) {
	if p.empty {
		return 0, nil, true
	}
	total := 0
	var positions []int

	for _, set := range p.termSets {
		matched := false
		setScore := 0
		var setPos []int

		for _, t := range set {
			res, ok := t.fn(text, t.text, t.caseSensitive, withPos)
			if ok {
				if t.inv {
					continue // a forbidden term matched; this term can't satisfy the set
				}
				matched = true
				setScore = res.Score
				if withPos {
					setPos = res.Positions
				}
				break
			} else if t.inv {
				matched = true // inverse term absent -> set satisfied
				setScore = 0
				setPos = nil
				continue
			}
		}
		if !matched {
			return 0, nil, false
		}
		total += setScore
		if withPos && len(setPos) > 0 {
			positions = append(positions, setPos...)
		}
	}
	return total, positions, true
}

func hasUpper(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
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
