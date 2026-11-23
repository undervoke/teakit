package teakit

import (
	"context"
	"errors"
	"io"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

var (
	// ErrBack reports that the user asked to return to the previous screen.
	ErrBack = errors.New("back")
	// ErrLeave reports that the user left the flow.
	ErrLeave = errors.New("leave")
)

// Form is a screen of fields that a Step hosts, such as a huh form behind a
// small adapter. The step handles the navigation keys before the form sees
// them.
type Form interface {
	Init() tea.Cmd
	Update(tea.Msg) tea.Cmd
	View() string
	// Done reports that the form has ended, normally by being submitted.
	Done() bool
	// AtFirst reports that the first field has focus; Step reads it only for
	// a text form.
	AtFirst() bool
}

// Step is one screen of a flow: a List, a Form, or with neither, the leave
// question alone. Run shows it as its own program, so leaving it by any key
// erases its frame.
//
// While text is being typed, in a text form or in a list's field, backspace
// edits it and esc goes back. On a choice screen backspace goes back and esc
// leaves, asking the leave question first when Unsaved is set; esc while
// typing with no previous screen leaves the same way. Shift+tab goes back,
// in a text form only from its first field. Ctrl+C leaves at once.
type Step struct {
	List *List
	Form Form
	// Text marks Form as a form for typing text.
	Text bool
	// Back is set when a previous screen exists.
	Back bool
	// Unsaved is set when leaving loses something, so esc asks first.
	Unsaved bool
	// Leave words the leave question, which is shown as a list.
	Leave LeaveConfig
	// Head is shown above the list or form; nil for nothing.
	Head func(width int) string
	// Hint is shown under the form.
	Hint string
}

// Run shows s until the user leaves it. It returns nil when an option was
// picked, the form ended, or the user stayed at a lone leave question;
// ErrBack when the user went back; and ErrLeave when they left the flow.
func (s *Step) Run(ctx context.Context, opts ...tea.ProgramOption) error {
	m := &stepModel{s: s}
	if s.List != nil {
		s.List.focusField()
	}
	if s.List == nil && s.Form == nil {
		q := NewLeave(s.Leave)
		m.quit = &q
	}
	if _, err := tea.NewProgram(m, append(opts, tea.WithContext(ctx))...).Run(); err != nil {
		return err
	}
	return m.result
}

// AltScreen runs flow on the terminal's alternate screen, so the shell text
// before it stays hidden until flow returns. The programs flow runs, such as
// Step.Run, draw inline on that screen; give them out with tea.WithOutput.
// Whatever flow prints there is gone when it returns.
func AltScreen(out io.Writer, flow func() error) error {
	if _, err := io.WriteString(out, ansi.SetModeAltScreenSaveCursor+ansi.EraseEntireScreen+ansi.CursorHomePosition); err != nil {
		return err
	}
	defer io.WriteString(out, ansi.ResetModeAltScreenSaveCursor)
	return flow()
}

type stepModel struct {
	s *Step
	// quit is the leave question while it is up.
	quit   *LeaveModel
	width  int
	result error
	done   bool
}

func (m *stepModel) Init() tea.Cmd {
	if m.s.Form == nil {
		return nil
	}
	return m.s.Form.Init()
}

func (m *stepModel) end(result error) (tea.Model, tea.Cmd) {
	m.result, m.done = result, true
	return m, tea.Quit
}

func (m *stepModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.done {
		return m, nil
	}
	s := m.s
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyPressMsg:
		key := msg.String()
		typing := s.List != nil && s.List.typing()
		editing := s.Text || typing
		switch {
		case key == "ctrl+c":
			return m.end(ErrLeave)
		case m.quit != nil:
			next, _ := m.quit.Update(msg)
			*m.quit = next
			switch next.Decision() {
			case Stay:
				m.quit = nil
				if s.List == nil && s.Form == nil {
					return m.end(nil)
				}
			case Leave, Interrupt:
				return m.end(ErrLeave)
			}
			return m, nil
		case key == "esc" && editing && s.Back,
			key == "backspace" && s.Back && !editing,
			key == "shift+tab" && s.Back && (!s.Text || s.Form.AtFirst()):
			return m.end(ErrBack)
		case key == "esc" && s.Unsaved:
			q := NewLeave(s.Leave)
			m.quit = &q
			return m, nil
		case key == "esc":
			return m.end(ErrLeave)
		case typing:
			accepted, cmd := s.List.typeKey(msg)
			if accepted {
				return m.end(nil)
			}
			return m, cmd
		case s.List != nil:
			picked := s.List.press(key)
			s.List.focusField()
			if picked {
				return m.end(nil)
			}
			return m, nil
		}
	case tea.PasteMsg:
		if s.List != nil && s.List.typing() {
			return m, s.List.Field.edit(msg)
		}
	}
	if s.Form == nil {
		return m, nil
	}
	cmd := s.Form.Update(msg)
	if s.Form.Done() {
		return m.end(nil)
	}
	return m, cmd
}

func (m *stepModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}
	width := screenWidth(m.width)
	s := m.s
	if m.quit != nil {
		return tea.NewView(leaveList(s.Leave, m.quit.Cursor()).view(width))
	}
	var view string
	if s.List != nil {
		view = s.List.view(width)
	} else {
		view = s.Form.View()
		if s.Hint != "" {
			view += "\n\n" + dim.Render(Wrap(s.Hint, width))
		}
	}
	if s.Head != nil {
		view = s.Head(width) + "\n\n" + view
	}
	return tea.NewView(view)
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
func leaveList(cfg LeaveConfig, cursor int) *List {
	cfg = NewLeave(cfg).cfg
	l := &List{
		Title:   cfg.Title,
		Options: []string{cfg.StayLabel, cfg.LeaveLabel},
		Cursor:  cursor,
		Hint:    leaveHint(cfg),
	}
	if cfg.Body != "" {
		l.Body = func(width int) string { return Wrap(cfg.Body, width) }
	}
	return l
}

// Wait shows view while work runs, and returns what work returns. Esc
// cancels work's context and returns context.Canceled; ctrl+c returns
// ErrLeave at once. When ctx ends first, Wait returns its error without
// waiting for work, which should stop on its context.
func Wait[T any](ctx context.Context, view func(width int) string, work func(context.Context) (T, error), opts ...tea.ProgramOption) (T, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan waitResult[T], 1)
	go func() {
		v, err := work(ctx)
		results <- waitResult[T]{v, err}
	}()
	m := &waitModel[T]{view: view, wait: func() tea.Msg {
		select {
		case r := <-results:
			return r
		case <-ctx.Done():
			return waitResult[T]{err: ctx.Err()}
		}
	}}
	var zero T
	if _, err := tea.NewProgram(m, opts...).Run(); err != nil {
		return zero, err
	}
	return m.result.v, m.result.err
}

type waitResult[T any] struct {
	v   T
	err error
}

type waitModel[T any] struct {
	view   func(width int) string
	wait   tea.Cmd
	width  int
	result waitResult[T]
	done   bool
}

func (m *waitModel[T]) Init() tea.Cmd { return m.wait }

func (m *waitModel[T]) end(r waitResult[T]) (tea.Model, tea.Cmd) {
	m.result, m.done = r, true
	return m, tea.Quit
}

func (m *waitModel[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.done {
		return m, nil
	}
	switch msg := msg.(type) {
	case waitResult[T]:
		return m.end(msg)
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
	return m, nil
}

func (m *waitModel[T]) View() tea.View {
	if m.done {
		return tea.NewView("")
	}
	return tea.NewView(m.view(screenWidth(m.width)))
}
