package teakit

import (
	"strconv"
	"strings"
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

// DiffSection is a changed part of a file: a label, what else to say about it,
// and its lines. A side without lines is marked "absent" when it does not
// exist, and "empty" when it does.
type DiffSection struct {
	Label        string
	Notes        []string
	BeforeAbsent bool
	AfterAbsent  bool
	Ops          []DiffOp
}

// DiffFile groups the changed parts of one file.
type DiffFile struct {
	Path     string
	Sections []DiffSection
}

// RenderDiff draws files as a DiffField does, for text to print. Widths below
// 1 default to 80 columns.
func RenderDiff(files []DiffFile, width int) string {
	if width < 1 {
		width = 80
	}
	return strings.Join(diffRows(files, width), "\n")
}

func diffRows(files []DiffFile, width int) []string {
	var rows []string
	for i, f := range files {
		if i > 0 {
			rows = append(rows, "")
		}
		if f.Path != "" {
			for _, line := range diffWrapIndented(f.Path, width, 2) {
				rows = append(rows, diffStyle(diffSGRBold, line))
			}
		}
		numbers := diffNumWidth(f)
		for _, s := range f.Sections {
			rows = append(rows, s.rows(width, numbers)...)
		}
	}
	return rows
}

// rows draws the section in one column. Lines that change together make a
// block, the removed ones above the added ones. A removed line has its number
// before the change, and any other line the one after it.
func (s DiffSection) rows(width, numbers int) []string {
	var rows, added []string
	if s.Label != "" {
		for _, line := range diffWrapIndented("@ "+s.Label, width, 2) {
			rows = append(rows, Subhead.Render(line))
		}
	}
	for _, note := range s.Notes {
		for _, line := range diffWrapIndented("  note: "+note, width, 8) {
			rows = append(rows, Muted.Render(line))
		}
	}
	indent := strings.Repeat(" ", numbers+3)
	before, after := s.counts()
	if before == 0 {
		rows = append(rows, indent+diffMissing("before", s.BeforeAbsent))
	}
	var oldNo, newNo int
	for _, op := range s.Ops {
		switch op.Kind {
		case OpDelete:
			oldNo++
			rows = append(rows, op.rows(oldNo, numbers, width)...)
		case OpInsert:
			newNo++
			added = append(added, op.rows(newNo, numbers, width)...)
		default:
			oldNo++
			newNo++
			rows, added = append(rows, added...), nil
			rows = append(rows, op.rows(newNo, numbers, width)...)
		}
	}
	rows = append(rows, added...)
	if after == 0 {
		rows = append(rows, indent+diffMissing("after", s.AfterAbsent))
	}
	return rows
}

// counts returns how many lines each side has.
func (s DiffSection) counts() (before, after int) {
	for _, op := range s.Ops {
		if op.Kind != OpInsert {
			before++
		}
		if op.Kind != OpDelete {
			after++
		}
	}
	return before, after
}

// diffMissing marks a side that has no lines.
func diffMissing(side string, absent bool) string {
	why := "empty"
	if absent {
		why = "absent"
	}
	return diffStyle(diffSGRFaint+";"+diffSGRItalic, "("+why+" "+side+")")
}

// rows draws the line numbered no. The rows it folds into keep its color but
// not its number and mark.
func (op DiffOp) rows(no, numbers, width int) []string {
	mark, code := " ", ""
	switch op.Kind {
	case OpDelete:
		mark, code = "-", diffSGRRed
	case OpInsert:
		mark, code = "+", diffSGRGreen
	}
	number := strconv.Itoa(no)
	lead := strings.Repeat(" ", numbers-len(number)) + Muted.Render(number) + " " + diffStyle(code, mark) + " "
	var rows []string
	for _, line := range diffWrapLine(op.Text, max(width-numbers-3, 1)) {
		rows = append(rows, lead+diffStyle(code, line))
		lead = strings.Repeat(" ", numbers+3)
	}
	return rows
}

// diffNumWidth returns the digits of the file's largest line number.
func diffNumWidth(f DiffFile) int {
	largest := 1
	for _, s := range f.Sections {
		before, after := s.counts()
		largest = max(largest, before, after)
	}
	return len(strconv.Itoa(largest))
}
