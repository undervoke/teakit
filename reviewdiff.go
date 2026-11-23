package teakit

import (
	"context"

	tea "charm.land/bubbletea/v2"
)

// diffScreen shows the diff question for ReviewDiff. With receipt set, a
// confirm first prints the whole static diff above the frame, which
// persists in the scrollback, and the screen is done once it is printed.
type diffScreen struct {
	inner    DiffModel
	receipt  bool
	printing bool
	done     bool
}

type diffPrintedMsg struct{}

func (s *diffScreen) Init() tea.Cmd { return s.inner.Init() }

func (s *diffScreen) Update(msg tea.Msg) tea.Cmd {
	if _, ok := msg.(diffPrintedMsg); ok {
		s.done = true
		return nil
	}
	if s.printing {
		return nil
	}
	next, cmd := s.inner.Update(msg)
	s.inner = next
	switch s.inner.Decision() {
	case DiffPending:
		return cmd
	case DiffConfirm:
		if s.receipt {
			s.printing = true
			receipt := renderStatic(s.inner.cfg, s.inner.files, s.inner.effWidth(), true)
			return tea.Sequence(tea.Println(receipt), func() tea.Msg { return diffPrintedMsg{} })
		}
	}
	s.done = true
	return nil
}

// View is empty once answered, so only the frame clears.
func (s *diffScreen) View() string {
	if s.inner.Decision() != DiffPending {
		return ""
	}
	return s.inner.View()
}

func (s *diffScreen) Done() bool { return s.done }

// ReviewDiff runs the diff question as its own inline program, or under
// AltScreen as the flow's current screen, and returns the decision. Inline,
// on confirm the whole static diff stays in the scrollback; on any other
// answer the frame clears and the host prints the outcome, e.g. "Cancelled;
// nothing was changed." A program that ends without an answer, as on a
// termination signal, returns DiffInterrupt.
func ReviewDiff(ctx context.Context, cfg DiffConfig, files []DiffFile, opts ...tea.ProgramOption) (DiffDecision, error) {
	s := &diffScreen{inner: NewDiff(cfg).WithFiles(files), receipt: hostOf(ctx) == nil}
	if err := show(ctx, s, opts); err != nil {
		if interrupted(ctx) {
			return DiffInterrupt, nil
		}
		return DiffPending, err
	}
	if !s.Done() {
		return DiffInterrupt, nil
	}
	return s.inner.Decision(), nil
}
