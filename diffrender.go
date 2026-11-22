// Row layout for the diff view: side-by-side and stacked tables.
package teakit

import (
	"strings"
)

// minDiffSideCells is the minimum text area per column for the side-by-side
// table. Below it the sections stack before above after.
const minDiffSideCells = 24

// diffRow is one rendered content line. file and sec locate it for the
// pinned context line and the section counter; chrome rows (headings,
// labels, notes) carry the file and section they belong to.
type diffRow struct {
	text     string
	file     int
	sec      int // global section index, or -1 for body rows
	secStart bool
	block    int // stacked block: 0 before, 1 after, -1 otherwise
	path     string
	label    string
}

// diffLayout holds one file's table geometry.
type diffLayout struct {
	side     bool
	leftW    int
	rightW   int
	textW    int // per-side text area in side-by-side mode
	numWidth int
}

// diffFileLayout computes the table geometry for one file.
func diffFileLayout(width, numWidth int) diffLayout {
	leftW := (width - 3) / 2
	return diffLayout{
		side:     leftW-(numWidth+3) >= minDiffSideCells,
		leftW:    leftW,
		rightW:   width - 3 - leftW,
		textW:    leftW - (numWidth + 3),
		numWidth: numWidth,
	}
}

// diffNumWidth returns the per-file number width: the digits of the largest
// line number shown, at least 1.
func diffNumWidth(f DiffFile) int {
	width := 1
	for _, s := range f.Sections {
		oldNo, newNo := 0, 0
		for _, op := range s.Ops {
			switch op.Kind {
			case OpEqual:
				oldNo++
				newNo++
			case OpDelete:
				oldNo++
			case OpInsert:
				newNo++
			}
		}
		if d := diffDigits(oldNo); d > width {
			width = d
		}
		if d := diffDigits(newNo); d > width {
			width = d
		}
	}
	return width
}

func diffDigits(n int) int {
	if n <= 0 {
		return 1
	}
	w := 0
	for n > 0 {
		w++
		n /= 10
	}
	return w
}

// diffLayoutFile renders one file's rows: path, blank, heading, then one
// block per section. It returns the global section index after the file.
func diffLayoutFile(f DiffFile, fileIdx, secBase, width int, rows *[]diffRow) int {
	layout := diffFileLayout(width, diffNumWidth(f))
	path, _ := diffEscape(f.Path)
	for i, line := range diffWrapLine(path, width) {
		if i > 0 {
			line = "  " + line
		}
		*rows = append(*rows, diffRow{text: diffStyle(diffSGRBold, line), file: fileIdx, sec: secBase, path: f.Path})
	}
	*rows = append(*rows, diffRow{file: fileIdx, sec: secBase, path: f.Path})
	if layout.side {
		*rows = append(*rows, diffRow{
			text: diffStyle(diffSGRFaint, diffPadRight("before", layout.leftW)+" | "+diffPadRight("after", layout.rightW)),
			file: fileIdx, sec: secBase, path: f.Path,
		})
	}
	sec := secBase
	for _, s := range f.Sections {
		diffLayoutSection(s, fileIdx, sec, f.Path, width, layout, rows)
		sec++
	}
	return sec
}

// diffLayoutSection renders one section's label, notes, and content rows.
func diffLayoutSection(s DiffSection, fileIdx, sec int, path string, width int, layout diffLayout, rows *[]diffRow) {
	label, _ := diffEscape(s.Label)
	for i, line := range diffWrapLine("@ "+label, width) {
		if i > 0 {
			line = "  " + line
		}
		*rows = append(*rows, diffRow{text: diffStyle(diffSGRBold+";"+diffSGRCyan, line), file: fileIdx, sec: sec, secStart: i == 0, path: path, label: s.Label})
	}
	for _, note := range s.Notes {
		text, _ := diffEscape(note)
		for i, line := range diffWrapLine("  note: "+text, width) {
			if i > 0 {
				line = strings.Repeat(" ", 8) + line
			}
			*rows = append(*rows, diffRow{text: diffStyle(diffSGRFaint, line), file: fileIdx, sec: sec, path: path, label: s.Label})
		}
	}
	if layout.side {
		diffSideSection(s, fileIdx, sec, path, layout, rows)
	} else {
		diffStackedSection(s, fileIdx, sec, path, width, layout, rows)
	}
}

// diffContentItem pairs an op with its line number on one side.
type diffContentItem struct {
	no int
	op DiffOp
}

// diffItems splits a section's ops into per-side items with line numbers
// counted from 1 per section.
func diffItems(s DiffSection) (oldItems, newItems []diffContentItem) {
	oldNo, newNo := 0, 0
	for _, op := range s.Ops {
		switch op.Kind {
		case OpEqual:
			oldNo++
			newNo++
			oldItems = append(oldItems, diffContentItem{no: oldNo, op: op})
			newItems = append(newItems, diffContentItem{no: newNo, op: op})
		case OpDelete:
			oldNo++
			oldItems = append(oldItems, diffContentItem{no: oldNo, op: op})
		case OpInsert:
			newNo++
			newItems = append(newItems, diffContentItem{no: newNo, op: op})
		}
	}
	return oldItems, newItems
}

// diffSideSection renders a section as paired left/right rows. Within each
// maximal run of non-equal ops, deletes fill the left in order and inserts
// fill the right; row i pairs the i-th of each, and the shorter side gets
// blank cells.
func diffSideSection(s DiffSection, fileIdx, sec int, path string, layout diffLayout, rows *[]diffRow) {
	oldItems, newItems := diffItems(s)
	leftPH, rightPH := diffPlaceholders(s, len(oldItems) == 0, len(newItems) == 0)
	oi, ni := 0, 0
	row := 0
	emit := func(o, n *diffContentItem) {
		left := diffSideCell(o, leftPH, row == 0, layout)
		right := diffSideCell(n, rightPH, row == 0, layout)
		row++
		height := len(left)
		if len(right) > height {
			height = len(right)
		}
		for i := 0; i < height; i++ {
			l, r := "", ""
			if i < len(left) {
				l = left[i]
			}
			if i < len(right) {
				r = right[i]
			}
			*rows = append(*rows, diffRow{
				text:  diffPadRight(l, layout.leftW) + diffStyle(diffSGRFaint, " | ") + diffPadRight(r, layout.rightW),
				file:  fileIdx,
				sec:   sec,
				path:  path,
				label: s.Label,
			})
		}
	}
	for oi < len(oldItems) || ni < len(newItems) {
		if oi < len(oldItems) && ni < len(newItems) &&
			oldItems[oi].op.Kind == OpEqual && newItems[ni].op.Kind == OpEqual {
			o, n := oldItems[oi], newItems[ni]
			oi++
			ni++
			emit(&o, &n)
			continue
		}
		var olds, news []*diffContentItem
		for oi < len(oldItems) && oldItems[oi].op.Kind != OpEqual {
			olds = append(olds, &oldItems[oi])
			oi++
		}
		for ni < len(newItems) && newItems[ni].op.Kind != OpEqual {
			news = append(news, &newItems[ni])
			ni++
		}
		for i := 0; i < len(olds) || i < len(news); i++ {
			var o, n *diffContentItem
			if i < len(olds) {
				o = olds[i]
			}
			if i < len(news) {
				n = news[i]
			}
			emit(o, n)
		}
	}
	if len(oldItems) == 0 && len(newItems) == 0 {
		emit(nil, nil)
	}
}

// diffPlaceholders returns the placeholder text for sides with no lines:
// "(absent)" when the side does not exist, "(empty)" when it exists but has
// no lines. A side with lines gets "".
func diffPlaceholders(s DiffSection, leftEmpty, rightEmpty bool) (string, string) {
	left, right := "", ""
	if leftEmpty {
		left = "(empty)"
		if s.BeforeAbsent {
			left = "(absent)"
		}
	}
	if rightEmpty {
		right = "(empty)"
		if s.AfterAbsent {
			right = "(absent)"
		}
	}
	return left, right
}

// diffSideCell renders one side's wrapped cell lines. A nil item is a blank
// cell, except on the section's first row, where an empty side shows its
// placeholder.
func diffSideCell(item *diffContentItem, placeholder string, first bool, layout diffLayout) []string {
	if item == nil {
		if first && placeholder != "" {
			return []string{diffPlaceholder(placeholder)}
		}
		return nil
	}
	text, _ := diffEscape(item.op.Text)
	lines := diffWrapLine(text, layout.textW)
	if len(lines) == 0 {
		lines = []string{""}
	}
	num := diffStyle(diffSGRFaint, diffPadLeft(diffItoa(item.no), layout.numWidth))
	mark := diffCellMark(item.op.Kind)
	var cell []string
	for i, line := range lines {
		n := num
		if i > 0 {
			n = strings.Repeat(" ", layout.numWidth)
		}
		body := line
		if item.op.Kind == OpDelete {
			body = diffStyle(diffSGRRed, line)
		} else if item.op.Kind == OpInsert {
			body = diffStyle(diffSGRGreen, line)
		}
		cell = append(cell, n+" "+mark+" "+diffPadRight(body, layout.textW))
	}
	return cell
}

// diffCellMark renders the row marker, colored for changed lines.
func diffCellMark(kind DiffOpKind) string {
	switch kind {
	case OpDelete:
		return diffStyle(diffSGRRed, "-")
	case OpInsert:
		return diffStyle(diffSGRGreen, "+")
	}
	return " "
}

// diffStackedSection renders a section as a before block above an after
// block, each with its own numbers. Equal lines appear in both.
func diffStackedSection(s DiffSection, fileIdx, sec int, path string, width int, layout diffLayout, rows *[]diffRow) {
	oldItems, newItems := diffItems(s)
	diffStackedBlock("before", oldItems, s.BeforeAbsent, fileIdx, sec, path, s.Label, width, layout, rows)
	diffStackedBlock("after", newItems, s.AfterAbsent, fileIdx, sec, path, s.Label, width, layout, rows)
}

// diffStackedBlock renders one stacked block with its heading.
func diffStackedBlock(heading string, items []diffContentItem, absent bool, fileIdx, sec int, path, label string, width int, layout diffLayout, rows *[]diffRow) {
	block := 0
	if heading == "after" {
		block = 1
	}
	*rows = append(*rows, diffRow{text: diffStyle(diffSGRFaint, heading), file: fileIdx, sec: sec, block: block, path: path, label: label})
	if absent {
		*rows = append(*rows, diffRow{text: diffPlaceholder("(absent)"), file: fileIdx, sec: sec, block: block, path: path, label: label})
		return
	}
	if len(items) == 0 {
		*rows = append(*rows, diffRow{text: diffPlaceholder("(empty)"), file: fileIdx, sec: sec, block: block, path: path, label: label})
		return
	}
	textW := width - (layout.numWidth + 3)
	if textW < 1 {
		textW = 1
	}
	for _, item := range items {
		item := item
		text, _ := diffEscape(item.op.Text)
		lines := diffWrapLine(text, textW)
		if len(lines) == 0 {
			lines = []string{""}
		}
		num := diffStyle(diffSGRFaint, diffPadLeft(diffItoa(item.no), layout.numWidth))
		mark := diffCellMark(item.op.Kind)
		for i, line := range lines {
			n := num
			if i > 0 {
				n = strings.Repeat(" ", layout.numWidth)
			}
			body := line
			if item.op.Kind == OpDelete {
				body = diffStyle(diffSGRRed, line)
			} else if item.op.Kind == OpInsert {
				body = diffStyle(diffSGRGreen, line)
			}
			*rows = append(*rows, diffRow{text: n + " " + mark + " " + body, file: fileIdx, sec: sec, block: block, path: path, label: label})
		}
	}
}

// diffPlaceholder renders the faint italic absent/empty marker.
func diffPlaceholder(s string) string {
	return diffStyle(diffSGRFaint+";"+diffSGRChar, s)
}

// diffItoa formats a line number.
func diffItoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
