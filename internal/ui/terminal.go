// Package ui implements sift's interactive, full-screen fuzzy-finder interface.
package ui

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/Anshika2203/sift/internal/matcher"
)

// Options configures an interactive session.
type Options struct {
	Prompt  string // text shown before the query, e.g. "> "
	Query   string // initial query
	Multi   bool   // allow selecting multiple items with Tab
	Preview string // command template; "{}" is replaced by the highlighted item
	Header  string // a fixed header line shown above the list
}

// Result is what the user picked.
type Result struct {
	Selected []string // chosen items (empty when aborted)
	Aborted  bool     // true if the user pressed Esc / Ctrl-C
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

	m := &model{
		screen:   screen,
		opts:     opts,
		items:    items,
		selected: map[int]bool{},
		query:    []rune(opts.Query),
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

// model holds all interactive state.
type model struct {
	screen   tcell.Screen
	opts     Options
	items    []string
	matches  []matcher.Match
	query    []rune
	cursor   int          // index into matches (0 = best match, at the top)
	offset   int          // first visible match (scroll position)
	selected map[int]bool // keyed by original item index

	previewGen   int
	previewItem  string
	previewLines []string
}

// previewEvent carries asynchronous preview output back to the event loop.
type previewEvent struct {
	t     time.Time
	gen   int
	lines []string
}

func (e *previewEvent) When() time.Time { return e.t }

func (m *model) recompute() {
	m.matches = matcher.Filter(m.items, string(m.query), true)
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

func (m *model) toggleSelect() {
	if cur, ok := m.current(); ok {
		if m.selected[cur.Index] {
			delete(m.selected, cur.Index)
		} else {
			m.selected[cur.Index] = true
		}
	}
}

func (m *model) accept() Result {
	if m.opts.Multi && len(m.selected) > 0 {
		var out []string
		for i := range m.items {
			if m.selected[i] {
				out = append(out, m.items[i])
			}
		}
		return Result{Selected: out}
	}
	if cur, ok := m.current(); ok {
		return Result{Selected: []string{cur.Text}}
	}
	return Result{Aborted: true}
}

func (m *model) handleKey(ev *tcell.EventKey) (bool, Result) {
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		return true, Result{Aborted: true}
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

// listHeight returns the number of rows currently available for the list.
func (m *model) listHeight() int {
	_, h := m.screen.Size()
	top := 2 // prompt + counter
	if m.opts.Header != "" {
		top++
	}
	if h-top < 1 {
		return 1
	}
	return h - top
}

func (m *model) draw() {
	s := m.screen
	s.Clear()
	w, h := s.Size()

	listW := w
	previewOn := m.opts.Preview != ""
	if previewOn {
		listW = w / 2
		if listW < 12 { // too narrow to bother splitting
			listW = w
			previewOn = false
		}
	}

	var (
		promptStyle  = tcell.StyleDefault.Foreground(tcell.ColorAqua)
		counterStyle = tcell.StyleDefault.Foreground(tcell.ColorGray)
		pointerStyle = tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true)
		matchStyle   = tcell.StyleDefault.Foreground(tcell.ColorGreen).Bold(true)
		selStyle     = tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
		headerStyle  = tcell.StyleDefault.Foreground(tcell.ColorPurple)
		sepStyle     = tcell.StyleDefault.Foreground(tcell.ColorGray)
	)

	y := 0

	// Prompt + query.
	x := puts(s, 0, y, m.opts.Prompt, promptStyle)
	x = puts(s, x, y, string(m.query), tcell.StyleDefault)
	s.ShowCursor(x, y)
	y++

	// Counter line.
	counter := fmt.Sprintf("  %d/%d", len(m.matches), len(m.items))
	if m.opts.Multi && len(m.selected) > 0 {
		counter += fmt.Sprintf(" (%d selected)", len(m.selected))
	}
	puts(s, 0, y, counter, counterStyle)
	y++

	// Optional header.
	if m.opts.Header != "" {
		puts(s, 0, y, truncate(m.opts.Header, listW), headerStyle)
		y++
	}

	listTop := y
	listRows := h - listTop
	if listRows < 0 {
		listRows = 0
	}

	// Keep the cursor on screen.
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
		ly := listTop + row

		if idx == m.cursor {
			puts(s, 0, ly, ">", pointerStyle)
		}
		if m.opts.Multi && m.selected[mt.Index] {
			puts(s, 1, ly, "+", selStyle)
		}

		base := tcell.StyleDefault
		if idx == m.cursor {
			base = base.Bold(true)
		}
		drawMatch(s, 2, ly, listW-2, mt, base, matchStyle)
	}

	// Preview pane.
	if previewOn {
		for yy := 0; yy < h; yy++ {
			s.SetContent(listW, yy, '│', nil, sepStyle)
		}
		px := listW + 2
		pw := w - px
		for i, line := range m.previewLines {
			if i >= h {
				break
			}
			puts(s, px, i, truncate(expandTabs(line), pw), tcell.StyleDefault)
		}
	}

	s.Show()
}

// refreshPreview launches the preview command for the highlighted item, if the
// item changed since last time. The command runs in a goroutine and posts its
// output back to the event loop so a slow preview never blocks input.
func (m *model) refreshPreview() {
	if m.opts.Preview == "" {
		return
	}
	cur, ok := m.current()
	if !ok {
		m.previewItem = ""
		m.previewLines = nil
		return
	}
	if cur.Text == m.previewItem {
		return
	}
	m.previewItem = cur.Text
	m.previewGen++
	gen := m.previewGen
	cmdStr := strings.ReplaceAll(m.opts.Preview, "{}", shellQuote(cur.Text))

	go func() {
		out := runShell(cmdStr)
		lines := strings.Split(strings.ReplaceAll(stripANSI(out), "\r\n", "\n"), "\n")
		m.screen.PostEvent(&previewEvent{t: time.Now(), gen: gen, lines: lines})
	}()
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

func drawMatch(s tcell.Screen, x, y, maxw int, mt matcher.Match, base, high tcell.Style) {
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
