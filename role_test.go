package teakit

import (
	"strings"
	"testing"
)

// TestRoleWithinTitle catches Wrap or Role.Render reduced to lipgloss's own
// joining and rendering, which drop a style at a fold and after a nested one.
func TestRoleWithinTitle(t *testing.T) {
	s := &Select{Title: Warning.Render("unsaved changes") + " here"}
	rows := s.list().draw(len("changes here")).above
	warning, _, _ := strings.Cut(Warning.Render("x"), "x")
	header, _, _ := strings.Cut(Header.Render("x"), "x")
	for i, want := range []string{warning + "unsaved", warning + "changes", header + " here"} {
		if row := rows[min(i, 1)]; !strings.Contains(row, want) {
			t.Errorf("row %q lacks %q", row, want)
		}
	}
}
