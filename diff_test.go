package teakit

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// diffTestFiles returns a small diff.
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
	files[0].Sections[0].Ops = []DiffOp{{Kind: OpInsert, Text: strings.Repeat("a", 74) + "\x01" + `\x01`}}
	out := RenderDiff(files, 80)
	if strings.Count(out, "\x1b[7m"+`\x01`) != 1 || strings.Count(ansi.Strip(out), `\x01`) != 2 {
		t.Fatalf("the wrapped control byte must be distinguishable from the literal escape:\n%q", out)
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
	const width, lead = 34, 4
	files := diffTestFiles()
	text := " " + strings.Repeat("x", width-lead-1) + "界"
	files[0].Sections[0] = DiffSection{Ops: []DiffOp{{Kind: OpInsert, Text: text}}, BeforeAbsent: true}
	_, view, _ := strings.Cut(ansi.Strip(RenderDiff(files, width)), "(absent before)\n")
	var got string
	for _, row := range strings.Split(view, "\n") {
		if n := ansi.StringWidth(row); n > width {
			t.Errorf("wrapped row exceeds width: %d > %d", n, width)
		}
		got += row[lead:]
	}
	if got != text {
		t.Fatalf("wrapping lost content: %q", got)
	}
}
