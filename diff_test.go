package teakit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// diffTestFiles returns a small two-section diff for gesture tests.
func diffTestFiles() []DiffFile {
	return []DiffFile{{
		Path: "settings.json",
		Sections: []DiffSection{{
			Label: "/a",
			Ops: []DiffOp{
				{Kind: OpEqual, Text: "{"},
				{Kind: OpDelete, Text: `"old"`},
				{Kind: OpInsert, Text: `"new"`},
				{Kind: OpEqual, Text: "}"},
			},
		}},
	}}
}

// diffPress feeds one exact key name through Update, like a terminal would.
func diffPress(m DiffModel, name string) DiffModel {
	var code rune
	var text string
	if len([]rune(name)) == 1 {
		r := []rune(name)[0]
		code, text = r, name
	} else {
		text = name
		switch name {
		case "enter":
			code = rune(tea.KeyEnter)
		case "esc":
			code = rune(tea.KeyEscape)
		case "backspace":
			code = rune(tea.KeyBackspace)
		case "up":
			code = rune(tea.KeyUp)
		case "down":
			code = rune(tea.KeyDown)
		}
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: code, Text: text})
	return next
}

// diffArmed runs fn with the settle delay disabled and restores it after.
func diffArmed(t *testing.T, fn func()) {
	t.Helper()
	old := diffSettleDelay
	diffSettleDelay = 0
	defer func() { diffSettleDelay = old }()
	fn()
}

// TestDiffEnterGuards is the consent test: Enter must stay Pending unless
// the diff is Ready, armed, fitting, and acknowledged. Each row names a
// bypass that would apply someone else's decision.
func TestDiffEnterGuards(t *testing.T) {
	enter := tea.KeyPressMsg{Code: rune(tea.KeyEnter)}
	files := diffTestFiles()
	ack := DiffConfig{Acknowledge: "Discard the data"}
	t.Run("Loading", func(t *testing.T) {
		m := NewDiff(DiffConfig{})
		next, _ := m.Update(enter)
		if next.Decision() != DiffPending {
			t.Fatalf("Enter on Loading decided %v", next.Decision())
		}
	})
	t.Run("Incomplete", func(t *testing.T) {
		m := NewDiff(DiffConfig{}).WithIncomplete("host failed")
		next, _ := m.Update(enter)
		if next.Decision() != DiffPending {
			t.Fatalf("Enter on Incomplete decided %v", next.Decision())
		}
	})
	t.Run("Contradiction", func(t *testing.T) {
		m := NewDiff(DiffConfig{}).WithFiles([]DiffFile{{
			Path: "f",
			Sections: []DiffSection{{
				Label:       "/a",
				AfterAbsent: true,
				Ops:         []DiffOp{{Kind: OpInsert, Text: "x"}},
			}},
		}})
		if m.State() != DiffIncomplete {
			t.Fatalf("contradictory presence did not enter Incomplete, state=%v", m.State())
		}
		next, _ := m.Update(enter)
		if next.Decision() != DiffPending {
			t.Fatalf("Enter on contradictory diff decided %v", next.Decision())
		}
	})
	t.Run("Settle", func(t *testing.T) {
		m := NewDiff(DiffConfig{}).WithFiles(files)
		next, _ := m.Update(enter)
		if next.Decision() != DiffPending {
			t.Fatalf("Enter before the settle delay decided %v", next.Decision())
		}
	})
	t.Run("Unacknowledged", func(t *testing.T) {
		diffArmed(t, func() {
			m := NewDiff(ack).WithFiles(files)
			next, _ := m.Update(enter)
			if next.Decision() != DiffPending {
				t.Fatalf("unacknowledged Enter decided %v", next.Decision())
			}
		})
	})
	t.Run("TooSmall", func(t *testing.T) {
		diffArmed(t, func() {
			m := NewDiff(ack).WithFiles(files).SetSize(20, 5)
			if next, _ := m.Update(enter); next.Decision() != DiffPending {
				t.Fatalf("Enter on a too-small diff decided %v", next.Decision())
			}
		})
	})
	t.Run("Repeat", func(t *testing.T) {
		diffArmed(t, func() {
			m := NewDiff(ack).WithFiles(files)
			m = diffPress(m, "y")
			rep, _ := m.Update(tea.KeyPressMsg{Code: rune(tea.KeyEnter), IsRepeat: true})
			if rep.Decision() != DiffPending {
				t.Fatalf("repeat Enter decided %v", rep.Decision())
			}
		})
	})
	t.Run("PastedY", func(t *testing.T) {
		diffArmed(t, func() {
			m := NewDiff(ack).WithFiles(files)
			next, _ := m.Update(tea.PasteMsg{Content: "y\n"})
			if next.Decision() != DiffPending {
				t.Fatalf("PasteMsg decided %v", next.Decision())
			}
			if nxt, _ := next.Update(enter); nxt.Decision() != DiffPending {
				t.Fatalf("pasted y armed the acknowledgement")
			}
		})
	})
	t.Run("Confirm", func(t *testing.T) {
		diffArmed(t, func() {
			m := NewDiff(ack).WithFiles(files)
			m = diffPress(m, "y")
			if next, _ := m.Update(enter); next.Decision() != DiffConfirm {
				t.Fatalf("armed acknowledged Enter decided %v", next.Decision())
			}
		})
	})
	t.Run("ResetOnNewContent", func(t *testing.T) {
		diffArmed(t, func() {
			m := NewDiff(ack).WithFiles(files)
			m = diffPress(m, "y")
			m = m.WithFiles(files)
			if next, _ := m.Update(enter); next.Decision() != DiffPending {
				t.Fatalf("replacement content kept the old acknowledgement")
			}
		})
	})
}

// TestDiffSeparatorAlignment pins the side-by-side table geometry: every
// content row's separator must sit at the same cell column, including rows
// with CJK text and wrapped continuations. A byte- or rune-counting width
// regression shifts those rows.
func TestDiffSeparatorAlignment(t *testing.T) {
	files := []DiffFile{{
		Path: "config.toml",
		Sections: []DiffSection{{
			Label: "/s",
			Ops: []DiffOp{
				{Kind: OpDelete, Text: `key = "high"    # 한글 주석 테스트 길게 이어지는 줄`},
				{Kind: OpInsert, Text: `key = "medium"`},
				{Kind: OpEqual, Text: "plain ascii line here"},
			},
		}},
	}}
	width := 80
	var leftWidths []int
	for _, line := range strings.Split(ansi.Strip(RenderDiff(files, width)), "\n") {
		parts := strings.SplitN(line, " | ", 2)
		if len(parts) != 2 {
			continue
		}
		leftWidths = append(leftWidths, ansi.StringWidth(parts[0]))
	}
	if len(leftWidths) < 3 {
		t.Fatalf("expected table rows, got %d", len(leftWidths))
	}
	for _, w := range leftWidths[1:] {
		if w != leftWidths[0] {
			t.Fatalf("separator misaligned: widths %v", leftWidths)
		}
	}
}

// TestDiffEscapeStripsControlBytes feeds hostile text through the public
// render and requires that no raw control byte survives outside SGR
// sequences. A passthrough regression hands the terminal a live ESC.
func TestDiffEscapeStripsControlBytes(t *testing.T) {
	files := []DiffFile{{
		Path: "ho\x1bst",
		Sections: []DiffSection{{
			Label: "a\u202eb",
			Ops: []DiffOp{
				{Kind: OpDelete, Text: "x\x00y\t\x7fz"},
				{Kind: OpInsert, Text: "ok"},
			},
		}},
	}}
	out := RenderDiff(files, 80)
	stripped := ansi.Strip(out)
	for i := 0; i < len(stripped); i++ {
		if stripped[i] < 0x20 && stripped[i] != '\n' || stripped[i] == 0x7f {
			t.Fatalf("raw control byte 0x%02x survives at offset %d", stripped[i], i)
		}
	}
	if strings.Contains(stripped, "\x1b") {
		t.Fatalf("raw ESC survives outside SGR sequences")
	}
	for _, want := range []string{`\x1b`, `\x00`, `\u202e`} {
		if !strings.Contains(stripped, want) {
			t.Fatalf("expected visible escape %q in %q", want, stripped)
		}
	}
}

// TestDiffAckFlow checks y arming, n disarming, and repeat-y through Update.
func TestDiffAckFlow(t *testing.T) {
	diffArmed(t, func() {
		enter := tea.KeyPressMsg{Code: rune(tea.KeyEnter)}
		m := NewDiff(DiffConfig{Acknowledge: "Discard"}).WithFiles(diffTestFiles())
		m = diffPress(m, "n")
		if next, _ := m.Update(enter); next.Decision() != DiffPending {
			t.Fatalf("Enter after n decided %v", next.Decision())
		}
		m = diffPress(m, "y")
		rep, _ := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y", IsRepeat: true})
		m = rep
		if next, _ := m.Update(enter); next.Decision() != DiffConfirm {
			t.Fatalf("repeat y disarmed the acknowledgement")
		}
	})
}

// TestDiffViewHeight requires exactly height lines whenever a height is set.
func TestDiffViewHeight(t *testing.T) {
	m := NewDiff(DiffConfig{Title: "T"}).WithFiles(diffTestFiles()).SetSize(80, 12)
	if got := len(strings.Split(m.View(), "\n")); got != 12 {
		t.Fatalf("View returned %d lines, want 12", got)
	}
	m = NewDiff(DiffConfig{}).WithFiles(diffTestFiles()).SetSize(10, 4)
	if got := len(strings.Split(m.View(), "\n")); got != 4 {
		t.Fatalf("too-small View returned %d lines, want 4", got)
	}
}

// TestDiffLeaveCancel checks the embedded leave question: one Esc opens it,
// the second Esc cancels, and Ctrl+C interrupts from inside it.
func TestDiffLeaveCancel(t *testing.T) {
	m := NewDiff(DiffConfig{Leave: &LeaveConfig{Title: "Leave?"}}).WithFiles(diffTestFiles())
	m = diffPress(m, "esc")
	if m.Decision() != DiffPending {
		t.Fatalf("first Esc decided %v", m.Decision())
	}
	if !strings.Contains(ansi.Strip(m.View()), "Leave?") {
		t.Fatalf("leave question is not shown after Esc")
	}
	m = diffPress(m, "esc")
	if m.Decision() != DiffCancel {
		t.Fatalf("Esc Esc decided %v, want Cancel", m.Decision())
	}
	m = NewDiff(DiffConfig{Leave: &LeaveConfig{Title: "Leave?"}}).WithFiles(diffTestFiles())
	m = diffPress(m, "esc")
	m2, _ := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if m2.Decision() != DiffInterrupt {
		t.Fatalf("Ctrl+C in the leave question decided %v", m2.Decision())
	}
}
