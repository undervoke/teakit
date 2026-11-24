package teakit

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// typedText is the text of an Input or an InputField as it is edited.
type typedText struct {
	textinput.Model
	invalid string // why the text was rejected
}

func newTypedText(value, placeholder string) typedText {
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = placeholder
	st := in.Styles()
	st.Focused.Text, st.Blurred.Text = lipgloss.NewStyle(), lipgloss.NewStyle()
	st.Focused.Placeholder, st.Blurred.Placeholder = dim, dim
	st.Cursor.Blink = false
	in.SetStyles(st)
	in.SetValue(value)
	return typedText{Model: in}
}

// edit passes msg to the text. A key or a change to the text clears why it
// was rejected.
func (t *typedText) edit(msg tea.Msg) tea.Cmd {
	before := t.Value()
	var cmd tea.Cmd
	t.Model, cmd = t.Update(msg)
	if _, key := msg.(tea.KeyPressMsg); key || t.Value() != before {
		t.invalid = ""
	}
	return cmd
}

// check runs accept on value, the text's answer, and keeps why it was
// rejected. Without accept, any text is taken.
func (t *typedText) check(accept func(string) error, value string) bool {
	t.invalid = ""
	if accept != nil {
		if err := accept(value); err != nil {
			t.invalid = err.Error()
			return false
		}
	}
	return true
}

// Input is the typed answer that takes a Select's last option. Its unit
// follows the text as a fixed suffix, so only the number is typed; a unit
// typed after it anyway is dropped.
type Input struct {
	// Value is the text without its unit, which typing changes in place.
	Value       string
	Placeholder string
	Unit        string
	// Accept, when set, checks the text on enter. Its error is shown under
	// the text until the next key.
	Accept func(text string) error

	text typedText
}

// start readies the input for a new showing.
func (in *Input) start() {
	in.Value = in.trimUnit(in.Value)
	in.text = newTypedText(in.Value, in.Placeholder)
	in.fit()
}

func (in *Input) trimUnit(text string) string {
	if in.Unit == "" {
		return text
	}
	text = strings.TrimSpace(text)
	if n := len(text) - len(in.Unit); n >= 0 && strings.EqualFold(text[n:], in.Unit) {
		text = strings.TrimSpace(text[:n])
	}
	return text
}

// fit sizes the input to its text, since textinput pads to its width and the
// unit has to follow the text. Empty, it needs the placeholder's width, or it
// shows only the placeholder's first letter.
func (in *Input) fit() {
	w := 0
	if in.text.Value() == "" {
		w = lipgloss.Width(in.Placeholder)
	}
	in.text.SetWidth(w)
	// Recompute the visible span for the new width.
	in.text.SetCursor(in.text.Position())
}

func (in *Input) check() bool { return in.text.check(in.Accept, in.Value) }

func (in *Input) edit(msg tea.Msg) tea.Cmd {
	cmd := in.text.edit(msg)
	in.Value = in.trimUnit(in.text.Value())
	in.fit()
	return cmd
}

func (in *Input) view() string {
	if in.Unit == "" {
		return in.text.View()
	}
	unit := in.Unit
	// At the end, the cursor's cell already separates the unit.
	if in.text.Position() < len([]rune(in.text.Value())) {
		unit = " " + unit
	}
	return in.text.View() + dim.Render(unit)
}
