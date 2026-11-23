package teakit

import (
	"fmt"
	"strings"
	"testing"
	"time"

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

// diffArm runs the same Init/Update commands the host forwards, without a wait.
func diffArm(t *testing.T, m DiffModel) DiffModel {
	t.Helper()
	old := diffSettleDelay
	diffSettleDelay = 0
	defer func() { diffSettleDelay = old }()
	m, tick := m.Update(m.Init()())
	if tick == nil {
		t.Fatal("fixture has no visible diff to settle")
	}
	m, _ = m.Update(tick())
	return m
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
	t.Run("Unacknowledged", func(t *testing.T) {
		m := diffArm(t, NewDiff(ack).WithFiles(files).SetSize(80, 12))
		next, _ := m.Update(enter)
		if next.Decision() != DiffPending {
			t.Fatalf("unacknowledged Enter decided %v", next.Decision())
		}
	})
	t.Run("TooSmall", func(t *testing.T) {
		m := diffArm(t, NewDiff(plain).WithFiles(files).SetSize(80, 12)).SetSize(20, 5)
		if next, _ := m.Update(enter); next.Decision() != DiffPending {
			t.Fatalf("Enter on a too-small diff decided %v", next.Decision())
		}
	})
	t.Run("Repeat", func(t *testing.T) {
		m := diffArm(t, NewDiff(ack).WithFiles(files).SetSize(80, 12))
		m = diffPress(m, "y")
		repeat := enter
		repeat.IsRepeat = true
		rep, _ := m.Update(repeat)
		if rep.Decision() != DiffPending {
			t.Fatalf("repeat Enter decided %v", rep.Decision())
		}
	})
	t.Run("PastedY", func(t *testing.T) {
		m := diffArm(t, NewDiff(ack).WithFiles(files).SetSize(80, 12))
		next, _ := m.Update(tea.PasteMsg{Content: "y\n"})
		if next.Decision() != DiffPending {
			t.Fatalf("PasteMsg decided %v", next.Decision())
		}
		if nxt, _ := next.Update(enter); nxt.Decision() != DiffPending {
			t.Fatalf("pasted y armed the acknowledgement")
		}
	})
	t.Run("Confirm", func(t *testing.T) {
		m := diffArm(t, NewDiff(ack).WithFiles(files).SetSize(80, 12))
		m = diffPress(m, "y")
		if next, _ := m.Update(enter); next.Decision() != want {
			t.Fatalf("armed acknowledged Enter decided %v", next.Decision())
		}
	})
	t.Run("ResetOnNewContent", func(t *testing.T) {
		m := diffArm(t, NewDiff(ack).WithFiles(files).SetSize(80, 12))
		m = diffPress(m, "y")
		m = diffArm(t, m.WithFiles(files))
		if next, _ := m.Update(enter); next.Decision() != DiffPending {
			t.Fatalf("replacement content kept the old acknowledgement")
		}
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

// TestDiffWrappedEscapeIsHighlighted catches wrappers that split a marked escape or drop its reverse style.
func TestDiffWrappedEscapeIsHighlighted(t *testing.T) {
	files := diffTestFiles()
	files[0].Sections[0].Ops = []DiffOp{{Kind: OpInsert, Text: strings.Repeat("a", 32) + "\x01" + `\x01`}}
	out := RenderDiff(files, 80)
	if strings.Count(out, "\x1b[7m"+`\x01`) != 1 || strings.Count(ansi.Strip(out), `\x01`) != 2 {
		t.Fatalf("the wrapped control byte must be distinguishable from the literal escape:\n%q", out)
	}
}

// TestDiffLeaveControlSequenceDoesNotEatText catches a CSI stripper that eats title text while scanning for 'm'.
func TestDiffLeaveControlSequenceDoesNotEatText(t *testing.T) {
	cfg := DiffConfig{Leave: &LeaveConfig{Title: "\x1b[2JPreserve this message"}}
	m := diffPress(NewDiff(cfg).WithFiles(diffTestFiles()).SetSize(80, 12), "esc")
	if view := ansi.Strip(m.View()); !strings.Contains(view, "Preserve this message") {
		t.Fatalf("leave title lost text after a CSI sequence:\n%s", view)
	}
}

// TestDiffContinuationWidth catches adding indents after a row has already filled its width.
func TestDiffContinuationWidth(t *testing.T) {
	const width = 80
	for _, field := range []string{"path", "label", "note"} {
		t.Run(field, func(t *testing.T) {
			files := diffTestFiles()
			text := strings.Repeat("x", width*3) + "end"
			switch field {
			case "path":
				files[0].Path = text
			case "label":
				files[0].Sections[0].Label = text
			case "note":
				files[0].Sections[0].Notes = []string{text}
			}
			view := ansi.Strip(RenderDiff(files, width))
			for _, row := range strings.Split(view, "\n") {
				if n := ansi.StringWidth(row); n > width {
					t.Errorf("continuation exceeds width: %d > %d", n, width)
				}
			}
			if !strings.Contains(view, "end") {
				t.Fatal("continuation lost its tail")
			}
		})
	}
}

// TestDiffTabAfterGrapheme catches counting emoji runes separately and missing a tab stop.
func TestDiffTabAfterGrapheme(t *testing.T) {
	files := diffTestFiles()
	files[0].Sections[0].Ops = []DiffOp{{Kind: OpInsert, Text: "👩‍💻\tend"}}
	if got := ansi.Strip(RenderDiff(files, 80)); !strings.Contains(got, "👩‍💻  end") {
		t.Fatalf("tab after a two-cell grapheme misses the four-cell stop:\n%s", got)
	}
}

// TestDiffWrapRetainedTailBeforeWideGrapheme catches appending a wide glyph to a tail that no longer fits it.
func TestDiffWrapRetainedTailBeforeWideGrapheme(t *testing.T) {
	const width = 30
	body := " " + strings.Repeat("x", width-1) + "界"
	view := ansi.Strip(renderStatic(DiffConfig{Body: body}, nil, width, false))
	for _, row := range strings.Split(view, "\n") {
		if n := ansi.StringWidth(row); n > width {
			t.Errorf("wrapped row exceeds width: %d > %d", n, width)
		}
	}
	if strings.ReplaceAll(view, "\n", "") != body {
		t.Fatalf("wrapping lost content: %q", view)
	}
}

// TestDiffAckFlow catches repeat y arming consent or n failing to disarm it.
func TestDiffAckFlow(t *testing.T) {
	enter := tea.KeyPressMsg{Code: tea.KeyEnter}
	m := diffArm(t, NewDiff(DiffConfig{Acknowledge: "Discard"}).WithFiles(diffTestFiles()).SetSize(80, 12))
	repeatY := tea.KeyPressMsg{Code: 'y', Text: "y", IsRepeat: true}
	repeated, _ := m.Update(repeatY)
	if next, _ := repeated.Update(enter); next.Decision() != DiffPending {
		t.Errorf("repeat y armed the acknowledgement: %v", next.Decision())
	}
	m = diffPress(diffPress(m, "y"), "n")
	if next, _ := m.Update(enter); next.Decision() != DiffPending {
		t.Errorf("Enter after n decided %v", next.Decision())
	}
	m = diffPress(m, "y")
	m, _ = m.Update(repeatY)
	if next, _ := m.Update(enter); next.Decision() != DiffConfirm {
		t.Fatalf("repeat y disarmed the acknowledgement")
	}
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

// TestDiffNoRowsCannotAnswer catches skipping the visibility guard or clipping its refusal under the title.
func TestDiffNoRowsCannotAnswer(t *testing.T) {
	cfg := DiffConfig{AlternateKey: "c", Title: strings.Repeat("Title\n", 12)}
	m := diffArm(t, NewDiff(cfg).WithFiles(diffTestFiles()).SetSize(80, 30)).SetSize(80, 12)
	for _, key := range []tea.KeyPressMsg{{Code: tea.KeyEnter}, {Code: 'c', Text: "c"}} {
		next, _ := m.Update(key)
		if next.Decision() != DiffPending {
			t.Errorf("%s answered a diff with no rows: %v", key, next.Decision())
		}
		if view := ansi.Strip(next.View()); !strings.Contains(view, "No diff data is visible") {
			t.Errorf("refusal was not shown when the title filled the box:\n%s", view)
		}
	}
}

// diffFirstChange reads the first numbered change from the rendered fixture.
func diffFirstChange(m DiffModel) string {
	_, rest, found := strings.Cut(ansi.Strip(m.View()), "change ")
	if !found || len(rest) < 2 {
		return ""
	}
	return rest[:2]
}

// TestDiffPageDownWithPinnedContext catches using the unpinned height and skipping unseen rows.
func TestDiffPageDownWithPinnedContext(t *testing.T) {
	m := NewDiff(DiffConfig{}).WithFiles(diffLongTestFiles()).SetSize(80, 12)
	for range 4 {
		m = diffPress(m, "j")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	if first := diffFirstChange(m); first != "09" {
		t.Fatalf("Page Down from a pinned view should start at the next unseen row, got %q:\n%s", first, ansi.Strip(m.View()))
	}
}

// TestDiffResizeKeepsSourceRow catches dropping the anchor and jumping to the file top.
func TestDiffResizeKeepsSourceRow(t *testing.T) {
	m := NewDiff(DiffConfig{}).WithFiles(diffLongTestFiles()).SetSize(80, 12)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	first := diffFirstChange(m)
	if first == "" || first == "01" {
		t.Fatalf("fixture did not scroll inside the section: %q", first)
	}
	for _, w := range []int{80, 48} {
		if got := diffFirstChange(m.SetSize(w, 13)); got != first {
			t.Errorf("resize to %d columns jumped to %q, want source row %q", w, got, first)
		}
	}
}

// TestDiffRespectsHostSize catches a terminal resize overwriting the embedder's allotted box.
func TestDiffRespectsHostSize(t *testing.T) {
	m := NewDiff(DiffConfig{}).WithFiles(diffTestFiles())
	if _, tick := m.Update(m.Init()()); tick != nil {
		t.Error("an unsized diff started settling")
	}
	msg := tea.WindowSizeMsg{Width: 100, Height: 24}
	if next, tick := m.Update(msg); tick == nil || len(strings.Split(next.View(), "\n")) != msg.Height {
		t.Error("standalone sizing did not use the terminal height")
	}
	m = m.SetSize(60, 10)
	m, _ = m.Update(msg)
	lines := strings.Split(ansi.Strip(m.View()), "\n")
	if len(lines) != 10 {
		t.Errorf("terminal message replaced the host height: %d", len(lines))
	}
	for _, line := range lines {
		if ansi.StringWidth(line) > 60 {
			t.Errorf("terminal message replaced the host width: %q", line)
		}
	}
}

// TestDiffLeaveCancel checks the embedded leave question: one Esc opens it,
// the second Esc cancels, and Ctrl+C interrupts from inside it.
func TestDiffLeaveCancel(t *testing.T) {
	m := NewDiff(DiffConfig{Leave: &LeaveConfig{Title: "Leave?"}}).WithFiles(diffTestFiles()).SetSize(80, 12)
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
	m = NewDiff(DiffConfig{Leave: &LeaveConfig{Title: "Leave?"}}).WithFiles(diffTestFiles()).SetSize(80, 12)
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

// TestDiffHelpShowsAvailableAction catches advertising a shadowed key or losing y in narrow help.
func TestDiffHelpShowsAvailableAction(t *testing.T) {
	t.Run("shadowed alternate", func(t *testing.T) {
		m := NewDiff(DiffConfig{AlternateKey: "j", AlternateLabel: "publish"}).WithFiles(diffLongTestFiles()).SetSize(120, 12)
		if view := ansi.Strip(m.View()); strings.Contains(view, "publish") {
			t.Fatalf("help advertised an alternate action bound to scrolling:\n%s", view)
		}
	})
	t.Run("narrow acknowledgement", func(t *testing.T) {
		m := NewDiff(DiffConfig{Acknowledge: "Discard"}).WithFiles(diffTestFiles()).SetSize(30, 14)
		if view := ansi.Strip(m.View()); !strings.Contains(view, "y accept") {
			t.Fatalf("help didn't show how to acknowledge:\n%s", view)
		}
	})
}

// TestDiffCannotAcknowledgeWhileTooSmall catches handling y without checking visibility.
func TestDiffCannotAcknowledgeWhileTooSmall(t *testing.T) {
	m := diffArm(t, NewDiff(DiffConfig{Acknowledge: "Discard"}).WithFiles(diffTestFiles()).SetSize(80, 12))
	m = diffPress(m.SetSize(20, 5), "y").SetSize(80, 12)
	if next := diffPress(m, "enter"); next.Decision() != DiffPending {
		t.Fatal("acknowledgement on a too-small screen allowed consent after resize")
	}
}

// TestDiffPreloadWaitsForSettleMessage catches starting the settle timer during preload.
func TestDiffPreloadWaitsForSettleMessage(t *testing.T) {
	old := diffSettleDelay
	diffSettleDelay = 10 * time.Millisecond
	t.Cleanup(func() { diffSettleDelay = old })
	m := NewDiff(DiffConfig{}).WithFiles(diffTestFiles()).SetSize(80, 12)
	time.Sleep(diffSettleDelay) // simulate a preloaded diff before the program starts
	if next := diffPress(m, "enter"); next.Decision() != DiffPending {
		t.Fatalf("preloading armed consent before the program started: %v", next.Decision())
	}
	start := m.Init()
	if start == nil {
		t.Fatal("Init did not start the settle message flow")
	}
	m, tick := m.Update(start())
	if tick == nil {
		t.Fatal("visible diff did not schedule the settle tick")
	}
	if next := diffPress(m, "enter"); next.Decision() != DiffPending {
		t.Fatalf("consent was accepted before delivering the settle message: %v", next.Decision())
	}
	m, _ = m.Update(tick())
	if next := diffPress(m, "enter"); next.Decision() != DiffConfirm {
		t.Fatalf("visible diff could not be confirmed after the settle message: %v", next.Decision())
	}
}

// TestDiffIgnoresSettleForReplacedFiles catches a stale timer arming replacement content.
func TestDiffIgnoresSettleForReplacedFiles(t *testing.T) {
	old := diffSettleDelay
	diffSettleDelay = 0
	t.Cleanup(func() { diffSettleDelay = old })
	m := NewDiff(DiffConfig{}).WithFiles(diffTestFiles()).SetSize(80, 12)
	m, oldTick := m.Update(m.Init()())
	m = m.WithFiles(diffLongTestFiles())
	m, currentTick := m.Update(m.Init()())
	m, _ = m.Update(oldTick())
	if next := diffPress(m, "enter"); next.Decision() != DiffPending {
		t.Fatalf("a stale settle message armed the replacement diff: %v", next.Decision())
	}
	m, _ = m.Update(currentTick())
	if next := diffPress(m, "enter"); next.Decision() != DiffConfirm {
		t.Fatalf("the replacement diff did not settle on its own message: %v", next.Decision())
	}
}

// TestDiffHeadersAloneCannotConfirm catches counting file headers as visible data.
func TestDiffHeadersAloneCannotConfirm(t *testing.T) {
	files := diffTestFiles()
	files[0].Sections[0].Notes = []string{"review note"}
	m := diffArm(t, NewDiff(DiffConfig{}).WithFiles(files).SetSize(80, 12)).SetSize(80, 7)
	next := diffPress(m, "enter")
	if next.Decision() != DiffPending {
		t.Fatalf("a file header without data allowed consent: %v", next.Decision())
	}
	if view := ansi.Strip(next.View()); !strings.Contains(view, "No diff data is visible") {
		t.Fatalf("no-data refusal wasn't shown:\n%s", view)
	}
}

// TestDiffResizeUsesReviewedSnapshot catches a reflow reading mutated caller slices.
func TestDiffResizeUsesReviewedSnapshot(t *testing.T) {
	files := diffLongTestFiles()
	m := NewDiff(DiffConfig{}).WithFiles(files).SetSize(80, 12)
	files[0].Sections[0].Ops[0].Text = "unreviewed change"
	if view := ansi.Strip(m.SetSize(75, 12).View()); !strings.Contains(view, "change 01") {
		t.Fatalf("a resize picked up caller mutations instead of the reviewed snapshot:\n%s", view)
	}
}
