// Row layout for the diff view: side-by-side and stacked tables.
package teakit

import (
	"strings"
)

// minDiffSideCells is the minimum text area per column for the side-by-side
// table. Below it the sections stack before above after.
const minDiffSideCells = 24

// diffRowKind distinguishes data from chrome and identifies rows across rewraps.
type diffRowKind uint8

const (
	diffBodyRow diffRowKind = iota
	diffPathRow
	diffGapRow
	diffColumnsRow
	diffLabelRow
	diffNoteRow
	diffSideHeading
	diffDataRow
)

// diffRow is one rendered content line. file and sec locate it for the
// pinned context line and the section counter; chrome rows (headings,
// labels, notes) carry the file and section they belong to.
type diffRow struct {
	text     string
	file     int
	sec      int // global section index, or -1 for body rows
	secStart bool
	block    int // stacked block: 0 before, 1 after, -1 otherwise
	kind     diffRowKind
	item     int // paragraph or note index, or side for a stacked heading
	part     int // wrapped fragment within the source row
	before   int // source line numbers (0 when absent from this row)
	after    int
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
func diffLayoutFile(f DiffFile, fileIdx, secBase, width int, layout diffLayout, rows *[]diffRow) int {
	for i, line := range diffWrapIndented(f.Path, width, 2) {
		*rows = append(*rows, diffRow{text: diffStyle(diffSGRBold, line), file: fileIdx, sec: secBase, kind: diffPathRow, part: i})
	}
	*rows = append(*rows, diffRow{file: fileIdx, sec: secBase, kind: diffGapRow})
	if layout.side {
		*rows = append(*rows, diffRow{
			text: diffStyle(diffSGRFaint, diffPadRight("before", layout.leftW)+" | "+diffPadRight("after", layout.rightW)),
			file: fileIdx, sec: secBase, kind: diffColumnsRow,
		})
	}
	sec := secBase
	for _, s := range f.Sections {
		diffLayoutSection(s, fileIdx, sec, width, layout, rows)
		sec++
	}
	return sec
}

// diffLayoutSection renders one section's label, notes, and content rows.
func diffLayoutSection(s DiffSection, fileIdx, sec, width int, layout diffLayout, rows *[]diffRow) {
	for i, line := range diffWrapIndented("@ "+s.Label, width, 2) {
		*rows = append(*rows, diffRow{text: diffStyle(diffSGRBold+";"+diffSGRCyan, line), file: fileIdx, sec: sec, secStart: i == 0, kind: diffLabelRow, part: i})
	}
	for j, note := range s.Notes {
		for i, line := range diffWrapIndented("  note: "+note, width, 8) {
			*rows = append(*rows, diffRow{text: diffStyle(diffSGRFaint, line), file: fileIdx, sec: sec, kind: diffNoteRow, item: j, part: i})
		}
	}
	if layout.side {
		diffSideSection(s, fileIdx, sec, layout, rows)
	} else {
		diffStackedSection(s, fileIdx, sec, width, layout, rows)
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
func diffSideSection(s DiffSection, fileIdx, sec int, layout diffLayout, rows *[]diffRow) {
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
			var before, after int
			if i < len(left) {
				l = left[i]
				if o != nil {
					before = o.no
				}
			}
			if i < len(right) {
				r = right[i]
				if n != nil {
					after = n.no
				}
			}
			*rows = append(*rows, diffRow{
				text: diffPadRight(l, layout.leftW) + diffStyle(diffSGRFaint, " | ") + diffPadRight(r, layout.rightW),
				file: fileIdx,
				sec:  sec,
				kind: diffDataRow, part: i, before: before, after: after,
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
	lines := diffWrapLine(item.op.Text, layout.textW)
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
func diffStackedSection(s DiffSection, fileIdx, sec, width int, layout diffLayout, rows *[]diffRow) {
	oldItems, newItems := diffItems(s)
	diffStackedBlock("before", oldItems, s.BeforeAbsent, fileIdx, sec, width, layout, rows)
	diffStackedBlock("after", newItems, s.AfterAbsent, fileIdx, sec, width, layout, rows)
}

// diffStackedBlock renders one stacked block with its heading.
func diffStackedBlock(heading string, items []diffContentItem, absent bool, fileIdx, sec, width int, layout diffLayout, rows *[]diffRow) {
	block := 0
	if heading == "after" {
		block = 1
	}
	*rows = append(*rows, diffRow{text: diffStyle(diffSGRFaint, heading), file: fileIdx, sec: sec, block: block, kind: diffSideHeading, item: block})
	if absent {
		*rows = append(*rows, diffRow{text: diffPlaceholder("(absent)"), file: fileIdx, sec: sec, block: block, kind: diffDataRow})
		return
	}
	if len(items) == 0 {
		*rows = append(*rows, diffRow{text: diffPlaceholder("(empty)"), file: fileIdx, sec: sec, block: block, kind: diffDataRow})
		return
	}
	textW := width - (layout.numWidth + 3)
	if textW < 1 {
		textW = 1
	}
	for _, item := range items {
		item := item
		lines := diffWrapLine(item.op.Text, textW)
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
			r := diffRow{text: n + " " + mark + " " + body, file: fileIdx, sec: sec, block: block, kind: diffDataRow, part: i}
			if block == 0 {
				r.before = item.no
			} else {
				r.after = item.no
			}
			*rows = append(*rows, r)
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
