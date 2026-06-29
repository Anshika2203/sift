// Package ui implements sift's interactive, full-screen fuzzy-finder interface.
package ui

import (
	"fmt"
	"os"
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

	Reverse bool   // top-down layout (prompt at top); false = bottom-up
	Cycle   bool   // wrap-around cursor movement
	Mouse   bool   // enable mouse (wheel scroll, click to select)
	Color   string // --color theme overrides, e.g. "prompt:cyan,hl:green"
	History string // path to a query-history file
	Margin  string // empty space outside the finder: "T,R,B,L" forms
	Padding string // empty space inside the border
	Border  string // border style: "", "none", "rounded", "sharp", ...

	Bind       []string // --bind specs (key/event -> actions)
	JumpLabels string   // characters used to label rows in jump mode
	Footer     []string // sticky footer lines shown at the very bottom
}

// Result is what the user picked.
type Result struct {
	Selected []string // chosen items (empty when aborted)
	Aborted  bool     // true if the user pressed Esc / Ctrl-C
	Query    string   // the final query string
	Key      string   // the expect key that was pressed (empty for plain Enter)
	Become   string   // if non-empty, the caller should exec this command
}

// RunCommand runs a command interactively (attached to the terminal) and
// returns its exit code. The caller uses it to honour a `become` action after
// Run has returned and the screen has been finalized.
func RunCommand(command string) int { return runInteractive(command) }

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

	bindings, err := parseBindings(opts.Bind)
	if err != nil {
		screen.Fini()
		return Result{}, err
	}

	if opts.Mouse {
		screen.EnableMouse()
	}

	m := &model{
		screen:        screen,
		opts:          opts,
		items:         items,
		selected:      map[int]bool{},
		query:         []rune(opts.Query),
		expect:        expect,
		pv:            pv,
		previewHidden: hidden,
		theme:         parseTheme(opts.Color),
		reverse:       opts.Reverse,
		histFile:      opts.History,
		margin:        parseInsets(opts.Margin),
		padding:       parseInsets(opts.Padding),
		bindings:      bindings,
		labelRows:     map[rune]int{},
	}
	labels := opts.JumpLabels
	if labels == "" {
		labels = "asdfghjkl;qwertyuiopzxcvbnm"
	}
	m.jumpLabels = []rune(labels)
	switch strings.ToLower(opts.Border) {
	case "", "none":
		m.border = false
	case "sharp", "bold", "double", "block":
		m.border, m.borderRound = true, false
	default: // rounded and anything else
		m.border, m.borderRound = true, true
	}
	m.loadHistory()
	m.recompute()
	if b, ok := m.bindings["start"]; ok {
		m.inEvent = true
		if done, res := m.runActions(b); done {
			return res, nil
		}
		m.inEvent = false
	}
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
		case *tcell.EventMouse:
			if m.handleMouse(ev) {
				m.draw()
			}
		case *tcell.EventKey:
			if done, res := m.handleKey(ev); done {
				m.saveHistory(res.Query)
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

	theme   theme
	reverse bool

	history  []string
	histIdx  int
	histFile string

	// list geometry from the last render, for mapping mouse clicks to items
	geoLx, geoListRows, geoListY0, geoListBottom int
	geoReverse                                   bool

	margin      [4]int // top, right, bottom, left
	padding     [4]int
	border      bool
	borderRound bool

	bindings map[string][]action
	inEvent  bool // guards against event bindings re-firing events

	reloadStack [][]string // previous item lists, for the `backward` action

	jumpMode   bool
	jumpAccept bool
	jumpLabels []rune
	labelRows  map[rune]int // label rune -> match index (set during render)
}

// theme holds the foreground colors for UI elements (overridable via --color).
type theme struct {
	prompt, pointer, marker, info, header, hl, fg tcell.Color
}

func defaultTheme() theme {
	return theme{
		prompt:  tcell.ColorAqua,
		pointer: tcell.ColorRed,
		marker:  tcell.ColorYellow,
		info:    tcell.ColorGray,
		header:  tcell.ColorPurple,
		hl:      tcell.ColorGreen,
		fg:      tcell.ColorDefault,
	}
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
	n := len(m.matches)
	if n == 0 {
		return
	}
	old := m.cursor
	m.cursor += delta
	if m.opts.Cycle {
		m.cursor = ((m.cursor % n) + n) % n
	} else {
		if m.cursor < 0 {
			m.cursor = 0
		}
		if m.cursor >= n {
			m.cursor = n - 1
		}
	}
	if m.cursor != old {
		m.refreshPreview()
		m.fireEvent("focus")
	}
}

// editQuery recomputes after a query edit and fires the change event.
func (m *model) editQuery() {
	m.recompute()
	m.refreshPreview()
	m.fireEvent("change")
}

// fireEvent runs the actions bound to an event name (start/change/focus). The
// inEvent guard stops an event's actions from recursively re-firing events.
func (m *model) fireEvent(name string) {
	if m.inEvent {
		return
	}
	if acts, ok := m.bindings[name]; ok {
		m.inEvent = true
		m.runActions(acts)
		m.inEvent = false
	}
}

// runActions executes a sequence of bound actions. It returns done=true (with a
// Result) if an action ends the session (accept / abort / become).
func (m *model) runActions(acts []action) (bool, Result) {
	for _, a := range acts {
		switch a.typ {
		case actUp:
			m.move(-1)
		case actDown:
			m.move(1)
		case actPageUp:
			m.move(-m.listHeight())
		case actPageDown:
			m.move(m.listHeight())
		case actHalfPageUp:
			m.move(-m.listHeight() / 2)
		case actHalfPageDown:
			m.move(m.listHeight() / 2)
		case actFirst:
			m.move(-len(m.matches))
		case actLast:
			m.move(len(m.matches))
		case actAccept:
			return true, m.accept()
		case actAbort:
			return true, Result{Aborted: true, Query: string(m.query)}
		case actToggle:
			m.toggleSelect()
		case actToggleAll:
			m.toggleAll()
		case actSelectAll:
			m.selectAll(true)
		case actDeselectAll:
			m.selectAll(false)
		case actClearQuery:
			m.query = nil
			m.editQuery()
		case actClearSelection:
			m.selected = map[int]bool{}
		case actBackwardDeleteChar:
			if len(m.query) > 0 {
				m.query = m.query[:len(m.query)-1]
				m.editQuery()
			}
		case actTogglePreview:
			m.previewHidden = !m.previewHidden
		case actPreviewUp:
			m.previewScroll(-1)
		case actPreviewDown:
			m.previewScroll(1)
		case actPreviewPageUp:
			m.previewScroll(-10)
		case actPreviewPageDown:
			m.previewScroll(10)
		case actChangeQuery:
			m.query = []rune(a.arg)
			m.editQuery()
		case actChangePrompt:
			m.opts.Prompt = a.arg
		case actPut:
			m.query = append(m.query, []rune(a.arg)...)
			m.editQuery()
		case actExecuteSilent:
			runShell(m.expandFor(a.arg))
		case actExecute:
			m.execute(m.expandFor(a.arg))
		case actBecome:
			return true, Result{Become: m.expandFor(a.arg), Query: string(m.query)}
		case actReload:
			m.reload(m.expandFor(a.arg))
		case actBackward:
			m.backward()
		case actJump:
			m.startJump(false)
		case actJumpAccept:
			m.startJump(true)
		case actIgnore:
		}
	}
	return false, Result{}
}

func (m *model) toggleAll() {
	for _, mt := range m.matches {
		if m.selected[mt.Index] {
			delete(m.selected, mt.Index)
		} else {
			m.selected[mt.Index] = true
		}
	}
}

func (m *model) selectAll(sel bool) {
	if !sel {
		m.selected = map[int]bool{}
		return
	}
	for _, mt := range m.matches {
		m.selected[mt.Index] = true
	}
}

// reload replaces the candidate list with the output of command, remembering
// the previous list so `backward` can return to it (e.g. for folder navigation).
func (m *model) reload(command string) {
	prev := make([]string, len(m.items))
	copy(prev, m.items)
	const maxStack = 256
	if len(m.reloadStack) >= maxStack {
		m.reloadStack = m.reloadStack[1:]
	}
	m.reloadStack = append(m.reloadStack, prev)

	out := strings.ReplaceAll(runShell(command), "\r\n", "\n")
	var items []string
	for _, line := range strings.Split(out, "\n") {
		if line != "" {
			items = append(items, line)
		}
	}
	m.items = items
	m.opts.Colors = nil
	m.selected = map[int]bool{}
	m.recompute()
	m.refreshPreview()
}

// backward restores the item list from before the last reload (the "go back"
// half of folder navigation).
func (m *model) backward() {
	n := len(m.reloadStack)
	if n == 0 {
		return
	}
	m.items = m.reloadStack[n-1]
	m.reloadStack = m.reloadStack[:n-1]
	m.opts.Colors = nil
	m.selected = map[int]bool{}
	m.recompute()
	m.refreshPreview()
}

// startJump enters jump mode: the next render labels the visible rows and the
// next keypress jumps the cursor to the chosen row (and accepts if accept).
func (m *model) startJump(accept bool) {
	if len(m.matches) == 0 {
		return
	}
	m.jumpMode = true
	m.jumpAccept = accept
}

// execute suspends the screen, runs an interactive command attached to the
// terminal, then restores the screen.
func (m *model) execute(command string) {
	m.screen.Suspend()
	runInteractive(command)
	m.screen.Resume()
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
	// In jump mode the next keypress chooses a labeled row.
	if m.jumpMode {
		m.jumpMode = false
		accept := m.jumpAccept
		if ev.Key() == tcell.KeyRune {
			if idx, ok := m.labelRows[ev.Rune()]; ok {
				m.cursor = idx
				m.refreshPreview()
				if accept {
					return true, m.accept()
				}
			}
		}
		return false, Result{} // any non-label key just cancels jump mode
	}

	// Expect keys accept the selection and report which key was pressed.
	if len(m.expect) > 0 {
		if name := keyName(ev); name != "" && m.expect[name] {
			r := m.accept()
			r.Key = name
			return true, r
		}
	}

	// User-defined --bind key bindings take precedence over the built-in keys.
	if len(m.bindings) > 0 {
		if acts, ok := m.bindings[keyName(ev)]; ok {
			return m.runActions(acts)
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

	// With --history, Ctrl-P/Ctrl-N navigate the query history instead of the
	// list (the arrow keys still move the list).
	if m.histFile != "" {
		switch ev.Key() {
		case tcell.KeyCtrlP:
			m.historyPrev()
			return false, Result{}
		case tcell.KeyCtrlN:
			m.historyNext()
			return false, Result{}
		}
	}

	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		return true, Result{Aborted: true, Query: string(m.query)}
	case tcell.KeyEnter:
		return true, m.accept()
	case tcell.KeyCtrlU:
		m.query = nil
		m.editQuery()
	case tcell.KeyCtrlW:
		m.query = deleteWord(m.query)
		m.editQuery()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(m.query) > 0 {
			m.query = m.query[:len(m.query)-1]
			m.editQuery()
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
		m.editQuery()
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
	top := 2 + len(m.opts.Header) + len(m.opts.Footer)
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
	sw, sh := s.Size()

	// Inset the drawing area: margin (outside), then border, then padding.
	ix, iy, iw, ih := m.margin[3], m.margin[0], sw-m.margin[1]-m.margin[3], sh-m.margin[0]-m.margin[2]
	if m.border {
		drawBorder(s, ix, iy, iw, ih, m.borderRound)
		ix, iy, iw, ih = ix+1, iy+1, iw-2, ih-2
	}
	ix, iy = ix+m.padding[3], iy+m.padding[0]
	iw, ih = iw-m.padding[1]-m.padding[3], ih-m.padding[0]-m.padding[2]
	if iw < 1 {
		iw = 1
	}
	if ih < 1 {
		ih = 1
	}

	showPreview := m.opts.Preview != "" && !m.previewHidden
	lx, ly, lw, lh := ix, iy, iw, ih
	px, py, pw, ph := 0, 0, 0, 0
	sepPos, sepHoriz, hasSep := -1, false, false

	if showPreview {
		switch m.pv.pos {
		case "left", "right":
			pwd := m.previewSize(iw)
			pw, ph = pwd, ih
			if m.pv.pos == "right" {
				px, py, lw, sepPos = ix+iw-pwd, iy, iw-pwd-1, ix+iw-pwd-1
			} else {
				px, py, lx, lw, sepPos = ix, iy, ix+pwd+1, iw-pwd-1, ix+pwd
			}
		default: // up / down
			phd := m.previewSize(ih)
			pw, ph, sepHoriz = iw, phd, true
			if m.pv.pos == "up" {
				px, py, ly, lh, sepPos = ix, iy, iy+phd+1, ih-phd-1, iy+phd
			} else {
				px, py, lh, sepPos = ix, iy+ih-phd, ih-phd-1, iy+ih-phd-1
			}
		}
		hasSep = true
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
		if hasSep && sepHoriz {
			for x := ix; x < ix+iw; x++ {
				s.SetContent(x, sepPos, '─', nil, sepStyle)
			}
		} else if hasSep {
			for y := iy; y < iy+ih; y++ {
				s.SetContent(sepPos, y, '│', nil, sepStyle)
			}
		}
	}
	s.Show()
}

func drawBorder(s tcell.Screen, x, y, w, h int, round bool) {
	if w < 2 || h < 2 {
		return
	}
	st := tcell.StyleDefault.Foreground(tcell.ColorGray)
	tl, tr, bl, br := '┌', '┐', '└', '┘'
	if round {
		tl, tr, bl, br = '╭', '╮', '╰', '╯'
	}
	for i := 1; i < w-1; i++ {
		s.SetContent(x+i, y, '─', nil, st)
		s.SetContent(x+i, y+h-1, '─', nil, st)
	}
	for j := 1; j < h-1; j++ {
		s.SetContent(x, y+j, '│', nil, st)
		s.SetContent(x+w-1, y+j, '│', nil, st)
	}
	s.SetContent(x, y, tl, nil, st)
	s.SetContent(x+w-1, y, tr, nil, st)
	s.SetContent(x, y+h-1, bl, nil, st)
	s.SetContent(x+w-1, y+h-1, br, nil, st)
}

// parseInsets parses a margin/padding spec: "T,R,B,L", "T,RL", "TB,RL", or a
// single value applied to all sides. Non-numeric or empty values yield zeros.
func parseInsets(spec string) [4]int {
	var out [4]int
	if strings.TrimSpace(spec) == "" {
		return out
	}
	var v []int
	for _, p := range strings.Split(spec, ",") {
		n, _ := atoiPos(strings.TrimSpace(strings.TrimSuffix(p, "%")))
		v = append(v, n)
	}
	switch len(v) {
	case 1:
		out = [4]int{v[0], v[0], v[0], v[0]}
	case 2:
		out = [4]int{v[0], v[1], v[0], v[1]}
	case 3:
		out = [4]int{v[0], v[1], v[2], v[1]}
	default:
		out = [4]int{v[0], v[1], v[2], v[3]}
	}
	return out
}

func (m *model) renderFinder(lx, ly, lw, lh int) {
	s := m.screen
	th := m.theme
	promptStyle := tcell.StyleDefault.Foreground(th.prompt)
	counterStyle := tcell.StyleDefault.Foreground(th.info)
	pointerStyle := tcell.StyleDefault.Foreground(th.pointer).Bold(true)
	matchStyle := tcell.StyleDefault.Foreground(th.hl).Bold(true)
	selStyle := tcell.StyleDefault.Foreground(th.marker).Bold(true)
	headerStyle := tcell.StyleDefault.Foreground(th.header)
	footerStyle := tcell.StyleDefault.Foreground(tcell.ColorGray)
	textStyle := tcell.StyleDefault
	if th.fg != tcell.ColorDefault {
		textStyle = textStyle.Foreground(th.fg)
	}

	nHeader := len(m.opts.Header)
	nFooter := len(m.opts.Footer)
	listRows := lh - 2 - nHeader - nFooter
	if listRows < 0 {
		listRows = 0
	}
	footerY0 := ly + lh - nFooter // footer occupies the last nFooter rows

	// Row positions depend on the layout. reverse = prompt at top; otherwise
	// (default) prompt at the bottom with the best match nearest it.
	var promptY, counterY, headerY0, listTop, listBottom int
	rowOf := func(row int) int { return listTop + row } // index offset -> screen row
	if m.reverse {
		promptY, counterY, headerY0, listTop = ly, ly+1, ly+2, ly+2+nHeader
		listBottom = listTop + listRows - 1
	} else {
		// bottom layout: [list][header][counter][prompt][footer]
		promptY = footerY0 - 1
		counterY = promptY - 1
		headerY0 = counterY - nHeader
		listBottom = headerY0 - 1
		rowOf = func(row int) int { return listBottom - row }
	}

	x := puts(s, lx, promptY, m.opts.Prompt, promptStyle)
	x = putsClip(s, x, promptY, string(m.query), textStyle, lx+lw)
	if x > lx+lw-1 {
		x = lx + lw - 1
	}
	s.ShowCursor(x, promptY)

	counter := fmt.Sprintf("  %d/%d", len(m.matches), len(m.items))
	if m.opts.Multi && len(m.selected) > 0 {
		counter += fmt.Sprintf(" (%d selected)", len(m.selected))
	}
	puts(s, lx, counterY, truncate(counter, lw), counterStyle)

	for k, hl := range m.opts.Header {
		puts(s, lx, headerY0+k, truncate(hl, lw), headerStyle)
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

	if m.jumpMode {
		m.labelRows = map[rune]int{}
	}
	for row := 0; row < listRows; row++ {
		idx := m.offset + row
		if idx >= len(m.matches) {
			break
		}
		mt := m.matches[idx]
		yy := rowOf(row)
		if m.jumpMode {
			if row < len(m.jumpLabels) {
				lbl := m.jumpLabels[row]
				m.labelRows[lbl] = idx
				puts(s, lx, yy, string(lbl), pointerStyle)
			}
		} else if idx == m.cursor {
			puts(s, lx, yy, ">", pointerStyle)
		}
		if m.opts.Multi && m.selected[mt.Index] {
			puts(s, lx+1, yy, "+", selStyle)
		}
		base := textStyle
		if idx == m.cursor {
			base = base.Bold(true)
		}
		var spans []ansi.Span
		if !m.opts.HasWithNth && m.opts.Colors != nil && mt.Index < len(m.opts.Colors) {
			spans = m.opts.Colors[mt.Index]
		}
		drawMatch(s, lx+2, yy, lw-2, mt, spans, base, matchStyle)
	}

	for k, fl := range m.opts.Footer {
		puts(s, lx, footerY0+k, truncate(fl, lw), footerStyle)
	}

	// Record geometry so mouse clicks can be mapped back to list indices.
	m.geoLx, m.geoListRows, m.geoReverse = lx, listRows, m.reverse
	if m.reverse {
		m.geoListY0 = listTop
	} else {
		m.geoListBottom = listBottom
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

// expandPreview substitutes placeholders in the preview command.
func (m *model) expandPreview(cur matcher.Match) string {
	return m.expandTemplate(m.opts.Preview, cur)
}

// expandFor expands placeholders in tmpl against the current selection, used by
// execute/become/reload actions.
func (m *model) expandFor(tmpl string) string {
	cur, _ := m.current()
	return m.expandTemplate(tmpl, cur)
}

// expandTemplate substitutes placeholders: {} (current item), {q} (query),
// {n} (index), {+} (selected items), and {N}/{-N}/{N..M} (fields).
func (m *model) expandTemplate(tmpl string, cur matcher.Match) string {
	query := string(m.query)
	selected := m.selectedItems(cur)
	return placeholderRe.ReplaceAllStringFunc(tmpl, func(tok string) string {
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

// --- mouse ---

func (m *model) handleMouse(ev *tcell.EventMouse) bool {
	switch ev.Buttons() {
	case tcell.WheelUp:
		m.move(-3)
		return true
	case tcell.WheelDown:
		m.move(3)
		return true
	case tcell.Button1:
		_, y := ev.Position()
		if idx, ok := m.indexAt(y); ok && idx != m.cursor {
			m.cursor = idx
			m.refreshPreview()
			return true
		}
	}
	return false
}

func (m *model) indexAt(y int) (int, bool) {
	rel := y - m.geoListY0
	if !m.geoReverse {
		rel = m.geoListBottom - y
	}
	if rel < 0 || rel >= m.geoListRows {
		return 0, false
	}
	idx := m.offset + rel
	if idx < 0 || idx >= len(m.matches) {
		return 0, false
	}
	return idx, true
}

// --- history ---

func (m *model) loadHistory() {
	if m.histFile == "" {
		return
	}
	if data, err := os.ReadFile(m.histFile); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if line != "" {
				m.history = append(m.history, line)
			}
		}
	}
	m.histIdx = len(m.history)
}

func (m *model) historyPrev() {
	if m.histIdx > 0 {
		m.histIdx--
		m.query = []rune(m.history[m.histIdx])
		m.recompute()
		m.refreshPreview()
	}
}

func (m *model) historyNext() {
	if m.histIdx < len(m.history)-1 {
		m.histIdx++
		m.query = []rune(m.history[m.histIdx])
	} else {
		m.histIdx = len(m.history)
		m.query = nil
	}
	m.recompute()
	m.refreshPreview()
}

func (m *model) saveHistory(q string) {
	if m.histFile == "" || q == "" {
		return
	}
	if n := len(m.history); n > 0 && m.history[n-1] == q {
		return
	}
	f, err := os.OpenFile(m.histFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(q + "\n")
}

// --- theme ---

func parseTheme(spec string) theme {
	th := defaultTheme()
	for _, part := range strings.Split(spec, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), ":", 2)
		if len(kv) != 2 {
			continue
		}
		col, ok := colorByName(kv[1])
		if !ok {
			continue
		}
		switch strings.ToLower(kv[0]) {
		case "prompt":
			th.prompt = col
		case "pointer":
			th.pointer = col
		case "marker":
			th.marker = col
		case "info":
			th.info = col
		case "header":
			th.header = col
		case "hl", "hl+":
			th.hl = col
		case "fg", "fg+":
			th.fg = col
		}
	}
	return th
}

func colorByName(s string) (tcell.Color, bool) {
	switch strings.ToLower(s) {
	case "black":
		return tcell.PaletteColor(0), true
	case "red":
		return tcell.PaletteColor(1), true
	case "green":
		return tcell.PaletteColor(2), true
	case "yellow":
		return tcell.PaletteColor(3), true
	case "blue":
		return tcell.PaletteColor(4), true
	case "magenta":
		return tcell.PaletteColor(5), true
	case "cyan":
		return tcell.PaletteColor(6), true
	case "white":
		return tcell.PaletteColor(7), true
	case "default":
		return tcell.ColorDefault, true
	}
	if n, ok := atoiPos(s); ok {
		return tcell.PaletteColor(n), true
	}
	return tcell.ColorDefault, false
}

// --- small helpers ---

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

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
	case tcell.KeyUp:
		return "up"
	case tcell.KeyDown:
		return "down"
	case tcell.KeyLeft:
		return "left"
	case tcell.KeyRight:
		return "right"
	case tcell.KeyPgUp:
		return "pgup"
	case tcell.KeyPgDn:
		return "pgdn"
	case tcell.KeyHome:
		return "home"
	case tcell.KeyEnd:
		return "end"
	case tcell.KeyDelete:
		return "del"
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return "bspace"
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
