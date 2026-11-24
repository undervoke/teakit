// Package teakit provides shared Bubble Tea components with one set of key
// conventions for terminal tools. This file adds the side-by-side override
// diff view: a consent screen that shows what applying an update would
// change, and answers Enter, or the host's alternate key, only on a
// complete, visible, armed diff.
package teakit

import (
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// DiffOpKind identifies one line of a section diff.
type DiffOpKind int

const (
	// OpEqual marks a line present on both sides.
	OpEqual DiffOpKind = iota
	// OpDelete marks a line present only on the before side.
	OpDelete
	// OpInsert marks a line present only on the after side.
	OpInsert
)

// DiffOp is one line of a section diff, without any trailing newline.
type DiffOp struct {
	Kind DiffOpKind
	Text string
}

// DiffSection is one override region: a label, host disclosures, and the
// per-line edit list. An absent side renders an "(absent)" placeholder;
// a present side with no lines renders "(empty)".
type DiffSection struct {
	Label        string
	Notes        []string
	BeforeAbsent bool
	AfterAbsent  bool
	Ops          []DiffOp
}

// DiffFile groups the sections of one logical file.
type DiffFile struct {
	Path     string
	Sections []DiffSection
}

// DiffState tracks whether the diff is ready to confirm.
type DiffState int

const (
	// DiffLoading waits for the host's content.
	DiffLoading DiffState = iota
	// DiffReady shows a complete diff that Enter can confirm.
	DiffReady
	// DiffIncomplete shows why the diff cannot be applied.
	DiffIncomplete
)

// DiffDecision is the answer to the diff question. Pending means unanswered.
type DiffDecision int

const (
	DiffPending DiffDecision = iota
	DiffConfirm
	DiffBack
	DiffCancel
	DiffInterrupt
	// DiffAlternate answers the host's AlternateKey.
	DiffAlternate
)

// DiffConfig supplies the diff view's wording and behavior. Teakit fixes the
// layout, styles, and key meanings; the host supplies the text and decides
// whether a second answer sits beside Enter, when cancelling asks first,
// when Back exists, and whether an extra acknowledgement gates the answers.
type DiffConfig struct {
	// Title is pinned above the diff; empty when the host draws its own.
	Title string
	// Body scrolls above the first file, e.g. a data-loss notice.
	Body string
	// ApplyLabel is Enter's hint verb; it defaults to "apply".
	ApplyLabel string
	// AlternateKey, when set, is an exact tea.Key name that answers
	// DiffAlternate under the same guards as Enter; built-in bindings win
	// over it. AlternateLabel is its hint verb.
	AlternateKey   string
	AlternateLabel string
	// CancelKeys holds exact tea.Key names that cancel like Esc (e.g. "q");
	// built-in bindings win over these names.
	CancelKeys []string
	// Back makes Backspace answer DiffBack; otherwise it is ignored.
	Back bool
	// Leave, when set, opens this leave question on cancel instead of
	// cancelling at once.
	Leave *LeaveConfig
	// Acknowledge, when set, pins "[ ] <text>" above the footer and makes
	// Enter and AlternateKey inert until y arms it.
	Acknowledge string
}

// diffSettleDelay is the quiet period after the first Ready frame before y,
// Enter, and AlternateKey act. Without it, an Enter typed ahead during the
// spinner, or a double-press on the wizard's Apply button, could confirm a
// diff nobody saw.
var diffSettleDelay = 300 * time.Millisecond

const (
	diffNotReady = "The diff is not ready; nothing can be applied yet."
	diffNoData   = "No diff data is visible; scroll or enlarge the terminal."
)

// diffStartMsg lets a preloaded diff begin settling from Update, not Init.
type diffStartMsg struct{}

// diffSettledMsg arms only the content that scheduled the timer.
type diffSettledMsg struct{ generation *int }

// DiffModel is an embeddable diff question. It never quits the program;
// the owner reads Decision and decides what follows.
type DiffModel struct {
	cfg          DiffConfig
	state        DiffState
	decision     DiffDecision
	files        []DiffFile
	reason       string
	width        int
	height       int
	explicitSize bool
	pager        viewport.Model
	keys         diffKeys
	rows         []diffRow
	pins         [][2][]string // indexed by section and stacked side
	title        []string
	generation   *int // identity of this WithFiles call, including across model copies
	settling     bool
	settled      bool
	acked        bool
	notice       string
	leave        LeaveModel
	leaveActive  bool
}

// NewDiff returns a diff waiting for content.
func NewDiff(cfg DiffConfig) DiffModel {
	if cfg.ApplyLabel == "" {
		cfg.ApplyLabel = "apply"
	}
	cfg.CancelKeys = slices.Clone(cfg.CancelKeys)
	if cfg.Leave != nil {
		leave := *cfg.Leave
		cfg.Leave = &leave
	}
	p := viewport.New(viewport.WithWidth(80))
	p.MouseWheelEnabled = false
	// Keep teakit's existing key surface: viewport's extra aliases can
	// already be in use as host-supplied answer or cancel keys.
	p.KeyMap = viewport.KeyMap{Up: p.KeyMap.Up, Down: p.KeyMap.Down, PageUp: p.KeyMap.PageUp, PageDown: p.KeyMap.PageDown}
	p.KeyMap.PageUp.SetKeys(tea.KeyPressMsg{Code: tea.KeyPgUp}.String())
	p.KeyMap.PageDown.SetKeys(tea.KeyPressMsg{Code: tea.KeyPgDown}.String())
	p.KeyMap.PageUp.SetHelp(p.KeyMap.PageUp.Keys()[0], "page up")
	p.KeyMap.PageDown.SetHelp(p.KeyMap.PageDown.Keys()[0], "page down")
	m := DiffModel{cfg: cfg, width: 80, pager: p, keys: newDiffKeys(cfg, p.KeyMap)}
	m.rebuild()
	return m
}

// WithFiles supplies the diff content and moves the model to Ready, or to
// Incomplete when a section contradicts its own presence flags. Supplying
// new content snapshots it and resets the acknowledgement and scroll position.
func (m DiffModel) WithFiles(files []DiffFile) DiffModel {
	m.files = slices.Clone(files)
	for i := range m.files {
		m.files[i].Sections = slices.Clone(files[i].Sections)
		for j := range m.files[i].Sections {
			s := &m.files[i].Sections[j]
			s.Notes = slices.Clone(s.Notes)
			s.Ops = slices.Clone(s.Ops)
		}
	}
	m.acked = false
	m.notice = ""
	m.leaveActive = false
	m.generation = new(int)
	m.settling, m.settled = false, false
	m.state = DiffReady
	m.reason = ""
	if reason := diffContradiction(m.files); reason != "" {
		m.state = DiffIncomplete
		m.reason = reason
	}
	m.rebuild()
	m.pager.SetYOffset(0)
	return m
}

// WithIncomplete shows a host-supplied failure instead of a diff. Only back,
// cancel, and interrupt act. Supplying it resets the acknowledgement.
func (m DiffModel) WithIncomplete(reason string) DiffModel {
	m.reason = reason
	m.state = DiffIncomplete
	m.acked = false
	m.notice = ""
	m.leaveActive = false
	m.generation = nil
	m.settling, m.settled = false, false
	m.rebuild()
	return m
}

// SetSize sets the host's allotted box, taking precedence over subsequent
// WindowSizeMsg values. A non-positive height cannot show an interactive diff.
func (m DiffModel) SetSize(width, height int) DiffModel {
	m.explicitSize = true
	return m.setSize(width, height)
}

// setSize reflows the box while preserving the source row at the top.
func (m DiffModel) setSize(width, height int) DiffModel {
	if width <= 0 {
		width = m.effWidth()
	}
	if width == m.width && height == m.height {
		return m
	}
	top := m.pager.YOffset()
	m.height = height
	m.notice = ""
	if m.width != width {
		old := m.rows
		m.width = width
		m.rebuild()
		if top < len(old) {
			top = diffFindAnchor(m.rows, old[top])
		}
	}
	m.pager = m.normalPage(top).pager
	return m
}

// Init queues a message so a preloaded diff starts settling only after the
// host starts its Bubble Tea message loop. Forward this command and all
// commands/messages from Update while the diff is active.
func (m DiffModel) Init() tea.Cmd { return func() tea.Msg { return diffStartMsg{} } }

// Update routes keys to the diff or to its embedded leave question. Enter
// confirms only on a complete, visible, armed diff with the acknowledgement
// (when configured) armed by y; AlternateKey answers under the same
// conditions. Paste messages are ignored so pasted text can never consent.
func (m DiffModel) Update(msg tea.Msg) (DiffModel, tea.Cmd) {
	if m.decision != DiffPending {
		return m, nil
	}
	var cmd, start tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if !m.explicitSize {
			m = m.setSize(msg.Width, msg.Height)
		}
	case diffSettledMsg:
		if m.settling && msg.generation == m.generation {
			m.settling, m.settled = false, true
			if m.notice == diffNotReady {
				m.notice = ""
			}
		}
	case tea.KeyPressMsg:
		// Evaluate the frame before the key can add a notice or open a
		// modal, and below once more after navigation or leaving a modal.
		m, start = m.startSettle()
		m, cmd = m.updateKey(msg)
	}
	m, settle := m.startSettle()
	return m, tea.Batch(start, cmd, settle)
}

// startSettle schedules one timer once the current content can be reviewed.
func (m DiffModel) startSettle() (DiffModel, tea.Cmd) {
	if m.decision != DiffPending || m.settled || m.settling || !m.hasVisibleData() {
		return m, nil
	}
	m.settling = true
	gen := m.generation
	return m, tea.Tick(diffSettleDelay, func(time.Time) tea.Msg { return diffSettledMsg{generation: gen} })
}

// updateKey handles one key press outside the leave question.
func (m DiffModel) updateKey(msg tea.KeyPressMsg) (DiffModel, tea.Cmd) {
	if m.leaveActive {
		return m.updateLeave(msg), nil
	}
	k := m.keys
	switch {
	case key.Matches(msg, k.interrupt):
		m.decision = DiffInterrupt
		return m, nil
	case key.Matches(msg, k.cancel):
		return m.cancel(), nil
	case key.Matches(msg, k.back):
		if m.cfg.Back {
			m.decision = DiffBack
		}
		return m, nil
	case key.Matches(msg, k.enter):
		if msg.Key().IsRepeat {
			return m, nil
		}
		return m.answer(DiffConfirm, "Enter"), nil
	case k.scrolls(msg):
		return m.scrollKey(msg)
	case key.Matches(msg, k.accept):
		if msg.Key().IsRepeat {
			return m, nil
		}
		if m.cfg.Acknowledge != "" && m.hasVisibleData() && m.settled {
			m.acked = true
		}
		m.notice = ""
		return m, nil
	case key.Matches(msg, k.undo):
		m.notice = ""
		if m.cfg.Acknowledge != "" && m.state == DiffReady {
			m.acked = false
		}
		return m, nil
	case key.Matches(msg, k.alternate):
		if msg.Key().IsRepeat {
			return m, nil
		}
		return m.answer(DiffAlternate, msg.String()), nil
	case key.Matches(msg, k.extra):
		return m.cancel(), nil
	}
	return m, nil
}

// updateLeave forwards keys to the embedded leave question. Stay returns to
// the diff with scroll and acknowledgement kept; Leave cancels; Ctrl+C
// interrupts from any state including the question.
func (m DiffModel) updateLeave(msg tea.KeyPressMsg) DiffModel {
	if key.Matches(msg, m.keys.interrupt) {
		m.decision = DiffInterrupt
		m.leaveActive = false
		return m
	}
	if m.leaveQuestion(m.effWidth()) == nil {
		return m
	}
	next, _ := m.leave.Update(msg)
	m.leave = next
	switch m.leave.Decision() {
	case Stay:
		m.leaveActive = false
	case Leave:
		m.decision = DiffCancel
		m.leaveActive = false
	case Interrupt:
		m.decision = DiffInterrupt
		m.leaveActive = false
	}
	return m
}

// scrollKey delegates ordinary scroll keys and adds section and edge jumps.
func (m DiffModel) scrollKey(msg tea.KeyPressMsg) (DiffModel, tea.Cmd) {
	p := m.normalPage(m.pager.YOffset()).pager
	top := p.YOffset()
	var cmd tea.Cmd
	switch k := m.keys; {
	case key.Matches(msg, k.top):
		p.GotoTop()
		top = p.YOffset()
	case key.Matches(msg, k.bottom):
		top = len(m.rows)
	case key.Matches(msg, k.next):
		next := len(m.rows)
		for i := top + 1; i < len(m.rows); i++ {
			if m.rows[i].secStart {
				next = i
				break
			}
		}
		top = next
	case key.Matches(msg, k.prev):
		prev := 0
		for i := top - 1; i >= 0; i-- {
			if m.rows[i].secStart {
				prev = i
				break
			}
		}
		top = prev
	default:
		p, cmd = p.Update(msg)
		top = p.YOffset()
	}
	m.notice = ""
	m.pager = m.normalPage(top).pager
	return m, cmd
}

// cancel asks the configured leave question or cancels at once.
func (m DiffModel) cancel() DiffModel {
	if m.cfg.Leave != nil {
		m.leave = NewLeave(*m.cfg.Leave)
		m.leaveActive = true
		return m
	}
	m.decision = DiffCancel
	return m
}

// answer gives decision on a Ready diff; key names the pressed key in the
// acknowledgement notice. Anything else sets a refusal notice and leaves the
// decision pending.
func (m DiffModel) answer(decision DiffDecision, key string) DiffModel {
	if m.state != DiffReady {
		m.notice = diffNotReady
		return m
	}
	if m.tooSmall() {
		m.notice = "Terminal too small to review; enlarge it or press esc."
		return m
	}
	if !m.hasVisibleData() {
		m.notice = diffNoData
		return m
	}
	if !m.settled {
		m.notice = diffNotReady
		return m
	}
	if m.cfg.Acknowledge != "" && !m.acked {
		m.notice = "Press y to acknowledge before " + key + "."
		return m
	}
	m.decision = decision
	return m
}

// View renders the pinned title, the scrolling rows, the acknowledgement
// and refusal lines, and the footer. It returns exactly height lines when a
// height is set.
func (m DiffModel) View() string {
	w := m.effWidth()
	var question []string
	if m.leaveActive {
		question = m.leaveQuestion(w)
		if question == nil {
			return m.fitHeight(diffWrapLine("Enlarge the terminal to show the leave question, or press ctrl+c.", w))
		}
		if m.tooSmall() || m.state != DiffReady {
			return m.fitHeight(question)
		}
	}
	if m.tooSmall() {
		return m.fitHeight(diffWrapLine("Terminal too small to review; enlarge it or press esc.", w))
	}
	out := slices.Clone(m.title)
	switch m.state {
	case DiffLoading:
		out = append(out, "Preparing the diff…")
		out = append(out, m.noticeLines()...)
		return m.fitHeight(out)
	case DiffIncomplete:
		out = append(out, diffIncompleteLines(m.reason)...)
		out = append(out, m.noticeLines()...)
		return m.fitHeight(out)
	}
	return m.viewReady(question)
}

// State reports whether the diff is ready to confirm.
func (m DiffModel) State() DiffState { return m.state }

// Decision reports the answer, or Pending while unanswered.
func (m DiffModel) Decision() DiffDecision { return m.decision }

// effWidth returns the set width, defaulting to 80 columns.
func (m DiffModel) effWidth() int {
	if m.width > 0 {
		return m.width
	}
	return 80
}

// tooSmall reports whether the terminal cannot show the pinned header, one
// content row, and the footer. A diff the user cannot see is never
// confirmable.
func (m DiffModel) tooSmall() bool {
	return m.effWidth() < 30 || m.height < 7
}

// pinnedContext returns the path context lines when the file header has
// scrolled off: `path > @ label` plus the column heading in side-by-side
// mode, or the same line with ` · before/after` in stacked mode.
func (m DiffModel) pinnedContext(top int) []string {
	if top <= 0 || top >= len(m.rows) {
		return nil
	}
	cur := m.rows[top]
	if cur.kind < diffLabelRow {
		return nil
	}
	return m.pins[cur.sec][cur.block]
}

// leaveQuestion returns the entire prompt, or nil when it can't be shown.
// View can give it the whole box; Update uses the same visibility check.
func (m DiffModel) leaveQuestion(width int) []string {
	var out []string
	for _, line := range m.leave.lines() {
		for _, wrapped := range diffWrapLine(ansi.Strip(line.text), width) {
			if diffWidth(wrapped) > width || len(out) >= m.height {
				return nil
			}
			out = append(out, line.draw(wrapped))
		}
	}
	return out
}

// fitHeight pads short views and cuts long ones to exactly height lines.
func (m DiffModel) fitHeight(lines []string) string {
	if m.height <= 0 {
		return ""
	}
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	for len(lines) < m.height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// diffSectionAt returns the 1-based section holding the top visible row.
func diffSectionAt(rows []diffRow, top, total int) int {
	for i := top; i < len(rows); i++ {
		if rows[i].sec >= 0 {
			return min(total, rows[i].sec+1)
		}
	}
	return total
}

// diffContradiction rejects an absent side that lists lines on that side.
// Equal ops count for both sides, so they contradict either absent flag.
func diffContradiction(files []DiffFile) string {
	for _, f := range files {
		for _, s := range f.Sections {
			for _, op := range s.Ops {
				switch op.Kind {
				case OpEqual:
					if s.BeforeAbsent || s.AfterAbsent {
						return "A section marks a side absent but lists lines on that side."
					}
				case OpDelete:
					if s.BeforeAbsent {
						return "A section marks its before side absent but lists before lines."
					}
				case OpInsert:
					if s.AfterAbsent {
						return "A section marks its after side absent but lists after lines."
					}
				}
			}
		}
	}
	return ""
}

// diffIncompleteLines renders the incomplete state.
func diffIncompleteLines(reason string) []string {
	out := []string{Warning.Render("! The diff could not be prepared completely, so it cannot be applied.")}
	if reason != "" {
		for _, line := range strings.Split(reason, "\n") {
			out = append(out, line)
		}
	}
	return append(out, "Nothing was changed.")
}

// RenderDiff renders the whole diff as static non-interactive text: the
// same table layout at the given width, without footer or hints. Widths
// below 1 default to 80 columns.
func RenderDiff(files []DiffFile, width int) string {
	return renderStatic(DiffConfig{}, files, width, false)
}

// renderStatic renders title, body, all rows, and the acknowledgement line.
// checked marks the acknowledgement as armed; the standalone runner sets it
// on confirm so the scrollback records what the user accepted.
func renderStatic(cfg DiffConfig, files []DiffFile, width int, checked bool) string {
	if width < 1 {
		width = 80
	}
	var out []string
	if cfg.Title != "" {
		out = append(out, strings.Split(Header.Render(cfg.Title), "\n")...)
	}
	if cfg.Body != "" {
		for _, paragraph := range strings.Split(cfg.Body, "\n") {
			out = append(out, diffWrapLine(paragraph, width)...)
		}
	}
	rows := staticRows(files, width)
	for _, r := range rows {
		out = append(out, r.text)
	}
	if cfg.Acknowledge != "" {
		box := "[ ]"
		if checked {
			box = "[x]"
		}
		out = append(out, Warning.Render(box+" "+cfg.Acknowledge))
	}
	return strings.Join(out, "\n")
}

// staticRows lays out every file's rows for the static render.
func staticRows(files []DiffFile, width int) []diffRow {
	var rows []diffRow
	sec := 0
	for i, f := range files {
		sec = diffLayoutFile(f, i, sec, width, diffFileLayout(width, diffNumWidth(f)), &rows)
	}
	return rows
}
