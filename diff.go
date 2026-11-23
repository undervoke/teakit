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

	tea "charm.land/bubbletea/v2"
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

// DiffModel is an embeddable diff question. It never quits the program;
// the owner reads Decision and decides what follows.
type DiffModel struct {
	cfg         DiffConfig
	state       DiffState
	decision    DiffDecision
	files       []DiffFile
	reason      string
	width       int
	height      int
	offset      int
	readyAt     time.Time
	acked       bool
	notice      string
	leave       LeaveModel
	leaveActive bool
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
	return DiffModel{cfg: cfg, width: 80}
}

// WithFiles supplies the diff content and moves the model to Ready, or to
// Incomplete when a section contradicts its own presence flags. Supplying
// new content resets the acknowledgement and the scroll position.
func (m DiffModel) WithFiles(files []DiffFile) DiffModel {
	m.files = files
	m.offset = 0
	m.acked = false
	m.notice = ""
	m.leaveActive = false
	m.readyAt = time.Now()
	m.state = DiffReady
	m.reason = ""
	if reason := diffContradiction(files); reason != "" {
		m.state = DiffIncomplete
		m.reason = reason
	}
	return m
}

// WithIncomplete shows a host-supplied failure instead of a diff. Only back,
// cancel, and interrupt act. Supplying it resets the acknowledgement.
func (m DiffModel) WithIncomplete(reason string) DiffModel {
	m.reason = reason
	m.state = DiffIncomplete
	m.offset = 0
	m.acked = false
	m.notice = ""
	m.leaveActive = false
	return m
}

// SetSize fixes the viewport. The scroll anchor is kept on the logical row
// at the top, because wrapping changes raw offsets on resize.
func (m DiffModel) SetSize(width, height int) DiffModel {
	if width > 0 {
		var sec, sub int
		var path, label string
		if rows := m.contentRows(m.effWidth()); m.offset < len(rows) && m.offset >= 0 {
			anchor := rows[m.offset]
			sec, path, label = anchor.sec, anchor.path, anchor.label
			sub = diffRowSub(rows, m.offset)
		}
		m.width = width
		m.height = height
		m.offset = 0
		rows := m.contentRows(m.effWidth())
		if len(rows) > 0 {
			best := 0
			for i, r := range rows {
				if r.sec > sec || (r.sec == sec && (r.path != path || r.label != label)) {
					break
				}
				if r.sec == sec && diffRowSub(rows, i) >= sub {
					best = i
					break
				}
				best = i
			}
			m.offset = best
		}
		return m.clampOffset(rows)
	}
	m.height = height
	return m.clampOffset(m.contentRows(m.effWidth()))
}

// diffRowSub counts the row's index within its section, so the resize anchor
// survives rewrapping.
func diffRowSub(rows []diffRow, idx int) int {
	sub := 0
	for i := idx; i >= 0; i-- {
		if rows[i].secStart && i != idx {
			break
		}
		if rows[i].sec != rows[idx].sec {
			break
		}
		sub++
	}
	return sub
}

// Init performs no I/O. Arming is timestamp-based (see readyAt), so there
// is no background tick to leak in tests or embedded hosts.
func (m DiffModel) Init() tea.Cmd { return nil }

// Update routes keys to the diff or to its embedded leave question. Enter
// confirms only on a complete, visible, armed diff with the acknowledgement
// (when configured) armed by y; AlternateKey answers under the same
// conditions. Paste messages are ignored so pasted text can never consent.
func (m DiffModel) Update(msg tea.Msg) (DiffModel, tea.Cmd) {
	if m.decision != DiffPending {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.SetSize(msg.Width, msg.Height), nil
	case tea.PasteMsg:
		return m, nil
	case tea.KeyPressMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

// updateKey handles one key press outside the leave question.
func (m DiffModel) updateKey(key tea.KeyPressMsg) (DiffModel, tea.Cmd) {
	if m.leaveActive {
		return m.updateLeave(key), nil
	}
	name := key.String()
	switch name {
	case "ctrl+c":
		m.decision = DiffInterrupt
		return m, nil
	case "esc":
		return m.cancel(), nil
	case "backspace":
		if m.cfg.Back {
			m.decision = DiffBack
		}
		return m, nil
	case "enter":
		if key.Key().IsRepeat {
			return m, nil
		}
		return m.answer(DiffConfirm, "Enter"), nil
	}
	var scrolled bool
	m, scrolled = m.scrollKey(name)
	if scrolled {
		return m, nil
	}
	switch name {
	case "y", "Y":
		if key.Key().IsRepeat {
			return m, nil
		}
		m.notice = ""
		if m.cfg.Acknowledge != "" && m.state == DiffReady && m.armed() {
			m.acked = true
		}
		return m, nil
	case "n", "N":
		m.notice = ""
		if m.cfg.Acknowledge != "" && m.state == DiffReady {
			m.acked = false
		}
		return m, nil
	}
	if m.cfg.AlternateKey != "" && name == m.cfg.AlternateKey {
		if key.Key().IsRepeat {
			return m, nil
		}
		return m.answer(DiffAlternate, name), nil
	}
	if slices.Contains(m.cfg.CancelKeys, name) {
		return m.cancel(), nil
	}
	return m, nil
}

// updateLeave forwards keys to the embedded leave question. Stay returns to
// the diff with scroll and acknowledgement kept; Leave cancels; Ctrl+C
// interrupts from any state including the question.
func (m DiffModel) updateLeave(key tea.KeyPressMsg) DiffModel {
	if key.String() == "ctrl+c" {
		m.decision = DiffInterrupt
		m.leaveActive = false
		return m
	}
	if m.leaveQuestion(m.effWidth()) == nil {
		return m
	}
	next, _ := m.leave.Update(key)
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

// scrollKey moves the viewport and reports whether the key was bound.
func (m DiffModel) scrollKey(name string) (DiffModel, bool) {
	rows := m.contentRows(m.effWidth())
	page := 1
	if whole := m.atTop(rows); len(whole.visible) > 0 {
		page = len(whole.visible)
	}
	switch name {
	case "up", "k":
		m.offset--
	case "down", "j":
		m.offset++
	case "pgup":
		m.offset -= page
	case "pgdown":
		m.offset += page
	case "home", "g":
		m.offset = 0
	case "end", "G":
		m.offset = len(rows)
	case "]":
		next := len(rows)
		for i := m.offset + 1; i < len(rows); i++ {
			if rows[i].secStart {
				next = i
				break
			}
		}
		m.offset = next
	case "[":
		prev := 0
		for i := m.offset - 1; i >= 0; i-- {
			if rows[i].secStart {
				prev = i
				break
			}
		}
		m.offset = prev
	default:
		return m, false
	}
	m.notice = ""
	return m.clampOffset(rows), true
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
		m.notice = "The diff is not ready; nothing can be applied yet."
		return m
	}
	if m.tooSmall() {
		m.notice = "Terminal too small to review; enlarge it or press esc."
		return m
	}
	if !m.armed() {
		m.notice = "The diff is not ready; nothing can be applied yet."
		return m
	}
	w := m.effWidth()
	rows := m.contentRows(w)
	lay := m.clampOffset(rows).layout(rows, w, nil)
	if !slices.ContainsFunc(lay.visible, func(r diffRow) bool { return r.sec >= 0 && r.text != "" }) {
		return m
	}
	if m.cfg.Acknowledge != "" && !m.acked {
		m.notice = "! Press y to accept: " + m.cfg.Acknowledge + ". " + key + " alone never confirms."
		return m
	}
	m.decision = decision
	return m
}

// armed reports whether the settle delay has elapsed since the first Ready
// frame.
func (m DiffModel) armed() bool {
	return !m.readyAt.IsZero() && time.Since(m.readyAt) >= diffSettleDelay
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
		return m.fitHeight([]string{"Terminal too small to review; enlarge it or press esc."})
	}
	var out []string
	if m.cfg.Title != "" {
		out = append(out, strings.Split(m.cfg.Title, "\n")...)
	}
	switch m.state {
	case DiffLoading:
		out = append(out, "Preparing the diff…")
		if m.notice != "" {
			out = append(out, diffStyle(diffSGRYellow+";"+diffSGRBold, m.notice))
		}
		return m.fitHeight(out)
	case DiffIncomplete:
		out = append(out, diffIncompleteLines(m.reason)...)
		if m.notice != "" {
			out = append(out, diffStyle(diffSGRYellow+";"+diffSGRBold, m.notice))
		}
		return m.fitHeight(out)
	}
	rows := m.contentRows(w)
	total := len(rows)
	m = m.clampOffset(rows)
	lay := m.layout(rows, w, question)
	out = append(out, lay.pinned...)
	for _, r := range lay.visible {
		out = append(out, r.text)
	}
	out = append(out, lay.below...)
	if question == nil {
		out = append(out, m.footerLines(w, rows, lay, total)...)
	} else if m.height > 0 && len(out) > m.height {
		return m.fitHeight(question)
	}
	return m.fitHeight(out)
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
	return m.effWidth() < 30 || (m.height > 0 && m.height < 7)
}

// contentRows builds the scrollable rows: body lines above the first file,
// then every file's table.
func (m DiffModel) contentRows(width int) []diffRow {
	var rows []diffRow
	if m.cfg.Body != "" {
		for _, paragraph := range strings.Split(m.cfg.Body, "\n") {
			body, _ := diffEscape(paragraph)
			for _, line := range diffWrapLine(body, width) {
				rows = append(rows, diffRow{text: line, sec: -1})
			}
		}
	}
	sec := 0
	for i, f := range m.files {
		sec = diffLayoutFile(f, i, sec, width, &rows)
	}
	return rows
}

// diffViewLayout splits the view into pinned lines, visible rows, and the
// lines below the viewport, sharing one height budget so the footer is
// never cut by the pinned context lines.
type diffViewLayout struct {
	top     int
	pinned  []string
	visible []diffRow
	below   []string
}

// layout computes the visible slice for the current offset and size.
func (m DiffModel) layout(rows []diffRow, width int, question []string) diffViewLayout {
	top := m.offset
	if top > len(rows) {
		top = len(rows)
	}
	if top < 0 {
		top = 0
	}
	pinned := m.pinnedContext(rows, top, width)
	var below []string
	if m.cfg.Acknowledge != "" {
		below = append(below, m.ackLine(width))
	}
	if m.notice != "" {
		for _, line := range diffWrapLine(mustEscape(m.notice), width) {
			below = append(below, diffStyle(diffSGRYellow+";"+diffSGRBold, line))
		}
	}
	footer := 0
	if question != nil {
		below = append(below, question...)
	} else {
		footer = 2 // status + hint
	}
	avail := len(rows) - top
	if m.height > 0 {
		avail = m.height - titleLines(m.cfg.Title) - len(pinned) - len(below) - footer
		if avail < 0 {
			avail = 0
		}
		if avail > len(rows)-top {
			avail = len(rows) - top
		}
	}
	return diffViewLayout{top: top, pinned: pinned, visible: rows[top : top+avail], below: below}
}

// titleLines counts the pinned title lines.
func titleLines(title string) int {
	if title == "" {
		return 0
	}
	return len(strings.Split(title, "\n"))
}

// footerLines renders the status line and the width-dependent hint line.

// pinnedContext returns the path context lines when the file header has
// scrolled off: `path > @ label` plus the column heading in side-by-side
// mode, or the same line with ` · before/after` in stacked mode.
func (m DiffModel) pinnedContext(rows []diffRow, top, width int) []string {
	if top <= 0 || top >= len(rows) {
		return nil
	}
	cur := rows[top]
	if cur.sec < 0 {
		return nil
	}
	// The file's header is its rows before the first section label.
	first := top
	for first > 0 && rows[first-1].file == cur.file {
		first--
	}
	headerEnd := top + 1 // a file always has a section at or below top
	for i := first; i < len(rows) && rows[i].file == cur.file; i++ {
		if rows[i].secStart {
			headerEnd = i
			break
		}
	}
	if top < headerEnd {
		return nil
	}
	ctx, _ := diffEscape(cur.path + " > @ " + cur.label)
	suffix := ""
	layout := diffFileLayout(width, diffNumWidth(m.files[cur.file]))
	if !layout.side {
		suffix = " · before"
		if cur.block == 1 {
			suffix = " · after"
		}
	}
	var out []string
	for _, line := range diffWrapLine(ctx+suffix, width) {
		out = append(out, diffStyle(diffSGRFaint, line))
	}
	if layout.side {
		lw := diffFileLayout(width, diffNumWidth(m.files[cur.file]))
		out = append(out, diffStyle(diffSGRFaint, diffPadRight("before", lw.leftW)+" | "+diffPadRight("after", lw.rightW)))
	}
	return out
}

// atTop lays out the view from the first row to measure a full page for
// pgup/pgdown, independent of the current scroll position.
func (m DiffModel) atTop(rows []diffRow) diffViewLayout {
	top := m
	top.offset = 0
	return top.layout(rows, top.effWidth(), nil)
}

// ackLine renders the pinned acknowledgement with its hint.
func (m DiffModel) ackLine(width int) string {
	box := "[ ]"
	hint := "y accept · n undo"
	if m.acked {
		box = "[x]"
		hint = "n undo"
	}
	left := box + " " + m.cfg.Acknowledge
	if diffWidth(left)+diffWidth(hint)+2 <= width {
		return diffStyle(diffSGRYellow+";"+diffSGRBold, left+strings.Repeat(" ", width-diffWidth(left)-diffWidth(hint))+hint)
	}
	return diffStyle(diffSGRYellow+";"+diffSGRBold, left)
}

// leaveQuestion returns the entire prompt, or nil when it can't be shown.
// View can give it the whole box; Update uses the same visibility check.
func (m DiffModel) leaveQuestion(width int) []string {
	var out []string
	for _, line := range strings.Split(m.leave.View(), "\n") {
		for _, wrapped := range diffWrapLine(mustEscapePlain(line), width) {
			if diffWidth(wrapped) > width || (m.height > 0 && len(out) == m.height) {
				return nil
			}
			out = append(out, wrapped)
		}
	}
	return out
}

func (m DiffModel) footerLines(width int, rows []diffRow, lay diffViewLayout, total int) []string {
	status := "lines 0 of 0"
	if total > 0 {
		a, b := lay.top+1, lay.top+len(lay.visible)
		if b > total {
			b = total
		}
		if a > total {
			a = total
		}
		status = "lines " + diffItoa(a) + "-" + diffItoa(b) + " of " + diffItoa(total)
		if n := diffTotalSections(m.files); n > 0 {
			status += " · section " + diffItoa(diffSectionAt(rows, lay.top, n)) + "/" + diffItoa(n)
		}
		if below := total - (lay.top + len(lay.visible)); below > 0 {
			status += " · " + diffItoa(below) + " more below"
		}
	}
	return []string{diffStyle(diffSGRFaint, status), m.hintLine(width)}
}

// hintLine picks the longest hint that fits the width. The answer, esc, and
// scroll hints survive longest.
func (m DiffModel) hintLine(width int) string {
	answers := []string{"enter " + m.cfg.ApplyLabel}
	if m.cfg.AlternateKey != "" {
		answers = append(answers, strings.TrimSpace(m.cfg.AlternateKey+" "+m.cfg.AlternateLabel))
	}
	parts := append([]string{"↑/↓ k/j scroll", "pgup/pgdown page", "g/G top/bottom", "]/[ section"}, answers...)
	if m.cfg.Acknowledge != "" && m.acked {
		parts = append(parts, "n undo")
	}
	if m.cfg.Back {
		parts = append(parts, "backspace back")
	}
	parts = append(parts, "esc cancel", "ctrl+c exit")
	if m.cfg.Acknowledge != "" && !m.acked {
		parts = []string{"↑/↓ k/j scroll", "y accept", "esc cancel", "ctrl+c exit"}
	}
	full := strings.Join(parts, " · ")
	if diffWidth(full) <= width {
		return diffStyle(diffSGRFaint, full)
	}
	short := append(append([]string{"↑/↓ k/j scroll", "pgup/pgdown", "]/[ section"}, answers...), "esc cancel")
	if m.cfg.Acknowledge != "" && !m.acked {
		short = []string{"k/j scroll", "y accept", "esc cancel"}
	}
	if s := strings.Join(short, " · "); diffWidth(s) <= width {
		return diffStyle(diffSGRFaint, s)
	}
	mini := append(append([]string{"k/j scroll"}, answers...), "esc")
	if s := strings.Join(mini, " · "); diffWidth(s) <= width {
		return diffStyle(diffSGRFaint, s)
	}
	last := strings.Join(answers, " · ") + " · esc"
	if diffWidth(last) <= width {
		return diffStyle(diffSGRFaint, last)
	}
	return diffStyle(diffSGRFaint, "esc")
}

// fitHeight pads short views and cuts long ones to exactly height lines.
func (m DiffModel) fitHeight(lines []string) string {
	if m.height <= 0 {
		return strings.Join(lines, "\n")
	}
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	for len(lines) < m.height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// clampOffset stops scrolling at the first page that includes the last row.
func (m DiffModel) clampOffset(rows []diffRow) DiffModel {
	m.offset = min(max(0, m.offset), max(0, len(rows)-1))
	if m.offset == 0 {
		return m
	}
	// The unpinned top page gives an upper bound on how many rows can fit.
	// Near the bottom, find the last page using each candidate's pinned context.
	start := len(rows) - len(m.atTop(rows).visible)
	for top := start; top < m.offset; top++ {
		candidate := m
		candidate.offset = top
		if top+len(candidate.layout(rows, m.effWidth(), nil).visible) == len(rows) {
			return candidate
		}
	}
	return m
}

// diffTotalSections counts sections across files.
func diffTotalSections(files []DiffFile) int {
	n := 0
	for _, f := range files {
		n += len(f.Sections)
	}
	return n
}

// diffSectionAt returns the 1-based section holding the top visible row.
func diffSectionAt(rows []diffRow, top, total int) int {
	for i := top; i < len(rows); i++ {
		if rows[i].sec >= 0 {
			return rows[i].sec + 1
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
	out := []string{diffStyle(diffSGRYellow+";"+diffSGRBold, "! The diff could not be prepared completely, so it cannot be applied.")}
	if reason != "" {
		for _, line := range strings.Split(reason, "\n") {
			out = append(out, line)
		}
	}
	return append(out, "Nothing was changed.")
}

// mustEscape escapes s for a refusal notice.
func mustEscape(s string) string {
	escaped, _ := diffEscape(s)
	return escaped
}

// mustEscapePlain escapes s that may already carry SGR codes from another
// renderer (the embedded leave question): strip those first so widths and
// wrapping account for visible cells only.
func mustEscapePlain(s string) string {
	escaped, _ := diffEscape(stripSGR(s))
	return escaped
}

// stripSGR removes SGR escape sequences.
func stripSGR(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				i = j + 1
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
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
		out = append(out, strings.Split(cfg.Title, "\n")...)
	}
	if cfg.Body != "" {
		for _, paragraph := range strings.Split(cfg.Body, "\n") {
			body, _ := diffEscape(paragraph)
			out = append(out, diffWrapLine(body, width)...)
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
		out = append(out, diffStyle(diffSGRYellow+";"+diffSGRBold, box+" "+cfg.Acknowledge))
	}
	return strings.Join(out, "\n")
}

// staticRows lays out every file's rows for the static render.
func staticRows(files []DiffFile, width int) []diffRow {
	var rows []diffRow
	sec := 0
	for i, f := range files {
		sec = diffLayoutFile(f, i, sec, width, &rows)
	}
	return rows
}
