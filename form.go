package teakit

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Form is a screen of fields, filled in one at a time. Enter or tab moves to
// the next field once the current one's answer is accepted, shift+tab moves
// to the previous one, and enter on the last field submits the form. Run it
// with Step; after the step, each field holds its answer. Fields that do not
// fit the terminal scroll with the cursor.
type Form struct {
	// Title is shown above the fields; empty for none.
	Title string
	// Body is the explanation under the title; nil for none.
	Body   func(width int) string
	Fields []Field
	Hint   string

	at int // the field that has the keys
}

// Field is a field of a Form: an InputField or a SelectField.
type Field interface {
	// start readies the field for a new showing.
	start()
	// focus gives the field the keys, or takes them away.
	focus(on bool)
	resize(width int)
	update(msg tea.Msg) tea.Cmd
	// accepted checks the answer and keeps why it was rejected.
	accepted() bool
	// draw returns the field's rows and which of them the cursor is on.
	draw(width int, focused bool) (rows []string, cursor int)
}

func (f *Form) start() {
	f.at = 0
	for i, field := range f.Fields {
		field.start()
		field.focus(i == 0)
	}
}

func (f *Form) resize(width int) {
	for _, field := range f.Fields {
		field.resize(width)
	}
}

func (f *Form) move(to int) {
	f.Fields[f.at].focus(false)
	f.at = to
	f.Fields[f.at].focus(true)
}

// update handles a message and reports whether the form was submitted.
func (f *Form) update(msg tea.Msg) (bool, tea.Cmd) {
	key := ""
	if m, ok := msg.(tea.KeyPressMsg); ok {
		key = m.String()
	}
	last := f.at >= len(f.Fields)-1
	switch {
	case key == "enter" && last:
		return f.submit(), nil
	case key == "tab" && last, key == "shift+tab" && f.at == 0, len(f.Fields) == 0:
	case key == "enter", key == "tab":
		if f.Fields[f.at].accepted() {
			f.move(f.at + 1)
		}
	case key == "shift+tab":
		f.move(f.at - 1)
	default:
		return false, f.Fields[f.at].update(msg)
	}
	return false, nil
}

// submit checks every field again, since an answer can be rejected for what
// another field holds, and moves to the first rejected one.
func (f *Form) submit() bool {
	for i, field := range f.Fields {
		if !field.accepted() {
			f.move(i)
			return false
		}
	}
	return true
}

func (f *Form) draw(width int) rows {
	var r rows
	if f.Title != "" {
		r.above = strings.Split(bold.Render(Wrap(f.Title, width)), "\n")
	}
	if f.Body != nil {
		r.above = append(r.above, strings.Split(f.Body(width), "\n")...)
	}
	if len(r.above) > 0 {
		r.above = append(r.above, "")
	}
	for i, field := range f.Fields {
		if i > 0 {
			r.items = append(r.items, "")
		}
		drawn, cursor := field.draw(width, i == f.at)
		if i == f.at {
			r.cursor, r.row = i, len(r.items)+cursor
		}
		r.add(drawn)
	}
	if f.Hint != "" {
		r.below = append([]string{""}, strings.Split(dim.Render(Wrap(f.Hint, width)), "\n")...)
	}
	return r
}

// drawField draws what is above a field's answer.
func drawField(title, help string, width int, focused bool) []string {
	title = Wrap(title, width)
	if focused {
		title = bold.Render(title)
	}
	rows := strings.Split(title, "\n")
	if help != "" {
		rows = append(rows, strings.Split(dim.Render(Wrap(help, width)), "\n")...)
	}
	return rows
}

// InputField is a Form's field for typing text.
type InputField struct {
	Title string
	Help  string
	// HelpFunc, when set, gives the help for the text typed so far, in place
	// of Help.
	HelpFunc func(text string) string
	// Value is the text, which typing changes in place.
	Value string
	// Accept, when set, checks the text when the user moves on from it and
	// when they submit the form. Its error is shown under the text until the
	// next key.
	Accept func(text string) error

	text typedText
	help string // what HelpFunc gave for Value
}

func (f *InputField) start() {
	f.text = newTypedText(f.Value, "")
	f.typed()
}

// typed takes the text as it now stands.
func (f *InputField) typed() {
	f.Value = f.text.Value()
	f.help = f.Help
	if f.HelpFunc != nil {
		f.help = f.HelpFunc(f.Value)
	}
}

func (f *InputField) focus(on bool) {
	if on {
		f.text.Focus()
	} else {
		f.text.Blur()
	}
}

func (f *InputField) resize(width int) {
	// The marker takes two columns, and the cursor one past the text's end.
	f.text.SetWidth(max(width-3, 1))
	// Recompute the visible span for the new width, from the end, since
	// textinput keeps a span that still holds the cursor.
	at := f.text.Position()
	f.text.CursorEnd()
	f.text.SetCursor(at)
}

func (f *InputField) update(msg tea.Msg) tea.Cmd {
	cmd := f.text.edit(msg)
	if f.text.Value() != f.Value {
		f.typed()
	}
	return cmd
}

func (f *InputField) accepted() bool { return f.text.check(f.Accept, f.Value) }

func (f *InputField) draw(width int, focused bool) ([]string, int) {
	rows := drawField(f.Title, f.help, width, focused)
	cursor := len(rows)
	marker := "> "
	if focused {
		marker = bold.Render("❯ ")
	}
	rows = append(rows, marker+f.text.View())
	if f.text.invalid != "" {
		rows = append(rows, strings.Split(alert.Render(Wrap(f.text.invalid, width)), "\n")...)
	}
	return rows, cursor
}

// SelectField is a Form's field that takes one of its options. It shows all
// of them while it has the keys, and otherwise the chosen one.
type SelectField struct {
	Title   string
	Help    string
	Options []string
	// Cursor is the option chosen, which the arrow keys change in place.
	Cursor int
}

func (f *SelectField) start()         {}
func (f *SelectField) focus(bool)     {}
func (f *SelectField) resize(int)     {}
func (f *SelectField) accepted() bool { return true }

func (f *SelectField) update(msg tea.Msg) tea.Cmd {
	if m, ok := msg.(tea.KeyPressMsg); ok && len(f.Options) > 0 {
		switch m.String() {
		case "up", "k":
			f.Cursor = Move(f.Cursor, -1, len(f.Options))
		case "down", "j":
			f.Cursor = Move(f.Cursor, 1, len(f.Options))
		}
	}
	return nil
}

func (f *SelectField) draw(width int, focused bool) ([]string, int) {
	rows := drawField(f.Title, f.Help, width, focused)
	cursor := len(rows)
	for i, option := range f.Options {
		option = strings.ReplaceAll(Wrap(option, max(width-2, 1)), "\n", "\n  ")
		switch {
		case i == f.Cursor && focused:
			cursor = len(rows)
			option = bold.Render("❯ " + option)
		case i == f.Cursor:
			option = "> " + option
		case focused:
			option = "  " + option
		default:
			continue
		}
		rows = append(rows, strings.Split(option, "\n")...)
	}
	return rows, cursor
}
