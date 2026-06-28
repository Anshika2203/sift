// Package ansi parses ANSI SGR (color/style) escape sequences out of a string,
// returning the plain text plus styled spans for rendering. It backs --ansi.
package ansi

import (
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// Span styles a half-open run of rune indices [Start, End) in the plain text.
type Span struct {
	Start, End int
	Style      tcell.Style
}

// Parse strips SGR escape sequences from s, returning the plain text and the
// styled spans (only runs whose style differs from the default are recorded).
func Parse(s string) (string, []Span) {
	runes := []rune(s)
	plain := make([]rune, 0, len(runes))
	var spans []Span

	cur := tcell.StyleDefault
	styled := false
	spanStart := 0

	flush := func() {
		if styled && len(plain) > spanStart {
			spans = append(spans, Span{spanStart, len(plain), cur})
		}
	}

	for i := 0; i < len(runes); {
		r := runes[i]
		if r == 0x1b && i+1 < len(runes) && runes[i+1] == '[' {
			j := i + 2
			for j < len(runes) && !(runes[j] >= '@' && runes[j] <= '~') {
				j++
			}
			if j < len(runes) {
				if runes[j] == 'm' { // SGR
					flush()
					cur, styled = applySGR(cur, string(runes[i+2:j]))
					spanStart = len(plain)
				}
				i = j + 1
				continue
			}
		}
		plain = append(plain, r)
		i++
	}
	flush()
	return string(plain), spans
}

func applySGR(st tcell.Style, params string) (tcell.Style, bool) {
	if params == "" || params == "0" {
		return tcell.StyleDefault, false
	}
	parts := strings.Split(params, ";")
	styled := st != tcell.StyleDefault
	for k := 0; k < len(parts); k++ {
		n, err := strconv.Atoi(parts[k])
		if err != nil {
			continue
		}
		switch {
		case n == 0:
			st, styled = tcell.StyleDefault, false
		case n == 1:
			st, styled = st.Bold(true), true
		case n == 2:
			st, styled = st.Dim(true), true
		case n == 3:
			st, styled = st.Italic(true), true
		case n == 4:
			st, styled = st.Underline(true), true
		case n == 7:
			st, styled = st.Reverse(true), true
		case n >= 30 && n <= 37:
			st, styled = st.Foreground(tcell.PaletteColor(n-30)), true
		case n >= 90 && n <= 97:
			st, styled = st.Foreground(tcell.PaletteColor(n-90+8)), true
		case n >= 40 && n <= 47:
			st, styled = st.Background(tcell.PaletteColor(n-40)), true
		case n >= 100 && n <= 107:
			st, styled = st.Background(tcell.PaletteColor(n-100+8)), true
		case n == 39:
			st = st.Foreground(tcell.ColorDefault)
		case n == 49:
			st = st.Background(tcell.ColorDefault)
		case n == 38 || n == 48:
			fg := n == 38
			if k+1 < len(parts) {
				mode, _ := strconv.Atoi(parts[k+1])
				if mode == 5 && k+2 < len(parts) {
					c, _ := strconv.Atoi(parts[k+2])
					col := tcell.PaletteColor(c)
					if fg {
						st = st.Foreground(col)
					} else {
						st = st.Background(col)
					}
					styled, k = true, k+2
				} else if mode == 2 && k+4 < len(parts) {
					rr, _ := strconv.Atoi(parts[k+2])
					gg, _ := strconv.Atoi(parts[k+3])
					bb, _ := strconv.Atoi(parts[k+4])
					col := tcell.NewRGBColor(int32(rr), int32(gg), int32(bb))
					if fg {
						st = st.Foreground(col)
					} else {
						st = st.Background(col)
					}
					styled, k = true, k+4
				}
			}
		}
	}
	return st, styled
}
