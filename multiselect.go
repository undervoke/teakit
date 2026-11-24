package teakit

// MultiSelect is a question with numbered options, one per line, that takes
// any number of them. Space checks or unchecks the option at the cursor, a
// number only moves there, and enter answers. Run it with Step; after the
// step, Checked is the answer. Options that do not fit the terminal scroll
// with the cursor.
type MultiSelect struct {
	Title string
	// Body is the explanation under the title; nil for none.
	Body    func(width int) string
	Options []string
	// Checked has one entry per option, which the user's changes set in place.
	Checked []bool
	// Accept, when set, checks the answer on enter. Its error is shown under
	// the options until the next key.
	Accept func(checked []bool) error
	Cursor int
	Hint   string

	typed   string // the first digit of a two-digit option number
	invalid string // why Accept rejected the answer
}

func (m *MultiSelect) start() { m.invalid = "" }

// press moves the cursor or checks an option, and reports whether the
// question was answered.
func (m *MultiSelect) press(key string) bool {
	m.invalid = ""
	moveKey(key, &m.Cursor, &m.typed, len(m.Options))
	switch {
	case key == "enter":
		return m.accepted()
	case key == "space" && len(m.Options) > 0:
		m.Checked[m.Cursor] = !m.Checked[m.Cursor]
	}
	return false
}

// accepted runs Accept on the answer and keeps why it was rejected.
func (m *MultiSelect) accepted() bool {
	if m.Accept == nil {
		return true
	}
	if err := m.Accept(m.Checked); err != nil {
		m.invalid = err.Error()
		return false
	}
	return true
}

func (m *MultiSelect) list() optionList {
	return optionList{title: m.Title, body: m.Body, options: m.Options, checked: m.Checked, cursor: m.Cursor, hint: m.Hint, invalid: m.invalid}
}
