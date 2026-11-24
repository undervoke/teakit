package teakit

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	"github.com/charmbracelet/x/ansi"
)

// rebuild lays out the immutable viewport content only on content/width changes.
func (m *DiffModel) rebuild() {
	w := m.effWidth()
	m.title = nil
	if m.cfg.Title != "" {
		m.title = strings.Split(Header.Render(ansi.Hardwrap(m.cfg.Title, w, true)), "\n")
	}
	m.rows = nil
	m.pins = nil
	if m.state == DiffReady {
		if m.cfg.Body != "" {
			for i, paragraph := range strings.Split(m.cfg.Body, "\n") {
				for j, line := range diffWrapLine(paragraph, w) {
					m.rows = append(m.rows, diffRow{text: line, file: -1, sec: -1, item: i, part: j})
				}
			}
		}
		sec := 0
		for i, f := range m.files {
			layout := diffFileLayout(w, diffNumWidth(f))
			for _, s := range f.Sections {
				m.pins = append(m.pins, diffPinnedLines(f.Path+" > @ "+s.Label, w, layout))
			}
			sec = diffLayoutFile(f, i, sec, w, layout, &m.rows)
		}
	}
	lines := make([]string, len(m.rows))
	for i, r := range m.rows {
		lines[i] = r.text
	}
	m.pager.SetWidth(w)
	m.pager.SetContentLines(lines)
}

// diffPinnedLines renders cached section context for each possible stacked side.
func diffPinnedLines(label string, width int, layout diffLayout) [2][]string {
	lines := func(s string) []string {
		out := diffWrapLine(s, width)
		for i, line := range out {
			out[i] = Muted.Render(line)
		}
		return out
	}
	if layout.side {
		heading := diffPadRight("before", layout.leftW) + " | " + diffPadRight("after", layout.rightW)
		return [2][]string{append(lines(label), Muted.Render(heading))}
	}
	return [2][]string{lines(label + " · before"), lines(label + " · after")}
}

// diffFindAnchor keeps the source row across rewraps. For data shown on both
// sides, use the before line unless the original viewport was stacked after.
func diffFindAnchor(rows []diffRow, from diffRow) int {
	fallback, same := 0, -1
	for i, r := range rows {
		if r.file != from.file || r.sec != from.sec {
			continue
		}
		if r.secStart || r.kind == diffPathRow && r.part == 0 {
			fallback = i
		}
		if r.kind != from.kind || r.item != from.item {
			continue
		}
		if r.kind == diffDataRow {
			switch {
			case from.before != 0:
				if r.before != from.before {
					continue
				}
			case from.after != 0:
				if r.after != from.after {
					continue
				}
			default:
				if r.before != 0 || r.after != 0 || r.block != from.block {
					continue
				}
			}
		}
		same = i
		if r.part >= from.part {
			return i
		}
	}
	if same >= 0 {
		return same
	}
	return fallback
}

// diffPage pairs the pager with its pinned context and body visibility.
type diffPage struct {
	pager  viewport.Model
	pinned []string
	shown  bool
}

// page accounts for the only height that the pager cannot know: pinned
// context depends on its top row. Start at the unpinned bound, then find the
// first fitting last page or stop at the requested top. The pager still owns
// clipping, offsets and movement. A hidden pager retains a one-row navigation
// budget so a user can scroll out of a region whose context fills the box.
func (m DiffModel) page(target, footerHeight int) diffPage {
	p := m.pager
	budget := m.height - len(m.title) - footerHeight
	p.SetHeight(max(1, budget))
	p.SetYOffset(target)
	for top := p.YOffset(); ; top++ {
		pinned := m.pinnedContext(top)
		h := budget - len(pinned)
		// At a file boundary context can shrink abruptly; cap the height
		// there to retain the requested top and still display the final row.
		p.SetHeight(min(max(1, h), max(1, p.TotalLineCount()-top)))
		p.SetYOffset(top)
		if top >= target || p.AtBottom() {
			return diffPage{p, pinned, h > 0}
		}
	}
}

// shownRows counts content rows, excluding the hidden pager's navigation budget.
func (p diffPage) shownRows() int {
	if !p.shown {
		return 0
	}
	return p.pager.VisibleLineCount()
}

// normalPage budgets for the regular footer rather than the leave question.
func (m DiffModel) normalPage(top int) diffPage {
	return m.page(top, len(m.ackLines())+len(m.noticeLines())+2)
}

// hasVisibleData checks whether the current interactive frame shows diff data.
func (m DiffModel) hasVisibleData() bool {
	if m.tooSmall() || m.state != DiffReady || m.leaveActive {
		return false
	}
	p := m.normalPage(m.pager.YOffset())
	return m.pageHasData(p)
}

// pageHasData distinguishes visible edits or placeholders from header rows.
func (m DiffModel) pageHasData(p diffPage) bool {
	top := p.pager.YOffset()
	for _, r := range m.rows[top : top+p.shownRows()] {
		if r.kind == diffDataRow {
			return true
		}
	}
	return false
}

// ackLines wraps the acknowledgement and its current checkbox state.
func (m DiffModel) ackLines() []string {
	if m.cfg.Acknowledge == "" {
		return nil
	}
	box := "[ ] "
	if m.acked {
		box = "[x] "
	}
	return diffWarningLines(box+ansi.Strip(m.cfg.Acknowledge), m.effWidth())
}

// noticeLines wraps the current refusal notice, if any.
func (m DiffModel) noticeLines() []string {
	if m.notice == "" {
		return nil
	}
	return diffWarningLines(m.notice, m.effWidth())
}

// diffWarningLines wraps warning text before styling each row.
func diffWarningLines(text string, width int) []string {
	lines := diffWrapLine(text, width)
	for i, line := range lines {
		lines[i] = Warning.Render(line)
	}
	return lines
}

// viewReady assembles the diff frame, prioritizing a crowded prompt or refusal.
func (m DiffModel) viewReady(question []string) string {
	ack, notice := m.ackLines(), m.noticeLines()
	footer := len(question)
	if question == nil {
		footer = 2
	}
	p := m.page(m.pager.YOffset(), len(ack)+len(notice)+footer)
	if len(m.title)+len(p.pinned)+len(ack)+len(notice)+footer > m.height {
		if question != nil {
			return m.fitHeight(question)
		}
		if len(notice) > 0 {
			return m.fitHeight(notice)
		}
	}
	var out []string
	out = append(out, m.title...)
	out = append(out, p.pinned...)
	if p.shown {
		out = append(out, strings.Split(p.pager.View(), "\n")...)
	}
	out = append(out, ack...)
	out = append(out, notice...)
	if question == nil {
		out = append(out, m.statusLine(p), m.hintLine(m.effWidth(), m.pageHasData(p)))
	} else {
		out = append(out, question...)
	}
	return m.fitHeight(out)
}

// statusLine reports the visible row range and current section.
func (m DiffModel) statusLine(p diffPage) string {
	total := len(m.rows)
	status := "lines 0 of 0"
	if total > 0 {
		top := p.pager.YOffset()
		end := top + p.shownRows()
		status = "lines " + diffItoa(min(top+1, end)) + "-" + diffItoa(end) + " of " + diffItoa(total)
		if n := len(m.pins); n > 0 {
			status += " · section " + diffItoa(diffSectionAt(m.rows, top, n)) + "/" + diffItoa(n)
		}
		if below := total - end; below > 0 {
			status += " · " + diffItoa(below) + " more below"
		}
	}
	return Muted.Render(ansi.Truncate(status, m.effWidth(), "…"))
}
