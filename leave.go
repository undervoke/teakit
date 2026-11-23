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
	var b strings.Builder
	if m.cfg.Title != "" {
		b.WriteString(m.cfg.Title + "\n")
	}
	if m.cfg.Body != "" {
		b.WriteString(m.cfg.Body + "\n")
	}
	if m.cfg.Title != "" || m.cfg.Body != "" {
		b.WriteString("\n")
	}
	labels := [...]string{m.cfg.StayLabel, m.cfg.LeaveLabel}
	for i, label := range labels {
		marker := "  "
		if i == m.cursor {
			marker = "> "
		}
		fmt.Fprintf(&b, "%s%d. %s\n", marker, i+1, label)
	}
	b.WriteString("\n" + leaveHint(m.cfg))
	return b.String()
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

// leaveStandalone adapts the submodel to a tea.Model for one-shot runs.
// Answered views render empty so only this inline frame clears on quit.
type leaveStandalone struct {
	inner LeaveModel
}

func (s *leaveStandalone) Init() tea.Cmd { return s.inner.Init() }

func (s *leaveStandalone) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := s.inner.Update(msg)
	s.inner = next
	if s.inner.Decision() != Pending {
		return s, tea.Quit
	}
	return s, cmd
}

func (s *leaveStandalone) View() tea.View {
	if s.inner.Decision() != Pending {
		return tea.NewView("")
	}
	return tea.NewView(s.inner.View())
}

// AskLeave runs the question as its own inline program and returns the
// decision.
func AskLeave(ctx context.Context, cfg LeaveConfig, opts ...tea.ProgramOption) (Decision, error) {
	s := &leaveStandalone{inner: NewLeave(cfg)}
	_, err := tea.NewProgram(s, append(opts, tea.WithContext(ctx))...).Run()
	if err != nil {
		if interrupted(ctx) {
			return Interrupt, nil
		}
		return Pending, err
	}
	return s.inner.Decision(), nil
}
