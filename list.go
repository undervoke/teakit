package teakit

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	dim   = lipgloss.NewStyle().Faint(true)
	bold  = lipgloss.NewStyle().Bold(true)
	alert = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

// List is a question with numbered options, one per line. Run it with Step;
// after the step, Cursor is the option picked.
type List struct {
	Title string
	// Body is the explanation under the title; nil for none.
	Body    func(width int) string
	Options []string
	// Heads puts a blank line, and the heading unless it is empty, above the
	// option at each index.
	Heads  map[int]string
	Cursor int
	Hint   string
	// Field, when set, takes the last option's place for a value the other
	// options do not offer; FieldHint replaces Hint while the cursor is on it.
	Field     *Field
	FieldHint string

	typed string // the first digit of a two-digit option number
}

// Move returns the index delta steps from cursor among n options, wrapping
// around at both ends. n must be positive.
func Move(cursor, delta, n int) int {
	return ((cursor+delta)%n + n) % n
}

// typing reports whether the cursor is on the field.
func (l *List) typing() bool { return l.Field != nil && l.Cursor == len(l.Options)-1 }

// focusField gives the field the keys while the cursor is on it.
func (l *List) focusField() {
	switch {
	case l.Field == nil:
	case l.typing():
		l.Field.input.Focus()
	default:
		l.Field.input.Blur()
		l.Field.invalid = ""
	}
}

// typeKey handles a key while the cursor is on the field, and reports
// whether its text was accepted.
func (l *List) typeKey(m tea.KeyPressMsg) (bool, tea.Cmd) {
	switch m.String() {
	case "up", "down":
		l.press(m.String())
		l.focusField()
		return false, nil
	case "enter":
		return l.Field.check(), nil
	}
	return false, l.Field.edit(m)
}

// press moves the cursor or picks an option, and reports whether one was
// picked. A number picks its option at once; when more than nine options
// make a digit ambiguous, it moves the cursor and waits for a second digit
// or enter.
func (l *List) press(key string) bool {
	typed := l.typed
	l.typed = ""
	switch key {
	case "up", "k":
		l.Cursor = Move(l.Cursor, -1, len(l.Options))
	case "down", "j":
		l.Cursor = Move(l.Cursor, 1, len(l.Options))
	case "enter":
		return true
	default:
		if len(key) != 1 || key[0] < '0' || key[0] > '9' {
			return false
		}
		n, _ := strconv.Atoi(typed + key)
		if n < 1 || n > len(l.Options) {
			return false
		}
		l.Cursor = n - 1
		if n*10 <= len(l.Options) {
			l.typed = strconv.Itoa(n)
			return false
		}
		// The field's number moves there for typing.
		return !l.typing()
	}
	return false
}

func (l *List) view(width int) string {
	var b strings.Builder
	b.WriteString(bold.Render(Wrap(l.Title, width)) + "\n")
	if l.Body != nil {
		b.WriteString(l.Body(width) + "\n")
	}
	digits := len(strconv.Itoa(len(l.Options)))
	for i, o := range l.Options {
		if head, ok := l.Heads[i]; ok || i == 0 {
			b.WriteString("\n")
			if head != "" {
				b.WriteString(head + "\n")
			}
		}
		number := fmt.Sprintf("%*d. ", digits, i+1)
		indent := 2 + len(number)
		if l.Field != nil && i == len(l.Options)-1 {
			prefix := "  " + number
			if i == l.Cursor {
				prefix = bold.Render("❯ " + number)
			}
			b.WriteString(prefix + l.Field.view() + "\n")
			if l.Field.invalid != "" {
				pad := strings.Repeat(" ", indent)
				b.WriteString(alert.Render(pad+strings.ReplaceAll(Wrap(l.Field.invalid, max(width-indent, 1)), "\n", "\n"+pad)) + "\n")
			}
			continue
		}
		if room := width - indent; room > 0 && lipgloss.Width(o) > room {
			o = strings.ReplaceAll(Wrap(o, room), "\n", "\n"+strings.Repeat(" ", indent))
		}
		line := number + o
		if i == l.Cursor {
			line = bold.Render("❯ " + line)
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}
	hint := l.Hint
	if l.typing() && l.FieldHint != "" {
		hint = l.FieldHint
	}
	return b.String() + "\n" + dim.Render(Wrap(hint, width))
}

// Wrap folds text to width at spaces, continuing each folded line at its own
// indent. Paths such as ~/Library/Application Support/self-hosted-gha stay
// whole, which lipgloss's wrapping would break at the hyphen; only a word
// wider than the column is split, since the screen would cut it off.
func Wrap(text string, width int) string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
		row := indent
		for _, word := range strings.Fields(line) {
			if row != indent && lipgloss.Width(row+" "+word) > width {
				out, row = append(out, row), indent
			}
			for room := width - len(indent); room > 0 && lipgloss.Width(word) > room; {
				r := []rune(word)
				n := min(room, len(r))
				out, word = append(out, indent+string(r[:n])), string(r[n:])
			}
			if row != indent {
				row += " "
			}
			row += word
		}
		out = append(out, row)
	}
	return strings.Join(out, "\n")
}
