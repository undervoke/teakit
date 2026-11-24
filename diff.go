package teakit

import (
	"fmt"
	"strconv"
	"strings"

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

// RenderDiff draws files as a Diff does, for text to print. Widths below 1
// default to 80 columns.
func RenderDiff(files []DiffFile, width int) string {
	if width < 1 {
		width = 80
	}
	rows, _ := diffRows(files, width)
	return strings.Join(rows, "\n")
}

// span is the rows from one up to another.
type span struct{ from, to int }

// diffRows draws files one after another, and returns where each one's head is.
func diffRows(files []DiffFile, width int) (rows []string, heads []span) {
	for i, f := range files {
		place := ""
		if len(files) > 1 {
			place = fmt.Sprintf("%d/%d", i+1, len(files))
		}
		if i > 0 {
			rows = append(rows, "")
		}
		head := span{from: len(rows)}
		rows = append(rows, f.head(place, width)...)
		head.to = len(rows)
		heads = append(heads, head)
		numbers := diffNumWidth(f)
		for _, s := range f.Sections {
			rows = append(rows, s.rows(width, numbers)...)
		}
	}
	return rows, heads
}

// head draws the file's path, and beside it the file's place among the others
// and how much changes in it.
func (f DiffFile) head(place string, width int) []string {
	var rows []string
	if f.Path != "" {
		for _, line := range diffWrapIndented(f.Path, width, 2) {
			rows = append(rows, diffStyle(diffSGRBold, line))
		}
	}
	return beside(rows, strings.TrimSpace(Muted.Render(place)+"  "+diffCounts(f)), width)
}

// diffCounts draws how many lines the files gain and lose.
func diffCounts(files ...DiffFile) string {
	var added, removed int
	for _, f := range files {
		for _, s := range f.Sections {
			for _, op := range s.Ops {
				switch op.Kind {
				case OpInsert:
					added++
				case OpDelete:
					removed++
				}
			}
		}
	}
	var counts []string
	if added > 0 {
		counts = append(counts, diffStyle(diffSGRGreen, fmt.Sprintf("+%d", added)))
	}
	if removed > 0 {
		counts = append(counts, diffStyle(diffSGRRed, fmt.Sprintf("-%d", removed)))
	}
	return strings.Join(counts, " ")
}

// beside puts text after the last of rows, or under it where it does not fit.
func beside(rows []string, text string, width int) []string {
	last := len(rows) - 1
	switch {
	case text == "":
	case last < 0:
		rows = append(rows, text)
	case ansi.StringWidth(rows[last]+"  "+text) <= width:
		rows[last] += "  " + text
	default:
		rows = append(rows, "  "+text)
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
