package teakit

import (
	"cmp"
	"math"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// DiffField is a Form's field that applies changes or skips them. While it is
// open, up and down scroll it, and past its ends go on to the neighboring
// field; enter or tab locks applying it, and backspace goes back to skipping.
type DiffField struct {
	// Title is shown above the changes; empty for the first file's path.
	Title string
	Files []DiffFile
	// Apply is the answer, which changes in place.
	Apply bool
	// ApplyLabel and SkipLabel name the answers; they default to "apply" and
	// "skip".
	ApplyLabel, SkipLabel string
	// Accept, when set, checks the answer when the user locks it and when
	// they submit the form. Its error is shown under the answer.
	Accept func(apply bool) error
	// Hint replaces the form's OpenHint while the field is open.
	Hint string

	title   string
	changes []string // the rows of the changes
	offset  int      // the first change shown while the field is open and does not fit
	invalid string
}

func (f *DiffField) start() { f.invalid = "" }

func (f *DiffField) resize(width int) {
	f.title = f.Title
	files := f.Files
	if f.title == "" && len(files) > 0 {
		// A single row, with control characters made visible.
		f.title = strings.Join(diffWrapLine(files[0].Path, math.MaxInt), "")
		files = slices.Clone(files)
		files[0].Path = ""
	}
	f.changes = diffRows(files, max(width-answerIndent, 1))
}

func (f *DiffField) opens(tea.Msg) bool { return false }
func (f *DiffField) open()              { f.offset = 0 }
func (f *DiffField) shut()              {}
func (f *DiffField) undo()              {}

func (f *DiffField) update(msg tea.Msg, by scroll) (fieldEnd, tea.Cmd) {
	m, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return fieldStays, nil
	}
	// The terminal may have grown.
	f.offset = min(f.offset, by.last)
	switch m.String() {
	case "up", "k":
		if f.offset == 0 {
			return fieldUp, nil
		}
		f.offset--
	case "down", "j":
		if f.offset == by.last {
			return fieldDown, nil
		}
		f.offset++
	case "pgup":
		f.offset -= by.page
	case "pgdown":
		f.offset += by.page
	case "home", "g":
		f.offset = 0
	case "end", "G":
		f.offset = by.last
	case "backspace":
		f.Apply, f.invalid = false, ""
	}
	f.offset = min(max(f.offset, 0), by.last)
	return fieldStays, nil
}

func (f *DiffField) lock() bool {
	was := f.Apply
	f.Apply = true
	if !f.accepted() {
		f.Apply = was
		return false
	}
	return true
}

func (f *DiffField) accepted() bool {
	f.invalid = ""
	if f.Accept != nil {
		if err := f.Accept(f.Apply); err != nil {
			f.invalid = err.Error()
		}
	}
	return f.invalid == ""
}

func (f *DiffField) draw(width int, state fieldState) ([]string, int) {
	rows := drawField(f.title, "", width, state)
	// Alone, the field has no answer but submitting the form.
	if state != fieldOnly {
		rows = append(rows, f.answer())
	}
	rows = append(rows, drawUnder(Error, f.invalid, width)...)
	gutter := strings.Repeat(" ", answerIndent)
	if state == fieldOpen {
		gutter = Header.Render("  │") + " "
	}
	for _, row := range f.changes {
		rows = append(rows, gutter+row)
	}
	return rows, 0
}

// answer draws what is locked.
func (f *DiffField) answer() string {
	if f.Apply {
		return Chosen.Render("  > " + cmp.Or(f.ApplyLabel, "apply"))
	}
	return Muted.Render("    " + cmp.Or(f.SkipLabel, "skip"))
}
