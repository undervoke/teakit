package teakit

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

var (
	optionRow = regexp.MustCompile(`(?m)^[❯ ] +\d+\. `)
	hiddenRow = regexp.MustCompile(`(?m)^  [↑↓] (\d+) more`)
)

// TestScrolledScreenShowsCursor catches scrolling that is off by a row, so
// that keys act on an option or a field that is not on screen, the frame is
// taller than the terminal and loses rows, or the counts of hidden options
// are wrong. Delete it with rows.window.
func TestScrolledScreenShowsCursor(t *testing.T) {
	const items = 20
	long := strings.Repeat("long ", 25) // more rows than the shortest screens have
	q := &Select{Title: "Pick", Hint: "hint", Heads: map[int]string{0: "First", 7: "Second"}}
	form := &Form{Title: "Fill in", Hint: "hint"}
	for i := range items {
		q.Options = append(q.Options, fmt.Sprint("option ", i+1))
		if i%2 == 0 {
			form.Fields = append(form.Fields, &InputField{Title: "Type"})
		} else {
			form.Fields = append(form.Fields, &SelectField{Title: "Choose", Options: []string{"a", "b", "c", "d"}, Choice: 3})
		}
	}
	q.Options[10] = long
	form.Fields[11].(*SelectField).Help = long
	press := func(t *testing.T, s *Step, height int, key rune, cursor func() string, check func(view string)) {
		s.Head = func(int) string { return "head" }
		m := &StepModel{s: s}
		m.Update(tea.WindowSizeMsg{Width: 40, Height: height})
		for range items {
			m.Update(tea.KeyPressMsg{Code: key})
			view := ansi.Strip(m.View())
			if rows := strings.Count(view, "\n") + 1; rows > height || !strings.Contains(view, cursor()) {
				t.Fatalf("%d rows for height %d, want %q in:\n%s", rows, height, cursor(), view)
			}
			check(view)
		}
	}
	for height := 1; height <= 35; height++ {
		press(t, &Step{Select: q}, height, tea.KeyDown, func() string { return fmt.Sprintf("❯ %2d. ", q.Cursor+1) }, func(view string) {
			counted := len(optionRow.FindAllString(view, -1))
			for _, hidden := range hiddenRow.FindAllStringSubmatch(view, -1) {
				n, _ := strconv.Atoi(hidden[1])
				counted += n
			}
			// Under three rows there is no room for the counts.
			if height >= 3 && counted != items {
				t.Fatalf("%d of %d options shown or counted for height %d in:\n%s", counted, items, height, view)
			}
		})
		// Tab moves over the fields; enter opens and locks each.
		for _, key := range []rune{tea.KeyTab, tea.KeyEnter} {
			form.start()
			press(t, &Step{Form: form}, height, key, func() string { return "❯ " }, func(string) {})
		}
	}
}

// TestDiffScrollShowsEveryLine catches the form drawing a scrolled diff in
// other rows than the diff counts on, so that it goes on to the next field
// with lines never shown, or draws more rows than the terminal has. Delete it
// with rows.scrolled.
func TestDiffScrollShowsEveryLine(t *testing.T) {
	const lines = 25
	var ops []DiffOp
	for i := range lines {
		ops = append(ops, DiffOp{Kind: OpInsert, Text: fmt.Sprintf("line %02d.", i+1)})
	}
	diff := &DiffField{Files: []DiffFile{{Path: "file", Sections: []DiffSection{{Label: "part", Ops: ops}}}}}
	for height := 1; height <= lines+12; height++ {
		for _, fields := range [][]Field{{diff}, {diff, &InputField{Title: "Name"}}, {&InputField{Title: "Name"}, diff}} {
			form := &Form{Title: "Review", Hint: "hint", Fields: fields}
			m := NewStepModel(&Step{Form: form, Head: func(int) string { return "head" }})
			m.Update(tea.WindowSizeMsg{Width: 40, Height: height})
			if !form.single() {
				form.move(slices.Index(fields, Field(diff)))
				form.enter()
			}
			seen := map[string]bool{}
			for range lines + 5 {
				if !form.open {
					break
				}
				view := ansi.Strip(m.View())
				if rows := strings.Count(view, "\n") + 1; rows > height {
					t.Fatalf("%d rows for height %d in:\n%s", rows, height, view)
				}
				for _, line := range regexp.MustCompile(`line \d\d\.`).FindAllString(view, -1) {
					seen[line] = true
				}
				m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
			}
			if len(seen) != lines {
				t.Fatalf("%d of %d lines shown for height %d among %d fields", len(seen), lines, height, len(fields))
			}
		}
	}
}

// TestHeldEnterAppliesOneDiff catches a key path that loses IsRepeat, so that
// holding Enter applies diffs the user has not seen, and submits them. Delete
// it with DiffField.
func TestHeldEnterAppliesOneDiff(t *testing.T) {
	first, last := &DiffField{Files: diffTestFiles()}, &DiffField{Files: diffTestFiles()}
	m := NewStepModel(&Step{Form: &Form{Fields: []Field{first, last}}})
	press := func(held bool) {
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter, IsRepeat: held})
	}
	press(false) // opens the first
	press(false)
	press(true)
	if !first.Apply || last.Apply {
		t.Fatalf("applied %t and %t", first.Apply, last.Apply)
	}
	press(false)
	press(true)
	if m.Done() {
		t.Fatal("submitted")
	}
}
