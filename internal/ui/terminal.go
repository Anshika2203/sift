// Package ui implements sift's interactive, full-screen fuzzy-finder interface.
package ui

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"

	"github.com/Anshika2203/sift/internal/ansi"
	"github.com/Anshika2203/sift/internal/matcher"
	"github.com/Anshika2203/sift/internal/pattern"
	"github.com/Anshika2203/sift/internal/tokenizer"
)

// Options configures an interactive session.
type Options struct {
	Prompt  string       // text shown before the query, e.g. "> "
	Query   string       // initial query
	Multi   bool         // allow selecting multiple items with Tab
	Preview string       // command template; placeholders like {}, {q}, {1} expand
	Header  []string     // fixed header lines shown above the list
	Expect  []string     // extra keys that accept and report which key was pressed
	Fuzzy   bool         // default term type fuzzy (true) or exact (--exact)
	Case    pattern.Case // case-sensitivity policy
	Sort    bool         // rank by score (false keeps input order)
	AlgoV2  bool         // use the optimal v2 fuzzy scorer

	Tiebreak   []matcher.Tiebreak
	Delimiter  tokenizer.Delimiter
	Nth        []tokenizer.Range
	WithNth    []tokenizer.Range
	HasNth     bool
	HasWithNth bool

	PreviewWindow string        // e.g. "right,50%", "up,40%", "hidden"
	Colors        [][]ansi.Span // per-item ANSI styling (--ansi), indexed by item index
}

// Result is what the user picked.
type Result struct {
	Selected []string // chosen items (empty when aborted)
	Aborted  bool     // true if the user pressed Esc / Ctrl-C
	Query    string   // the final query string
	Key      string   // the expect key that was pressed (empty for plain Enter)
}

// Run shows the finder over items and blocks until the user accepts or aborts.
func Run(items []string, opts Options) (Result, error) {
	if opts.Prompt == "" {
		opts.Prompt = "> "
	}

	screen, err := tcell.NewScreen()
	if err != nil {
		return Result{}, err
	}
	if err := screen.Init(); err != nil {
		return Result{}, err
	}
	defer screen.Fini()
	screen.SetStyle(tcell.StyleDefault)

	expect := make(map[string]bool, len(opts.Expect))
	for _, k := range opts.Expect {
		expect[k] = true
	}

	pv, hidden := parsePreviewWindow(opts.PreviewWindow)

	m := &model{
		screen:        screen,
		opts:          opts,
		items:         items,
		selected:      map[int]bool{},
		query:         []rune(opts.Query),
		expect:        expect,
		pv:            pv,
		previewHidden: hidden,
	}
	m.recompute()
	m.refreshPreview()
	m.draw()

	for {
		switch ev := screen.PollEvent().(type) {
		case *tcell.EventResize:
			screen.Sync()
			m.draw()
		case *previewEvent:
			if ev.gen == m.previewGen {
				m.previewLines = ev.lines
				m.draw()
			}
		case *tcell.EventKey:
			if done, res := m.handleKey(ev); done {
				return res, nil
			}
			m.draw()
		}
	}
}

// previewWindow describes the parsed --preview-window layout.
type previewWindow struct {
	pos     string // up | down | left | right
	pct     int
	abs     int
	percent bool
}

// model holds all interactive state.
type model struct {
	screen   tcell.Screen
	opts     Options
	items    []string
	matches  []matcher.Match
	query    []rune
	cursor   int             // index into matches (0 = best match, at the top)
	offset   int             // first visible match (scroll position)
	selected map[int]bool    // keyed by original item index
	expect   map[string]bool // keys that accept and report themselves

	pv            previewWindow
	previewHidden bool
	previewGen    int
	previewKey    string // last expanded preview command (cache guard)
	previewLines  []string
	previewOffset int
}

// previewEvent carries asynchronous preview output back to the event loop.
type previewEvent struct {
	t     time.Time
	gen   int
	lines []string
}

func (e *previewEvent) When() time.Time { return e.t }

func (m *model) recompute() {
	m.matches = matcher.Filter(m.items, string(m.query), matcher.Options{
		Fuzzy:      m.opts.Fuzzy,
		Case:       m.opts.Case,
		Sort:       m.opts.Sort,
		WithPos:    true,
		AlgoV2:     m.opts.AlgoV2,
		Tiebreak:   m.opts.Tiebreak,
		Delimiter:  m.opts.Delimiter,
		Nth:        m.opts.Nth,
		WithNth:    m.opts.WithNth,
		HasNth:     m.opts.HasNth,
		HasWithNth: m.opts.HasWithNth,
	})
	m.cursor = 0
	m.offset = 0
}

func (m *model) current() (matcher.Match, bool) {
	if m.cursor >= 0 && m.cursor < len(m.matches) {
		return m.matches[m.cursor], true
	}
	return matcher.Match{}, false
}

func (m *model) move(delta int) {
	if len(m.matches) == 0 {
		return
	}
	old := m.cursor
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.matches) {
		m.cursor = len(m.matches) - 1
	}
	if m.cursor != old {
		m.refreshPreview()
	}
}

func (m *model) previewScroll(delta int) {
	m.previewOffset += delta
	if m.previewOffset > len(m.previewLines)-1 {
		m.previewOffset = len(m.previewLines) - 1
	}
	if m.previewOffset < 0 {
		m.previewOffset = 0
	}
}

func (m *model) toggleSelect() {
	if cur, ok := m.current(); ok {
		if m.selected[cur.Index] {
			delete(m.selected, cur.Index)
		} else {
			m.selected[cur.Index] = true
		}
	}
}

func (m *model) selectedItems(cur matcher.Match) []string {
	if m.opts.Multi && len(m.selected) > 0 {
		var out []string
		for i := range m.items {
			if m.selected[i] {
				out = append(out, m.items[i])
			}
		}
		return out
	}
	return []string{cur.Output}
}

func (m *model) accept() Result {
	q := string(m.query)
	if items := m.selectedItems(matcher.Match{}); m.opts.Multi && len(m.selected) > 0 {
		return Result{Selected: items, Query: q}
	}
	if cur, ok := m.current(); ok {
		return Result{Selected: []string{cur.Output}, Query: q}
	}
	return Result{Aborted: true, Query: q}
}

func (m *model) handleKey(ev *tcell.EventKey) (bool, Result) {
	// Expect keys accept the selection and report which key was pressed.
	if len(m.expect) > 0 {
		if name := keyName(ev); name != "" && m.expect[name] {
			r := m.accept()
			r.Key = name
			return true, r
		}
	}

	// Preview navigation.
	if m.opts.Preview != "" {
		if ev.Key() == tcell.KeyCtrlO {
			m.previewHidden = !m.previewHidden
			return false, Result{}
		}
		if ev.Modifiers()&(tcell.ModShift|tcell.ModAlt) != 0 {
			switch ev.Key() {
			case tcell.KeyUp:
				m.previewScroll(-1)
				return false, Result{}
			case tcell.KeyDown:
				m.previewScroll(1)
				return false, Result{}
			case tcell.KeyPgUp:
				m.previewScroll(-10)
				return false, Result{}
			case tcell.KeyPgDn:
				m.previewScroll(10)
				return false, Result{}
			}
		}
	}

	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		return true, Result{Aborted: true, Query: string(m.query)}
	case tcell.KeyEnter:
		return true, m.accept()
	case tcell.KeyCtrlU:
		m.query = nil
		m.recompute()
		m.refreshPreview()
	case tcell.KeyCtrlW:
		m.query = deleteWord(m.query)
		m.recompute()
		m.refreshPreview()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(m.query) > 0 {
			m.query = m.query[:len(m.query)-1]
			m.recompute()
			m.refreshPreview()
		}
	case tcell.KeyDown, tcell.KeyCtrlN:
		m.move(1)
	case tcell.KeyUp, tcell.KeyCtrlP:
		m.move(-1)
	case tcell.KeyPgDn:
		m.move(m.listHeight())
	case tcell.KeyPgUp:
		m.move(-m.listHeight())
	case tcell.KeyTab:
		if m.opts.Multi {
			m.toggleSelect()
			m.move(1)
		}
	case tcell.KeyBacktab:
		if m.opts.Multi {
			m.toggleSelect()
			m.move(-1)
		}
	case tcell.KeyRune:
		m.query = append(m.query, ev.Rune())
		m.recompute()
		m.refreshPreview()
	}
	return false, Result{}
}

// listHeight returns the number of list rows currently available.
func (m *model) listHeight() int {
	_, h := m.screen.Size()
	lh := h
	if m.opts.Preview != "" && !m.previewHidden && (m.pv.pos == "up" || m.pv.pos == "down") {
		lh = h - m.previewSize(h) - 1
	}
	top := 2 + len(m.opts.Header)
	if lh-top < 1 {
		return 1
	}
	return lh - top
}

func (m *model) previewSize(total int) int {
	n := total / 2
	if m.pv.percent {
		n = total * m.pv.pct / 100
	} else {
		n = m.pv.abs
	}
	if n < 1 {
		n = 1
	}
	if n > total-2 {
		n = total - 2
	}
	return n
}

func (m *model) draw() {
	s := m.screen
	s.Clear()
	w, h := s.Size()

	showPreview := m.opts.Preview != "" && !m.previewHidden
	lx, ly, lw, lh := 0, 0, w, h
	px, py, pw, ph := 0, 0, 0, 0
	sepPos, sepHoriz := -1, false

	if showPreview {
		switch m.pv.pos {
		case "left", "right":
			pwd := m.previewSize(w)
			pw, ph = pwd, h
			if m.pv.pos == "right" {
				px, lw, sepPos = w-pwd, w-pwd-1, w-pwd-1
			} else {
				px, lx, lw, sepPos = 0, pwd+1, w-pwd-1, pwd
			}
		default: // up / down
			phd := m.previewSize(h)
			pw, ph, sepHoriz = w, phd, true
			if m.pv.pos == "up" {
				py, ly, lh, sepPos = 0, phd+1, h-phd-1, phd
			} else {
				py, lh, sepPos = h-phd, h-phd-1, h-phd-1
			}
		}
		if lw < 1 {
			lw = 1
		}
		if lh < 1 {
			lh = 1
		}
	}

	m.renderFinder(lx, ly, lw, lh)
	if showPreview {
		m.renderPreview(px, py, pw, ph)
		sepStyle := tcell.StyleDefault.Foreground(tcell.ColorGray)
		if sepHoriz {
			for x := 0; x < w; x++ {
				s.SetContent(x, sepPos, '─', nil, sepStyle)
			}
		} else {
			for y := 0; y < h; y++ {
				s.SetContent(sepPos, y, '│', nil, sepStyle)
			}
		}
	}
	s.Show()
}

func (m *model) renderFinder(lx, ly, lw, lh int) {
	s := m.screen
	var (
		promptStyle  = tcell.StyleDefault.Foreground(tcell.ColorAqua)
		counterStyle = tcell.StyleDefault.Foreground(tcell.ColorGray)
		pointerStyle = tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true)
		matchStyle   = tcell.StyleDefault.Foreground(tcell.ColorGreen).Bold(true)
		selStyle     = tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
		headerStyle  = tcell.StyleDefault.Foreground(tcell.ColorPurple)
	)

	y := ly
	x := puts(s, lx, y, m.opts.Prompt, promptStyle)
	x = putsClip(s, x, y, string(m.query), tcell.StyleDefault, lx+lw)
	if x > lx+lw-1 {
		x = lx + lw - 1
	}
	s.ShowCursor(x, y)
	y++

	counter := fmt.Sprintf("  %d/%d", len(m.matches), len(m.items))
	if m.opts.Multi && len(m.selected) > 0 {
		counter += fmt.Sprintf(" (%d selected)", len(m.selected))
	}
	puts(s, lx, y, truncate(counter, lw), counterStyle)
	y++

	for _, hl := range m.opts.Header {
		puts(s, lx, y, truncate(hl, lw), headerStyle)
		y++
	}

	listTop := y
	listRows := ly + lh - listTop
	if listRows < 0 {
		listRows = 0
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+listRows {
		m.offset = m.cursor - listRows + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}

	for row := 0; row < listRows; row++ {
		idx := m.offset + row
		if idx >= len(m.matches) {
			break
		}
		mt := m.matches[idx]
		yy := listTop + row
		if idx == m.cursor {
			puts(s, lx, yy, ">", pointerStyle)
		}
		if m.opts.Multi && m.selected[mt.Index] {
			puts(s, lx+1, yy, "+", selStyle)
		}
		base := tcell.StyleDefault
		if idx == m.cursor {
			base = base.Bold(true)
		}
		var spans []ansi.Span
		if !m.opts.HasWithNth && m.opts.Colors != nil && mt.Index < len(m.opts.Colors) {
			spans = m.opts.Colors[mt.Index]
		}
		drawMatch(s, lx+2, yy, lw-2, mt, spans, base, matchStyle)
	}
}

func (m *model) renderPreview(px, py, pw, ph int) {
	if pw < 1 || ph < 1 {
		return
	}
	s := m.screen
	for row := 0; row < ph; row++ {
		li := m.previewOffset + row
		if li < 0 || li >= len(m.previewLines) {
			continue
		}
		puts(s, px, py+row, truncate(expandTabs(m.previewLines[li]), pw), tcell.StyleDefault)
	}
}

// refreshPreview launches the preview command for the current selection if the
// expanded command changed. It runs in a goroutine and posts results back so a
// slow preview never blocks input.
func (m *model) refreshPreview() {
	if m.opts.Preview == "" {
		return
	}
	cur, ok := m.current()
	if !ok {
		m.previewKey, m.previewLines, m.previewOffset = "", nil, 0
		return
	}
	cmdStr := m.expandPreview(cur)
	if cmdStr == m.previewKey {
		return
	}
	m.previewKey = cmdStr
	m.previewOffset = 0
	m.previewGen++
	gen := m.previewGen
	go func() {
		out := runShell(cmdStr)
		lines := strings.Split(strings.ReplaceAll(stripANSI(out), "\r\n", "\n"), "\n")
		m.screen.PostEvent(&previewEvent{t: time.Now(), gen: gen, lines: lines})
	}()
}

var placeholderRe = regexp.MustCompile(`\{[^{}]*\}`)

// expandPreview substitutes preview placeholders: {} (current item), {q}
// (query), {n} (index), {+} (selected items), and {N}/{-N}/{N..M} (fields).
func (m *model) expandPreview(cur matcher.Match) string {
	query := string(m.query)
	selected := m.selectedItems(cur)
	return placeholderRe.ReplaceAllStringFunc(m.opts.Preview, func(tok string) string {
		switch inner := tok[1 : len(tok)-1]; inner {
		case "":
			return shellQuote(cur.Output)
		case "q":
			return shellQuote(query)
		case "n":
			return strconv.Itoa(cur.Index)
		case "+":
			parts := make([]string, len(selected))
			for i, s := range selected {
				parts[i] = shellQuote(s)
			}
			return strings.Join(parts, " ")
		default:
			if ranges, ok := tokenizer.ParseRanges(inner); ok {
				return shellQuote(tokenizer.Join(cur.Output, m.opts.Delimiter, ranges))
			}
			return tok
		}
	})
}

// --- small helpers ---

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

func runShell(cmd string) string {
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.Command("cmd", "/c", cmd)
	} else {
		c = exec.Command("sh", "-c", cmd)
	}
	out, _ := c.CombinedOutput()
	return string(out)
}

func shellQuote(s string) string {
	if runtime.GOOS == "windows" {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func puts(s tcell.Screen, x, y int, str string, style tcell.Style) int {
	for _, r := range str {
		s.SetContent(x, y, r, nil, style)
		x++
	}
	return x
}

func putsClip(s tcell.Screen, x, y int, str string, style tcell.Style, maxX int) int {
	for _, r := range str {
		if x >= maxX {
			break
		}
		s.SetContent(x, y, r, nil, style)
		x++
	}
	return x
}

func drawMatch(s tcell.Screen, x, y, maxw int, mt matcher.Match, spans []ansi.Span, base, high tcell.Style) {
	if maxw <= 0 {
		return
	}
	posSet := make(map[int]bool, len(mt.Positions))
	for _, p := range mt.Positions {
		posSet[p] = true
	}
	for i, r := range []rune(mt.Text) {
		if i >= maxw {
			s.SetContent(x+maxw-1, y, '…', nil, base)
			break
		}
		st := base
		for _, sp := range spans {
			if i >= sp.Start && i < sp.End {
				st = sp.Style
				break
			}
		}
		if posSet[i] {
			st = high
		}
		s.SetContent(x+i, y, r, nil, st)
	}
}

func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	return string(r[:w-1]) + "…"
}

func expandTabs(s string) string {
	return strings.ReplaceAll(s, "\t", "    ")
}

func deleteWord(q []rune) []rune {
	i := len(q)
	for i > 0 && q[i-1] == ' ' {
		i--
	}
	for i > 0 && q[i-1] != ' ' {
		i--
	}
	return q[:i]
}

// keyName returns a canonical name for a key event (e.g. "ctrl-y", "alt-x",
// "f5", "enter") used to match against --expect keys. Returns "" if unnamed.
func keyName(ev *tcell.EventKey) string {
	k := ev.Key()
	switch k {
	case tcell.KeyEnter:
		return "enter"
	case tcell.KeyTab:
		return "tab"
	case tcell.KeyEsc:
		return "esc"
	}
	if k >= tcell.KeyCtrlA && k <= tcell.KeyCtrlZ {
		return fmt.Sprintf("ctrl-%c", 'a'+rune(k-tcell.KeyCtrlA))
	}
	if k >= tcell.KeyF1 && k <= tcell.KeyF12 {
		return fmt.Sprintf("f%d", int(k-tcell.KeyF1)+1)
	}
	if k == tcell.KeyRune {
		if ev.Modifiers()&tcell.ModAlt != 0 {
			return "alt-" + string(unicode.ToLower(ev.Rune()))
		}
		return string(ev.Rune())
	}
	return ""
}

func atoiPos(s string) (int, bool) {
	n := 0
	if s == "" {
		return 0, false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

func parsePreviewWindow(spec string) (previewWindow, bool) {
	pv := previewWindow{pos: "right", pct: 50, percent: true}
	hidden := false
	for _, part := range strings.Split(spec, ",") {
		switch part = strings.ToLower(strings.TrimSpace(part)); part {
		case "":
		case "up", "top":
			pv.pos = "up"
		case "down", "bottom":
			pv.pos = "down"
		case "left":
			pv.pos = "left"
		case "right":
			pv.pos = "right"
		case "hidden":
			hidden = true
		default:
			if strings.HasSuffix(part, "%") {
				if n, ok := atoiPos(strings.TrimSuffix(part, "%")); ok {
					pv.pct, pv.percent = n, true
				}
			} else if n, ok := atoiPos(part); ok {
				pv.abs, pv.percent = n, false
			}
			// any other token (border styles, wrap, etc.) is accepted and ignored
		}
	}
	return pv, hidden
}
