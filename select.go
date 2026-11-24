package teakit

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var (
	dim   = lipgloss.NewStyle().Faint(true)
	bold  = lipgloss.NewStyle().Bold(true)
	alert = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

// Select is a question with numbered options, one per line, that takes one of
// them. Run it with Step; after the step, Cursor is the option picked.
// Options that do not fit the terminal scroll with the cursor.
type Select struct {
	Title string
	// Body is the explanation under the title; nil for none.
	Body    func(width int) string
	Options []string
	// Heads puts a blank line, and the heading unless it is empty, above the
	// option at each index.
	Heads  map[int]string
	Cursor int
	Hint   string
	// Input, when set, takes the last option's place for a value the other
	// options do not offer; InputHint replaces Hint while the cursor is on it.
	Input     *Input
	InputHint string

	typed string // the first digit of a two-digit option number
}

// Move returns the index delta steps from cursor among n options, wrapping
// around at both ends. n must be positive.
func Move(cursor, delta, n int) int {
	return ((cursor+delta)%n + n) % n
}

// typing reports whether the cursor is on the input.
func (s *Select) typing() bool { return s.Input != nil && s.Cursor == len(s.Options)-1 }

func (s *Select) start() {
	if s.Input != nil {
		s.Input.start()
	}
	s.focusInput()
}

// focusInput gives the input the keys while the cursor is on it.
func (s *Select) focusInput() {
	switch {
	case s.Input == nil:
	case s.typing():
		s.Input.text.Focus()
	default:
		s.Input.text.Blur()
		s.Input.text.invalid = ""
	}
}

// typeKey handles a key while the cursor is on the input, and reports
// whether its text was accepted.
func (s *Select) typeKey(m tea.KeyPressMsg) (bool, tea.Cmd) {
	switch m.String() {
	case "up", "down":
		s.press(m.String())
		return false, nil
	case "enter":
		return s.Input.check(), nil
	}
	return false, s.Input.edit(m)
}

// press moves the cursor or picks an option, and reports whether one was
// picked. A number picks its option at once, unless moveKey waits for its
// second digit or it is the input's, which moves there for typing.
func (s *Select) press(key string) bool {
	numbered := moveKey(key, &s.Cursor, &s.typed, len(s.Options))
	s.focusInput()
	if key == "enter" {
		return len(s.Options) > 0
	}
	return numbered && !s.typing()
}

// moveKey moves cursor among n options for an arrow key or a number, and
// reports whether the number is complete. When more than nine options make a
// digit ambiguous, it moves the cursor and keeps the digit in typed until the
// next key.
func moveKey(key string, cursor *int, typed *string, n int) bool {
	first := *typed
	*typed = ""
	if n == 0 {
		return false
	}
	switch key {
	case "up", "k":
		*cursor = Move(*cursor, -1, n)
	case "down", "j":
		*cursor = Move(*cursor, 1, n)
	default:
		if len(key) != 1 || key[0] < '0' || key[0] > '9' {
			return false
		}
		number, _ := strconv.Atoi(first + key)
		if number < 1 || number > n {
			return false
		}
		*cursor = number - 1
		if number*10 <= n {
			*typed = strconv.Itoa(number)
			return false
		}
		return true
	}
	return false
}

func (s *Select) list() optionList {
	l := optionList{title: s.Title, body: s.Body, options: s.Options, heads: s.Heads, cursor: s.Cursor, hint: s.Hint, input: s.Input}
	if s.typing() && s.InputHint != "" {
		l.hint = s.InputHint
	}
	return l
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
			if room := width - len(indent); room > 0 && lipgloss.Width(word) > room {
				// Hardwrap only inserts line breaks, by cells, past escapes.
				pieces := strings.Split(ansi.Hardwrap(word, room, true), "\n")
				for _, piece := range pieces[:len(pieces)-1] {
					out = append(out, indent+piece)
				}
				word = pieces[len(pieces)-1]
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
