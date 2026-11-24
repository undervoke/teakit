// Package teakit provides shared Bubble Tea components with one set of key
// conventions for terminal tools.
package teakit

import (
	"context"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Decision is the answer to the leave question. Pending means unanswered.
type Decision int

const (
	Pending Decision = iota
	Stay
	Leave
	Interrupt
)

// LeaveConfig supplies wording for the default view and extra answer keys.
// Hint replaces the default key line. StayKeys and LeaveKeys hold exact
// tea.Key names (for example "y" or "q"); anything else is left alone for
// the host. Built-ins always win.
type LeaveConfig struct {
	Title      string
	Body       string
	StayLabel  string
	LeaveLabel string
	Hint       string
	StayKeys   []string
	LeaveKeys  []string
}

// LeaveModel is an embeddable leave question. It never quits the program;
// the owner reads Decision and decides what follows.
type LeaveModel struct {
	cfg      LeaveConfig
	cursor   int
	decision Decision
}

// NewLeave returns a question starting on the stay option.
func NewLeave(cfg LeaveConfig) LeaveModel {
	if cfg.StayLabel == "" {
		cfg.StayLabel = "Stay"
	}
	if cfg.LeaveLabel == "" {
		cfg.LeaveLabel = "Leave"
	}
	cfg.StayKeys = slices.Clone(cfg.StayKeys)
	cfg.LeaveKeys = slices.Clone(cfg.LeaveKeys)
	return LeaveModel{cfg: cfg}
}

// Init performs no I/O.
func (m LeaveModel) Init() tea.Cmd { return nil }

// Update answers Esc with Leave, Ctrl+C with Interrupt, Enter with the
// highlighted option, and "1"/"2" with the matching option. Up/k selects
// stay, Down/j selects leave. Extra configured keys answer the same way;
// anything else is ignored.
func (m LeaveModel) Update(msg tea.Msg) (LeaveModel, tea.Cmd) {
	if m.decision != Pending {
		return m, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch name := key.String(); name {
	case "ctrl+c":
		m.decision = Interrupt
	case "esc", "2":
		m.decision = Leave
	case "1":
		m.decision = Stay
	case "enter":
		if m.cursor == 0 {
			m.decision = Stay
		} else {
			m.decision = Leave
		}
	case "up", "k":
		m.cursor = 0
	case "down", "j":
		m.cursor = 1
	default:
		switch {
		case slices.Contains(m.cfg.StayKeys, name):
			m.decision = Stay
		case slices.Contains(m.cfg.LeaveKeys, name):
			m.decision = Leave
		}
	}
	return m, nil
}

// View renders title, body, and a numbered two-option list. The first
// option always stays.
func (m LeaveModel) View() string {
	var lines []string
	for _, l := range m.lines() {
		lines = append(lines, l.draw(l.text))
	}
	return strings.Join(lines, "\n")
}

// leaveLine is a line of the default view and what draws it.
type leaveLine struct {
	text string
	draw func(string) string
}

func (m LeaveModel) lines() []leaveLine {
	var out []leaveLine
	add := func(text string, draw func(string) string) {
		for _, line := range strings.Split(text, "\n") {
			out = append(out, leaveLine{line, draw})
		}
	}
	plain := func(text string) string { return text }
	if m.cfg.Title != "" {
		add(m.cfg.Title, func(text string) string { return Header.Render(Warning.Render(text)) })
	}
	if m.cfg.Body != "" {
		add(m.cfg.Body, plain)
	}
	if m.cfg.Title != "" || m.cfg.Body != "" {
		add("", plain)
	}
	for i, label := range [...]string{m.cfg.StayLabel, m.cfg.LeaveLabel} {
		if i == m.cursor {
			add(fmt.Sprintf("> %d. %s", i+1, label), Focus.Render)
		} else {
			add(fmt.Sprintf("  %d. %s", i+1, label), plain)
		}
	}
	add("", plain)
	add(leaveHint(m.cfg), Muted.Render)
	return out
}

func leaveHint(cfg LeaveConfig) string {
	if cfg.Hint != "" {
		return cfg.Hint
	}
	return "↑/↓ k/j move · 1/2 or enter choose · esc leave · ctrl+c exit"
}

// Cursor returns the highlighted option's zero-based index: 0 for stay, 1 for
// leave. A host that draws its own question can use it to show what Enter does.
func (m LeaveModel) Cursor() int { return m.cursor }

// Decision reports the answer, or Pending while unanswered.
func (m LeaveModel) Decision() Decision { return m.decision }

// leaveScreen shows the question for AskLeave like a Step that has nothing
// else to show.
type leaveScreen struct {
	inner         LeaveModel
	width, height int
}

func (s *leaveScreen) Init() tea.Cmd { return s.inner.Init() }

func (s *leaveScreen) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width, s.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		if _, shown := s.view(); !shown && msg.String() != "ctrl+c" {
			return nil
		}
	}
	next, cmd := s.inner.Update(msg)
	s.inner = next
	return cmd
}

func (s *leaveScreen) View() string {
	view, _ := s.view()
	return view
}

func (s *leaveScreen) view() (string, bool) {
	return leaveView(s.inner.cfg, s.inner.Cursor(), screenWidth(s.width), s.height)
}

func (s *leaveScreen) Done() bool { return s.inner.Decision() != Pending }

// AskLeave runs the question as its own inline program, or under AltScreen
// as the flow's current screen, and returns the decision. A program that
// ends without an answer, as on a termination signal, returns Interrupt.
func AskLeave(ctx context.Context, cfg LeaveConfig, opts ...tea.ProgramOption) (Decision, error) {
	s := &leaveScreen{inner: NewLeave(cfg)}
	if err := show(ctx, s, opts); err != nil {
		if interrupted(ctx) {
			return Interrupt, nil
		}
		return Pending, err
	}
	if !s.Done() {
		return Interrupt, nil
	}
	return s.inner.Decision(), nil
}
