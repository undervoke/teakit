package teakit

import (
	"fmt"
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

// TestDiffAnswerGuards is the consent test: Enter and the alternate key
// must stay Pending unless the diff is Ready, armed, fitting, and
// acknowledged. Each row names a bypass that would apply someone else's
// decision.
func TestDiffAnswerGuards(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  tea.KeyPressMsg
		want DiffDecision
	}{
		{"Enter", tea.KeyPressMsg{Code: rune(tea.KeyEnter)}, DiffConfirm},
		{"Alternate", tea.KeyPressMsg{Code: 'c', Text: "c"}, DiffAlternate},
	} {
		t.Run(tc.name, func(t *testing.T) { diffAnswerGuards(t, tc.key, tc.want) })
	}
}

func diffAnswerGuards(t *testing.T, enter tea.KeyPressMsg, want DiffDecision) {
	files := diffTestFiles()
	plain := DiffConfig{AlternateKey: "c"}
	ack := DiffConfig{AlternateKey: "c", Acknowledge: "Discard the data"}
	t.Run("Loading", func(t *testing.T) {
		m := NewDiff(plain)
		next, _ := m.Update(enter)
		if next.Decision() != DiffPending {
			t.Fatalf("Enter on Loading decided %v", next.Decision())
		}
	})
	t.Run("Incomplete", func(t *testing.T) {
		m := NewDiff(plain).WithIncomplete("host failed")
		next, _ := m.Update(enter)
		if next.Decision() != DiffPending {
			t.Fatalf("Enter on Incomplete decided %v", next.Decision())
		}
	})
	t.Run("Contradiction", func(t *testing.T) {
		m := NewDiff(plain).WithFiles([]DiffFile{{
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
		m := NewDiff(plain).WithFiles(files)
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
			repeat := enter
			repeat.IsRepeat = true
			rep, _ := m.Update(repeat)
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
			if next, _ := m.Update(enter); next.Decision() != want {
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

func diffLongTestFiles() []DiffFile {
	files := diffTestFiles()
	var ops []DiffOp
	for n := 1; n <= 50; n++ {
		ops = append(ops, DiffOp{Kind: OpInsert, Text: fmt.Sprintf("change %02d", n)})
	}
	files[0].Sections[0].Ops = ops
	return files
}

func TestDiffBottomShowsLastPage(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{{Code: tea.KeyEnd}, {Code: 'G', Text: "G"}, {Code: ']', Text: "]"}} {
		t.Run(key.String(), func(t *testing.T) {
			m := NewDiff(DiffConfig{}).WithFiles(diffLongTestFiles()).SetSize(80, 12)
			if key.Code == ']' {
				m, _ = m.Update(key) // jump to the only section before jumping past it
			}
			m, _ = m.Update(key)
			view := ansi.Strip(m.View())
			if !strings.Contains(view, "change 43") || !strings.Contains(view, "change 50") {
				t.Fatalf("last eight rows aren't visible:\n%s", view)
			}
		})
	}
}

func TestDiffNoRowsCannotAnswer(t *testing.T) {
	diffArmed(t, func() {
		cfg := DiffConfig{AlternateKey: "c", Title: strings.Repeat("Title\n", 12)}
		m := NewDiff(cfg).WithFiles(diffTestFiles()).SetSize(80, 12)
		for _, key := range []tea.KeyPressMsg{{Code: tea.KeyEnter}, {Code: 'c', Text: "c"}} {
			next, _ := m.Update(key)
			if next.Decision() != DiffPending {
				t.Errorf("%s answered a diff with no rows: %v", key, next.Decision())
			}
		}
	})
}

func TestDiffPageDown(t *testing.T) {
	m := NewDiff(DiffConfig{}).WithFiles(diffLongTestFiles()).SetSize(80, 12)
	key := tea.KeyPressMsg{Code: tea.KeyPgDown}
	if !strings.Contains(ansi.Strip(m.View()), key.String()) {
		t.Errorf("hint omits %s", key)
	}
	m, _ = m.Update(key)
	if view := ansi.Strip(m.View()); !strings.Contains(view, "change 12") {
		t.Errorf("Page Down didn't scroll a page:\n%s", view)
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

func TestDiffLeaveQuestionVisible(t *testing.T) {
	cfg := DiffConfig{Leave: &LeaveConfig{Title: "Leave this review?"}}
	m := NewDiff(cfg).SetSize(80, 12)
	longTitle := cfg
	longTitle.Title = strings.Repeat("Title\n", 12)
	for _, tc := range []struct {
		name  string
		model DiffModel
	}{
		{"loading", m},
		{"incomplete", m.WithIncomplete("host failed")},
		{"narrow", m.WithFiles(diffTestFiles()).SetSize(28, 12)},
		{"crowded", NewDiff(longTitle).WithFiles(diffTestFiles()).SetSize(80, 12)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			esc := tea.KeyPressMsg{Code: tea.KeyEscape}
			m, _ := tc.model.Update(esc)
			if view := ansi.Strip(m.View()); !strings.Contains(view, cfg.Leave.Title) ||
				!strings.Contains(view, "1. Stay") || !strings.Contains(view, "2. Leave") {
				t.Errorf("leave question not drawn:\n%s", view)
			}
			m, _ = m.Update(esc)
			if m.Decision() != DiffCancel {
				t.Errorf("Esc did not answer the visible question: %v", m.Decision())
			}
		})
	}
}

func TestDiffLeaveQuestionDoesNotFit(t *testing.T) {
	esc := tea.KeyPressMsg{Code: tea.KeyEscape}
	m := NewDiff(DiffConfig{Leave: &LeaveConfig{Title: "Leave?"}}).WithFiles(diffTestFiles()).SetSize(80, 1)
	m, _ = m.Update(esc)
	if next, _ := m.Update(esc); next.Decision() != DiffPending {
		t.Errorf("Esc answered a question that couldn't fit: %v", next.Decision())
	}
	m = m.SetSize(80, 12)
	if view := ansi.Strip(m.View()); !strings.Contains(view, "Leave?") {
		t.Fatalf("leave question missing after resize:\n%s", view)
	}
	if next, _ := m.Update(esc); next.Decision() != DiffCancel {
		t.Errorf("Esc didn't answer the question after resize: %v", next.Decision())
	}
}
