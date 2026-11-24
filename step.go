package teakit

import (
	"context"
	"errors"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

var (
	// ErrBack reports that the user asked to return to the previous screen.
	ErrBack = errors.New("back")
	// ErrLeave reports that the user left the flow.
	ErrLeave = errors.New("leave")
)

// Step is one screen of a flow: a Select, a MultiSelect, a Form, or with none
// of them, the leave question alone. Run shows it as its own program, which
// leaving it by any key erases, or under AltScreen as the flow's current
// screen.
//
// In a form and in a select's input, esc goes back and backspace edits the
// text. On a choice screen backspace goes back and esc leaves, asking the
// leave question first when Unsaved is set; esc in a form or an input with no
// previous screen leaves the same way. Shift+tab goes back, in a form only
// from its first field. Ctrl+C leaves at once.
type Step struct {
	Select      *Select
	MultiSelect *MultiSelect
	Form        *Form
	// Back is set when a previous screen exists.
	Back bool
	// Unsaved is set when leaving loses something, so esc asks first.
	Unsaved bool
	// Leave words the leave question, which is shown like a select.
	Leave LeaveConfig
	// Head is shown above the screen; nil for nothing.
	Head func(width int) string
}

// choice is a question with options: a Select or a MultiSelect.
type choice interface {
	// start readies the question for a new showing.
	start()
	// press handles a key and reports whether it answered the question.
	press(key string) bool
	list() optionList
}

// choice returns the step's Select or MultiSelect, or nil.
func (s *Step) choice() choice {
	switch {
	case s.Select != nil:
		return s.Select
	case s.MultiSelect != nil:
		return s.MultiSelect
	}
	return nil
}

// Run shows s until the user leaves it. It returns nil when the question was
// answered, the form submitted, or the user stayed at a lone leave question;
// ErrBack when the user went back; and ErrLeave when they left the flow or
// the program ended without an answer, as on a termination signal.
func (s *Step) Run(ctx context.Context, opts ...tea.ProgramOption) error {
	m := &stepModel{s: s}
	if c := s.choice(); c != nil {
		c.start()
	} else if s.Form != nil {
		s.Form.start()
		s.Form.resize(screenWidth(0))
	} else {
		q := NewLeave(s.Leave)
		m.quit = &q
	}
	if err := show(ctx, m, opts); err != nil {
		if interrupted(ctx) {
			return ErrLeave
		}
		return err
	}
	if !m.done {
		return ErrLeave
	}
	return m.result
}

type stepModel struct {
	s             *Step
	quit          *LeaveModel // the leave question while it is up
	width, height int
	result        error
	done          bool
}

func (m *stepModel) Init() tea.Cmd { return nil }

func (m *stepModel) Done() bool { return m.done }

func (m *stepModel) end(result error) tea.Cmd {
	m.result, m.done = result, true
	return nil
}

func (m *stepModel) Update(msg tea.Msg) tea.Cmd {
	if m.done {
		return nil
	}
	s := m.s
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if s.Form != nil {
			s.Form.resize(screenWidth(m.width))
		}
		return nil
	case tea.KeyPressMsg:
		return m.press(msg)
	case tea.PasteMsg:
		if m.quit != nil {
			return nil
		}
	}
	// What is left is for the text being typed, such as what ctrl+v reads
	// from the clipboard.
	switch {
	case s.Select != nil && s.Select.typing():
		return s.Select.Input.edit(msg)
	case s.Form != nil:
		_, cmd := s.Form.update(msg)
		return cmd
	}
	return nil
}

func (m *stepModel) press(msg tea.KeyPressMsg) tea.Cmd {
	s, c, key := m.s, m.s.choice(), msg.String()
	typing := s.Select != nil && s.Select.typing()
	editing := s.Form != nil || typing
	switch {
	case key == "ctrl+c":
		return m.end(ErrLeave)
	case m.quit != nil:
		if _, shown := leaveView(s.Leave, m.quit.Cursor(), screenWidth(m.width), m.height); !shown {
			return nil
		}
		next, _ := m.quit.Update(msg)
		*m.quit = next
		switch next.Decision() {
		case Stay:
			m.quit = nil
			if c == nil && s.Form == nil {
				return m.end(nil)
			}
		case Leave, Interrupt:
			return m.end(ErrLeave)
		}
	case key == "esc" && editing && s.Back,
		key == "backspace" && s.Back && !editing,
		key == "shift+tab" && s.Back && (s.Form == nil || s.Form.at == 0):
		return m.end(ErrBack)
	case key == "esc" && s.Unsaved:
		q := NewLeave(s.Leave)
		m.quit = &q
	case key == "esc":
		return m.end(ErrLeave)
	case typing:
		accepted, cmd := s.Select.typeKey(msg)
		if accepted {
			return m.end(nil)
		}
		return cmd
	case c != nil:
		if c.press(key) {
			return m.end(nil)
		}
	case s.Form != nil:
		submitted, cmd := s.Form.update(msg)
		if submitted {
			return m.end(nil)
		}
		return cmd
	}
	return nil
}

func (m *stepModel) View() string {
	width := screenWidth(m.width)
	s := m.s
	if m.quit != nil {
		view, _ := leaveView(s.Leave, m.quit.Cursor(), width, m.height)
		return view
	}
	var head []string
	if s.Head != nil {
		head = append(strings.Split(s.Head(width), "\n"), "")
	}
	if s.Form != nil {
		return s.Form.draw(width).view(head, m.height)
	}
	return s.choice().list().draw(width).view(head, m.height)
}

// leaveView draws the leave question like the screens around it, in at
// most height rows once the height is known. A short terminal loses the
// text above the options first, then the hint below them. When even the
// options do not fit, it asks to enlarge the terminal instead and reports
// that the question is not shown, so it must not be answered.
func leaveView(cfg LeaveConfig, cursor, width, height int) (string, bool) {
	r := leaveList(cfg, cursor).draw(width)
	lines := slices.Concat(r.above, r.items, r.below)
	if height <= 0 || len(lines) <= height {
		return strings.Join(lines, "\n"), true
	}
	if len(r.items) > height {
		return ansi.Truncate("Ctrl+C exits. Enlarge to answer.", width, "…"), false
	}
	start := min(len(lines)-height, len(r.above))
	return strings.Join(lines[start:start+height], "\n"), true
}

// screenWidth keeps the text column readable on a wide terminal; before the
// terminal reports its size, it assumes 80 columns.
func screenWidth(terminal int) int {
	if terminal <= 0 {
		return 80
	}
	return min(terminal, 100)
}

// leaveList draws the leave question like the screens around it.
func leaveList(cfg LeaveConfig, cursor int) optionList {
	cfg = NewLeave(cfg).cfg
	l := optionList{
		title:   cfg.Title,
		options: []string{cfg.StayLabel, cfg.LeaveLabel},
		cursor:  cursor,
		hint:    leaveHint(cfg),
	}
	if cfg.Body != "" {
		l.body = func(width int) string { return Wrap(cfg.Body, width) }
	}
	return l
}

// Wait shows view while work runs, and returns what work returns. Esc
// cancels work's context and returns context.Canceled; ctrl+c, or the
// program ending before work does, as on a termination signal, returns
// ErrLeave at once. When ctx ends first, Wait returns its error without
// waiting for work, which should stop on its context. A panic in work ends
// the program as any Bubble Tea command's would, and Wait returns its error.
func Wait[T any](ctx context.Context, view func(width int) string, work func(context.Context) (T, error), opts ...tea.ProgramOption) (T, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	m := &waitModel[T]{view: view}
	m.work = func() tea.Msg {
		v, err := work(ctx)
		return waitResult[T]{m, v, err}
	}
	m.stop = func() tea.Msg {
		<-ctx.Done()
		return waitResult[T]{from: m, err: ctx.Err()}
	}
	var zero T
	if err := show(ctx, m, opts); err != nil {
		switch {
		case interrupted(ctx):
			return zero, ErrLeave
		case ctx.Err() != nil && errors.Is(err, ctx.Err()):
			return zero, ctx.Err()
		}
		return zero, err
	}
	if !m.done {
		return zero, ErrLeave
	}
	if m.result.err != nil && interrupted(ctx) {
		return zero, ErrLeave
	}
	return m.result.v, m.result.err
}

// waitResult ends the wait that from shows; under AltScreen a later screen
// can receive one from an earlier wait.
type waitResult[T any] struct {
	from *waitModel[T]
	v    T
	err  error
}

type waitModel[T any] struct {
	view func(width int) string
	// work runs work, and stop ends the wait when its context does.
	work, stop tea.Cmd
	width      int
	result     waitResult[T]
	done       bool
}

func (m *waitModel[T]) Init() tea.Cmd { return tea.Batch(m.work, m.stop) }

func (m *waitModel[T]) Done() bool { return m.done }

func (m *waitModel[T]) end(r waitResult[T]) tea.Cmd {
	m.result, m.done = r, true
	return nil
}

func (m *waitModel[T]) Update(msg tea.Msg) tea.Cmd {
	if m.done {
		return nil
	}
	switch msg := msg.(type) {
	case waitResult[T]:
		if msg.from == m {
			return m.end(msg)
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m.end(waitResult[T]{err: ErrLeave})
		case "esc":
			return m.end(waitResult[T]{err: context.Canceled})
		}
	}
	return nil
}

func (m *waitModel[T]) View() string { return m.view(screenWidth(m.width)) }
