package teakit

import (
	"context"
	"errors"
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
// question alone. Run shows it as its own program, which leaving it by any
// key erases, or under AltScreen as the flow's current screen.
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
// ErrBack when the user went back; and ErrLeave when they left the flow or
// the program ended without an answer, as on a termination signal.
func (s *Step) Run(ctx context.Context, opts ...tea.ProgramOption) error {
	m := &stepModel{s: s}
	if s.List != nil {
		s.List.focusField()
	}
	if s.List == nil && s.Form == nil {
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
	s *Step
	// quit is the leave question while it is up, and held what arrived for
	// the screen under it meanwhile.
	quit          *LeaveModel
	held          []tea.Msg
	width, height int
	result        error
	done          bool
}

func (m *stepModel) Init() tea.Cmd {
	if m.s.Form == nil {
		return nil
	}
	return m.s.Form.Init()
}

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
	case tea.KeyPressMsg:
		key := msg.String()
		typing := s.List != nil && s.List.typing()
		editing := s.Text || typing
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
				if s.List == nil && s.Form == nil {
					return m.end(nil)
				}
				return m.replay()
			case Leave, Interrupt:
				return m.end(ErrLeave)
			}
			return nil
		case key == "esc" && editing && s.Back,
			key == "backspace" && s.Back && !editing,
			key == "shift+tab" && s.Back && (!s.Text || s.Form.AtFirst()):
			return m.end(ErrBack)
		case key == "esc" && s.Unsaved:
			q := NewLeave(s.Leave)
			m.quit = &q
			return nil
		case key == "esc":
			return m.end(ErrLeave)
		case typing:
			accepted, cmd := s.List.typeKey(msg)
			if accepted {
				return m.end(nil)
			}
			return cmd
		case s.List != nil:
			picked := s.List.press(key)
			s.List.focusField()
			if picked {
				return m.end(nil)
			}
			return nil
		}
	default:
		if m.quit != nil {
			// The leave question owns the input, so the screen under it
			// neither changes nor ends until the user stays.
			switch msg.(type) {
			case tea.PasteMsg, tea.KeyReleaseMsg:
			default:
				m.held = append(m.held, msg)
			}
			return nil
		}
	}
	if s.List != nil {
		// The field's own messages, such as the text ctrl+v reads from the
		// clipboard, go back to it.
		if _, size := msg.(tea.WindowSizeMsg); !size && s.List.typing() && m.quit == nil {
			return s.List.Field.edit(msg)
		}
		return nil
	}
	if s.Form == nil {
		return nil
	}
	cmd := s.Form.Update(msg)
	if s.Form.Done() {
		return m.end(nil)
	}
	return cmd
}

func (m *stepModel) View() string {
	width := screenWidth(m.width)
	s := m.s
	if m.quit != nil {
		view, _ := leaveView(s.Leave, m.quit.Cursor(), width, m.height)
		return view
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
	return view
}

// replay gives the screen what arrived for it under the leave question.
func (m *stepModel) replay() tea.Cmd {
	held := m.held
	m.held = nil
	var cmds []tea.Cmd
	for _, msg := range held {
		if m.done {
			break
		}
		cmds = append(cmds, m.Update(msg))
	}
	return tea.Batch(cmds...)
}

// leaveView draws the leave question like the screens around it, in at
// most height rows once the height is known. A short terminal loses the
// text above the options first, then the hint below them. When even the
// options do not fit, it asks to enlarge the terminal instead and reports
// that the question is not shown, so it must not be answered.
func leaveView(cfg LeaveConfig, cursor, width, height int) (string, bool) {
	l := leaveList(cfg, cursor)
	view := l.view(width)
	lines := strings.Split(view, "\n")
	if height <= 0 || len(lines) <= height {
		return view, true
	}
	// Above the options are the title, the body and a blank row; below
	// them a blank row and the hint.
	from := len(strings.Split(bold.Render(Wrap(l.Title, width)), "\n")) + 1
	if l.Body != nil {
		from += len(strings.Split(l.Body(width), "\n"))
	}
	to := len(lines) - 1 - len(strings.Split(dim.Render(Wrap(l.Hint, width)), "\n"))
	if to-from > height {
		return ansi.Truncate("Ctrl+C exits. Enlarge to answer.", width, "…"), false
	}
	start := min(len(lines)-height, from)
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
