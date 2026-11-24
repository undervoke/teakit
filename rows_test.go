package teakit

import (
	"fmt"
	"regexp"
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
			form.Fields = append(form.Fields, &InputField{Title: "Type", Help: "help"})
		} else {
			form.Fields = append(form.Fields, &SelectField{Title: "Choose", Options: []string{"a", "b", "c", "d"}, Cursor: 3})
		}
	}
	q.Options[10] = long
	form.Fields[10].(*InputField).Help = long
	press := func(t *testing.T, s *Step, height int, key rune, cursor func() string, check func(view string)) {
		s.Head = func(int) string { return "head" }
		m := &stepModel{s: s}
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
		form.start()
		press(t, &Step{Form: form}, height, tea.KeyTab, func() string { return "❯ " }, func(string) {})
	}
}
