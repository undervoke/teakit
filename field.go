package teakit

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// FieldConfig supplies a List field's starting value, its placeholder, the
// unit shown after it, and the check of its text on enter.
type FieldConfig struct {
	Value       string
	Placeholder string
	Unit        string
	Accept      func(text string) error
}

// Field is the typed field that takes a List's last option. Its unit follows
// the text as a fixed suffix, so only the number is typed; a unit typed
// after it anyway is dropped.
type Field struct {
	input   textinput.Model
	unit    string
	accept  func(string) error
	invalid string // why accept rejected the text
}

// NewField returns a field starting with cfg.Value, without its unit.
func NewField(cfg FieldConfig) *Field {
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = cfg.Placeholder
	st := in.Styles()
	st.Focused.Text, st.Blurred.Text = lipgloss.NewStyle(), lipgloss.NewStyle()
	st.Focused.Placeholder, st.Blurred.Placeholder = dim, dim
	st.Cursor.Blink = false
	in.SetStyles(st)
	f := &Field{input: in, unit: cfg.Unit, accept: cfg.Accept}
	f.input.SetValue(f.trimUnit(cfg.Value))
	f.fit()
	return f
}

// Value returns the text, without a unit typed after it.
func (f *Field) Value() string { return f.trimUnit(f.input.Value()) }

func (f *Field) trimUnit(text string) string {
	if f.unit == "" {
		return text
	}
	text = strings.TrimSpace(text)
	if n := len(text) - len(f.unit); n >= 0 && strings.EqualFold(text[n:], f.unit) {
		text = strings.TrimSpace(text[:n])
	}
	return text
}

// fit sizes the input to its text, since textinput pads to its width and the
// unit has to follow the text. Empty, it needs the placeholder's width, or it
// shows only the placeholder's first letter.
func (f *Field) fit() {
	w := 0
	if f.input.Value() == "" {
		w = lipgloss.Width(f.input.Placeholder)
	}
	f.input.SetWidth(w)
	// Recompute the visible span for the new width.
	f.input.SetCursor(f.input.Position())
}

// check runs accept on the text and keeps why it was rejected. Without
// accept, any text is taken.
func (f *Field) check() bool {
	if f.accept == nil {
		return true
	}
	if err := f.accept(f.Value()); err != nil {
		f.invalid = err.Error()
		return false
	}
	return true
}

// edit passes msg to the input. A key or a change to the text clears why
// the text was rejected.
func (f *Field) edit(msg tea.Msg) tea.Cmd {
	before := f.input.Value()
	var cmd tea.Cmd
	f.input, cmd = f.input.Update(msg)
	f.fit()
	if _, key := msg.(tea.KeyPressMsg); key || f.input.Value() != before {
		f.invalid = ""
	}
	return cmd
}

func (f *Field) view() string {
	if f.unit == "" {
		return f.input.View()
	}
	unit := f.unit
	// At the end, the cursor's cell already separates the unit.
	if f.input.Position() < len([]rune(f.input.Value())) {
		unit = " " + unit
	}
	return f.input.View() + dim.Render(unit)
}
