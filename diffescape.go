// Escape and measurement helpers for the diff view.
// Escaped controls stay visible without reaching the terminal as raw input.
package teakit

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// SGR attributes used by the diff view. Colors are ANSI-16 foreground
// indices, so the terminal's palette decides the shade.
const (
	diffSGRBold   = "1"
	diffSGRFaint  = "2"
	diffSGRChar   = "3"
	diffSGRRed    = "31"
	diffSGRGreen  = "32"
	diffSGRYellow = "33"
	diffSGRCyan   = "36"
)

// diffStyle wraps nonempty text in SGR without styling blank cells.
func diffStyle(code, s string) string {
	if s == "" {
		return ""
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

// diffWidth measures visible terminal cells, ignoring SGR sequences.
func diffWidth(s string) int { return ansi.StringWidth(s) }

// diffPadRight pads to width cells without truncating wider text.
func diffPadRight(s string, width int) string {
	return s + strings.Repeat(" ", max(0, width-diffWidth(s)))
}

// diffPadLeft pads to width cells without truncating wider text.
func diffPadLeft(s string, width int) string {
	return strings.Repeat(" ", max(0, width-diffWidth(s))) + s
}

// An atom is a whole grapheme or a visible, reverse-highlighted escape. Keep
// escapes atomic without reparsing SGR; reverse-off preserves outer colors.
type diffAtom struct {
	text  string
	width int
}

// diffAtoms groups graphemes, marks controls, and expands tabs to cell stops.
func diffAtoms(s string) []diffAtom {
	var atoms []diffAtom
	col := 0
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		var escape string
		switch {
		case r == '\t':
			for n := 4 - col%4; n > 0; n-- {
				atoms = append(atoms, diffAtom{" ", 1})
				col++
			}
		case r == utf8.RuneError && size == 1:
			escape = fmt.Sprintf(`\x%02x`, s[0])
		case r < 0x20 || (r >= 0x7f && r <= 0x9f):
			escape = fmt.Sprintf(`\x%02x`, r)
		case diffEscapeU(r):
			escape = fmt.Sprintf(`\u%04x`, r)
		default:
			cluster, w := ansi.FirstGraphemeCluster(s, ansi.GraphemeWidth)
			size = len(cluster)
			atoms = append(atoms, diffAtom{cluster, w})
			col += w
		}
		if escape != "" {
			atoms = append(atoms, diffAtom{"\x1b[7m" + escape + "\x1b[27m", len(escape)})
			col += len(escape)
		}
		s = s[size:]
	}
	return atoms
}

// diffEscapeU reports whether a bidi control or invisible rune needs a \u escape.
func diffEscapeU(r rune) bool {
	switch r {
	case 0x061c, // ARABIC LETTER MARK
		0x200e, 0x200f, // LRM, RLM
		0x202a, 0x202b, 0x202c, 0x202d, 0x202e, // embeddings, overrides, PDF
		0x2066, 0x2067, 0x2068, 0x2069, // isolates
		0x200b, // ZERO WIDTH SPACE
		0x2060, // WORD JOINER
		0xfeff: // ZERO WIDTH NO-BREAK SPACE
		return true
	}
	return false
}

// diffWrapLine escapes raw text and wraps it, preserving every source space.
func diffWrapLine(s string, width int) []string {
	return diffWrapIndented(s, width, 0)
}

// diffWrapIndented reserves the continuation indent before packing each row.
// A glyph or escape wider than the entire box still gets its own row.
func diffWrapIndented(s string, width, indent int) []string {
	atoms := diffAtoms(s)
	if len(atoms) == 0 {
		return []string{""}
	}
	indent = min(indent, max(0, width-6)) // reserve space for an atomic \uNNNN
	var out []string
	prefix := 0
	for len(atoms) > 0 {
		end, space := 0, -1
		used := prefix
		for end < len(atoms) && (end == 0 || used+atoms[end].width <= width) {
			if atoms[end].text == " " {
				space = end
			}
			used += atoms[end].width
			end++
		}
		if end < len(atoms) && space >= 0 {
			end = space + 1
		}
		var line strings.Builder
		line.WriteString(strings.Repeat(" ", prefix))
		for _, a := range atoms[:end] {
			line.WriteString(a.text)
		}
		out = append(out, line.String())
		atoms = atoms[end:]
		prefix = indent
	}
	return out
}
