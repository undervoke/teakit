package teakit

import (
	"cmp"
	"fmt"
	"math"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Form is a screen of fields above a Submit row. Up and down, or shift+tab and
// tab, move between them. Enter opens the field at the cursor, as typing does
// for an InputField and a number for a SelectField. In an open field, enter or
// tab locks an accepted answer and opens the next field, and esc undoes what
// was changed and shuts it. Submitting checks every answer. A form of one
// field keeps it open and has no Submit row: enter locks and submits it. Run
// it with Step; after the step, each field holds its answer. Fields that do
// not fit the terminal scroll with the cursor.
type Form struct {
	// Title is shown above the fields; empty for none.
	Title string
	// Body is the explanation under the title; nil for none.
	Body   func(width int) string
	Fields []Field
	Hint   string
	// OpenHint replaces Hint while a field is open.
	OpenHint string

	at   int // the field at the cursor, or len(Fields) for the Submit row
	open bool
}

// Field is a field of a Form: an InputField, a SelectField or a DiffField.
type Field interface {
	// start readies the field for a new showing.
	start()
	// opens reports whether msg opens the field, which then handles it.
	opens(msg tea.Msg) bool
	// open gives the field the keys, and shut takes them away.
	open()
	shut()
	resize(width int)
	// update handles a message while the field is open and scrolls as by says.
	update(msg tea.Msg, by scroll) (fieldEnd, tea.Cmd)
	// lock takes the answer unless it is rejected.
	lock() bool
	// undo puts back the answer the field had when it was opened.
	undo()
	// accepted checks the answer and keeps why it was rejected.
	accepted() bool
	// draw returns the field's rows and which of them the cursor is on.
	draw(width int, state fieldState) (rows []string, cursor int)
}

type fieldState int

const (
	fieldAway fieldState = iota // the cursor is elsewhere
	fieldHere                   // the cursor is on the field
	fieldOpen                   // the cursor is inside it
	fieldOnly                   // it is the form's one field, which stays open
)

// fieldEnd is what a message does to an open field.
type fieldEnd int

const (
	fieldStays fieldEnd = iota
	fieldLocks          // locks its answer
	fieldUp             // goes on to the field above
	fieldDown           // goes on to the field below
)

// single reports that the form is one field, which stays open.
func (f *Form) single() bool { return len(f.Fields) == 1 }

// editing reports whether esc undoes the open field's change.
func (f *Form) editing() bool { return f.open && !f.single() }

func (f *Form) start() {
	f.at, f.open = 0, false
	for _, field := range f.Fields {
		field.start()
	}
	if f.single() {
		f.enter()
	}
}

func (f *Form) resize(width int) {
	for _, field := range f.Fields {
		field.resize(width)
	}
}

// enter opens the field at the cursor.
func (f *Form) enter() {
	f.open = true
	f.Fields[f.at].open()
}

// move shuts an open field and moves the cursor to the field or row at to.
func (f *Form) move(to int) {
	if f.open {
		f.open = false
		f.Fields[f.at].shut()
	}
	f.at = min(max(to, 0), len(f.Fields))
}

// update handles a message while an open field scrolls as by says, and reports
// whether the form was submitted.
func (f *Form) update(msg tea.Msg, by scroll) (bool, tea.Cmd) {
	key, held := "", false
	if m, ok := msg.(tea.KeyPressMsg); ok {
		key, held = m.String(), m.Key().IsRepeat
	}
	if f.open {
		return f.updateOpen(key, held, msg, by)
	}
	switch key {
	case "up", "shift+tab":
		f.move(f.at - 1)
	case "down", "tab":
		f.move(f.at + 1)
	case "enter":
		if f.at == len(f.Fields) {
			// A held key that applied the last diff must not submit it.
			return !held && f.submit(), nil
		}
		f.enter()
	default:
		if f.at < len(f.Fields) && f.Fields[f.at].opens(msg) {
			f.enter()
			return f.updateOpen(key, held, msg, by)
		}
	}
	return false, nil
}

func (f *Form) updateOpen(key string, held bool, msg tea.Msg, by scroll) (bool, tea.Cmd) {
	field := f.Fields[f.at]
	diff, _ := field.(*DiffField)
	end := fieldStays
	var cmd tea.Cmd
	switch {
	case f.single() && (key == "tab" || key == "shift+tab"):
	case key == "enter", key == "tab":
		// A held key must not apply one diff after another.
		if diff == nil || !held {
			end = fieldLocks
		}
	case key == "shift+tab":
		f.move(f.at - 1)
	case key == "esc":
		field.undo()
		f.move(f.at)
	default:
		end, cmd = field.update(msg, by)
	}
	switch {
	case end == fieldLocks && field.lock():
		if f.single() {
			return true, cmd
		}
		f.move(f.at + 1)
		if f.at < len(f.Fields) {
			f.enter()
		}
	case end == fieldUp && f.at > 0:
		f.read(f.at - 1)
		if above, ok := f.Fields[f.at].(*DiffField); ok {
			above.offset = math.MaxInt // its end
		}
	case end == fieldDown && !f.single():
		f.read(f.at + 1)
	}
	return false, cmd
}

// read moves the cursor to the field or row at to, and opens a diff there, so
// that one diff after another scrolls by.
func (f *Form) read(to int) {
	f.move(to)
	if f.at < len(f.Fields) {
		if _, ok := f.Fields[f.at].(*DiffField); ok {
			f.enter()
		}
	}
}

// submit checks every answer again, since one can be rejected for what
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
		r.above = strings.Split(Header.Render(Wrap(f.Title, width)), "\n")
	}
	if f.Body != nil {
		r.above = append(r.above, strings.Split(f.Body(width), "\n")...)
	}
	if len(r.above) > 0 {
		r.above = append(r.above, "")
	}
	add := func(i int, item []string, cursor int) {
		if i > 0 {
			r.items = append(r.items, "")
		}
		if i == f.at {
			r.cursor, r.row = i, len(r.items)+cursor
		}
		r.add(item)
	}
	for i, field := range f.Fields {
		state := fieldAway
		switch {
		case f.single():
			state = fieldOnly
		case i == f.at && f.open:
			state = fieldOpen
		case i == f.at:
			state = fieldHere
		}
		item, cursor := field.draw(width, state)
		add(i, item, cursor)
		if diff, ok := field.(*DiffField); ok && i == f.at && f.open {
			r.scrolls, r.pinned, r.offset = true, len(item)-len(diff.changes), diff.offset
		}
	}
	if n := len(f.Fields); !f.single() {
		submit := "  Submit"
		if f.at == n {
			submit = Focus.Render("❯ Submit")
		}
		add(n, []string{submit}, 0)
	}
	hint := f.Hint
	if f.open {
		hint = cmp.Or(f.OpenHint, hint)
		if diff, ok := f.Fields[f.at].(*DiffField); ok {
			hint = cmp.Or(diff.Hint, hint)
		}
	}
	if hint != "" {
		r.below = append([]string{""}, strings.Split(Muted.Render(Wrap(hint, width)), "\n")...)
	}
	return r
}

// indented wraps text that does not fit in what is left of width after prefix,
// and continues its lines under the first one. Text that fits keeps its spaces,
// which may line up columns.
func indented(prefix, text string, width int) string {
	indent := ansi.StringWidth(prefix)
	if room := max(width-indent, 1); lipgloss.Width(text) > room {
		text = Wrap(text, room)
	}
	return prefix + strings.ReplaceAll(text, "\n", "\n"+strings.Repeat(" ", indent))
}

// drawField draws what is above a field's answer.
func drawField(title, help string, width int, state fieldState) []string {
	marker, role := "  ", Subhead
	if state != fieldAway {
		role = Header
	}
	if state == fieldHere {
		marker = "❯ "
	}
	rows := strings.Split(role.Render(indented(marker, title, width)), "\n")
	if help != "" {
		rows = append(rows, strings.Split(Muted.Render(indented("  ", help, width)), "\n")...)
	}
	return rows
}

// answerIndent is the column of a field's answer, after its marker.
const answerIndent = 4

// drawUnder draws what is said about a field's answer.
func drawUnder(role Role, text string, width int) []string {
	if text == "" {
		return nil
	}
	return strings.Split(role.Render(indented(strings.Repeat(" ", answerIndent), text, width)), "\n")
}

// InputField is a Form's field for typing text.
type InputField struct {
	Title string
	Input
	// Note, when set, remarks on the text as it stands, such as that it names
	// no file yet; empty for nothing.
	Note func(text string) string

	note   string // what Note gave for Value
	before string // Value when the field was opened
}

func (f *InputField) start() {
	f.Input.start()
	f.remark()
}

func (f *InputField) remark() {
	if f.Note != nil {
		f.note = f.Note(f.Value)
	}
}

func (f *InputField) opens(msg tea.Msg) bool {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return msg.Key().Text != ""
	case tea.PasteMsg:
		return true
	}
	return false
}

func (f *InputField) open() {
	f.before = f.Value
	f.text.CursorEnd()
	f.text.Focus()
}

func (f *InputField) shut() { f.text.Blur() }

func (f *InputField) undo() {
	f.Value = f.before
	f.start()
}

func (f *InputField) resize(width int) {
	// The cursor takes one column past the text's end, before the unit.
	f.room = max(width-answerIndent-1-ansi.StringWidth(f.Unit), 1)
	f.fit()
}

func (f *InputField) update(msg tea.Msg, _ scroll) (fieldEnd, tea.Cmd) {
	before := f.Value
	cmd := f.edit(msg)
	if f.Value != before {
		f.remark()
	}
	return fieldStays, cmd
}

func (f *InputField) lock() bool { return f.check() }

func (f *InputField) accepted() bool { return f.check() }

func (f *InputField) draw(width int, state fieldState) ([]string, int) {
	rows := drawField(f.Title, "", width, state)
	cursor := 0
	switch {
	case state == fieldOpen, state == fieldOnly:
		cursor = len(rows)
		rows = append(rows, Focus.Render("  ❯ ")+f.view())
	case f.Value == "":
		rows = append(rows, "    "+f.view())
	default:
		rows = append(rows, f.answer())
	}
	rows = append(rows, drawUnder(Error, f.text.invalid, width)...)
	return append(rows, drawUnder(Muted, f.note, width)...), cursor
}

// answer draws the text of a shut field.
func (f *InputField) answer() string {
	text := "  > " + ansi.Truncate(f.Value, f.room, "…")
	if f.text.invalid == "" {
		text = Chosen.Render(text)
	}
	if f.Unit != "" {
		text += Muted.Render(" " + f.Unit)
	}
	return text
}

// SelectField is a Form's field that takes one of its numbered options. A
// number also locks its option; with more than nine options, a digit waits for
// a second digit or enter.
type SelectField struct {
	Title   string
	Help    string
	Options []string
	// Choice is the option locked, or -1 for none yet, which the form does
	// not submit.
	Choice int

	at      int    // the option at the cursor while the field is open
	digit   string // the first digit of a two-digit option number
	invalid string
}

func (f *SelectField) start()     { f.invalid = "" }
func (f *SelectField) open()      { f.at, f.digit = max(f.Choice, 0), "" }
func (f *SelectField) shut()      {}
func (f *SelectField) undo()      {}
func (f *SelectField) resize(int) {}

func (f *SelectField) opens(msg tea.Msg) bool {
	m, ok := msg.(tea.KeyPressMsg)
	return ok && len(m.String()) == 1 && m.String() >= "1" && m.String() <= "9"
}

func (f *SelectField) update(msg tea.Msg, _ scroll) (fieldEnd, tea.Cmd) {
	m, ok := msg.(tea.KeyPressMsg)
	if ok && moveKey(m.String(), &f.at, &f.digit, len(f.Options)) {
		return fieldLocks, nil
	}
	return fieldStays, nil
}

func (f *SelectField) lock() bool {
	f.Choice, f.invalid = f.at, ""
	return true
}

func (f *SelectField) accepted() bool {
	if f.Choice < 0 {
		f.invalid = "Choose one."
	}
	return f.Choice >= 0
}

func (f *SelectField) draw(width int, state fieldState) ([]string, int) {
	rows := drawField(f.Title, f.Help, width, state)
	cursor := 0
	open := state == fieldOpen || state == fieldOnly
	digits := len(strconv.Itoa(len(f.Options)))
	for i, option := range f.Options {
		marker := "    "
		switch {
		case open && i == f.at:
			cursor, marker = len(rows), "  ❯ "
		case !open && i == f.Choice:
			marker = "  > "
		}
		drawn := indented(fmt.Sprintf("%s%*d. ", marker, digits, i+1), option, width)
		if i == f.Choice {
			drawn = Chosen.Render(drawn)
		}
		rows = append(rows, strings.Split(drawn, "\n")...)
	}
	return append(rows, drawUnder(Error, f.invalid, width)...), cursor
}
