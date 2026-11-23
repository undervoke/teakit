package teakit

import (
	"context"

	tea "charm.land/bubbletea/v2"
)

// diffStandalone adapts the submodel to a tea.Model for one-shot runs.
// Answered views render empty so only this inline frame clears on quit.
// On confirm it first prints the whole static diff above the frame, which
// persists in the scrollback.
type diffStandalone struct {
	inner   DiffModel
	printed bool
}

func (s *diffStandalone) Init() tea.Cmd { return s.inner.Init() }

func (s *diffStandalone) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := s.inner.Update(msg)
	s.inner = next
	if s.inner.Decision() == DiffConfirm && !s.printed {
		s.printed = true
		width := s.inner.effWidth()
		return s, tea.Println(renderStatic(s.inner.cfg, s.inner.files, width, true))
	}
	if s.inner.Decision() != DiffPending {
		return s, tea.Quit
	}
	return s, cmd
}

func (s *diffStandalone) View() tea.View {
	if s.inner.Decision() != DiffPending {
		return tea.NewView("")
	}
	return tea.NewView(s.inner.View())
}

// ReviewDiff runs the diff question as its own inline program and returns
// the decision. On confirm the whole static diff stays in the scrollback;
// on any other answer the frame clears and the host prints the outcome, e.g.
// "Cancelled; nothing was changed." A program that ends without an answer,
// as on a termination signal, returns DiffInterrupt.
func ReviewDiff(ctx context.Context, cfg DiffConfig, files []DiffFile, opts ...tea.ProgramOption) (DiffDecision, error) {
	s := &diffStandalone{inner: NewDiff(cfg).WithFiles(files)}
	_, err := tea.NewProgram(s, append(opts, tea.WithContext(ctx))...).Run()
	if err != nil {
		if interrupted(ctx) {
			return DiffInterrupt, nil
		}
		return DiffPending, err
	}
	if s.inner.Decision() == DiffPending {
		return DiffInterrupt, nil
	}
	return s.inner.Decision(), nil
}
