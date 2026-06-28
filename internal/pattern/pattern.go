// Package pattern parses sift's extended search syntax and evaluates it against
// items.
//
// A query is split on spaces into terms that are ANDed together: an item must
// satisfy every term. A leading or trailing marker changes how a term matches:
//
//	foo      fuzzy match (the default)
//	'foo     exact substring match
//	^foo     prefix match
//	foo$     suffix match
//	^foo$    exact equality
//	!foo     inverse: the item must NOT contain foo
//
// The inverse marker combines with the others: !^foo, !foo$, !'foo. Matching is
// case-insensitive for a term unless that term contains an uppercase letter
// ("smart case").
package pattern

import (
	"strings"
	"unicode"

	"github.com/Anshika2203/sift/internal/algo"
)

type termType int

const (
	termFuzzy termType = iota
	termExact
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

// Pattern is a parsed query.
type Pattern struct {
	terms    []term
	sortable bool // false when every term is inverse (nothing to rank by)
	empty    bool
}

func fnFor(t termType) matchFunc {
	switch t {
	case termExact:
		return algo.ExactMatch
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
func Parse(query string) *Pattern {
	fields := strings.Fields(query)
	p := &Pattern{}
	if len(fields) == 0 {
		p.empty = true
		return p
	}

	for _, tok := range fields {
		typ := termFuzzy
		inv := false
		text := tok

		// Inverse marker. After stripping "!", the remaining markers still apply
		// (so "!^foo" is an inverse prefix match).
		if strings.HasPrefix(text, "!") {
			inv = true
			typ = termExact // a bare "!foo" is an inverse exact-substring term
			text = text[1:]
		}
		if text == "" {
			continue
		}

		hasCaret := strings.HasPrefix(text, "^")
		hasDollar := strings.HasSuffix(text, "$") && text != "$"
		switch {
		case hasCaret && hasDollar:
			typ = termEqual
			text = text[1 : len(text)-1]
		case hasCaret:
			typ = termPrefix
			text = text[1:]
		case hasDollar:
			typ = termSuffix
			text = text[:len(text)-1]
		case strings.HasPrefix(text, "'"):
			typ = termExact
			text = text[1:]
		}
		if text == "" {
			continue
		}

		caseSensitive := hasUpper(text)
		runes := []rune(text)
		if !caseSensitive {
			for i, r := range runes {
				runes[i] = toLower(r)
			}
		}

		p.terms = append(p.terms, term{
			inv:           inv,
			caseSensitive: caseSensitive,
			text:          runes,
			fn:            fnFor(typ),
		})
		if !inv {
			p.sortable = true
		}
	}

	if len(p.terms) == 0 {
		p.empty = true
	}
	return p
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
	for _, t := range p.terms {
		res, ok := t.fn(text, t.text, t.caseSensitive, withPos)
		if t.inv {
			if ok {
				return 0, nil, false // a forbidden term matched
			}
			continue
		}
		if !ok {
			return 0, nil, false
		}
		total += res.Score
		if withPos && len(res.Positions) > 0 {
			positions = append(positions, res.Positions...)
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
