// Escape and measurement helpers for the diff view.
//
// The layout measures and wraps the escaped text, so a hostile or stray
// control character shows up visibly instead of breaking the table or
// reaching the terminal raw.
package teakit

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// SGR attribute codes used by the diff view. Colors are foreground ANSI-16
// indices without backgrounds; the terminal's own palette decides the shade.
const (
	diffSGRBold    = "1"
	diffSGRFaint   = "2"
	diffSGRChar    = "3"
	diffSGRReverse = "7"
	diffSGRRed     = "31"
	diffSGRGreen   = "32"
	diffSGRYellow  = "33"
	diffSGRCyan    = "36"
)

// diffStyle wraps s in one SGR sequence. An empty string stays empty so
// blank cells never emit stray resets.
func diffStyle(code, s string) string {
	if s == "" {
		return ""
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

// diffWidth reports the visible cell width, ignoring SGR sequences.
func diffWidth(s string) int {
	return ansi.StringWidth(s)
}

// diffPadRight pads s with spaces to exactly width cells. Strings already
// wider than width are returned unchanged.
func diffPadRight(s string, width int) string {
	if pad := width - diffWidth(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

// diffPadLeft pads s on the left with spaces to exactly width cells.
func diffPadLeft(s string, width int) string {
	if pad := width - diffWidth(s); pad > 0 {
		return strings.Repeat(" ", pad) + s
	}
	return s
}

// diffEscape renders tabs, control characters, and invisible Unicode as
// visible text. Tabs expand to 4-cell stops relative to the text start.
// C0 (except tab), DEL, C1, and invalid UTF-8 bytes become \xNN; the listed
// bidi controls and zero-width characters become \uNNNN; everything else,
// including ZWJ sequences and variation selectors, is kept. Escape
// sequences are wrapped in SGR reverse (reset with reverse-off so an outer
// color survives), and the result reports whether any sequence was emitted.
func diffEscape(s string) (string, bool) {
	var b strings.Builder
	col := 0
	escaped := false
	for len(s) > 0 {
		c := s[0]
		if c == '\t' {
			next := col + 4 - col%4
			b.WriteString(strings.Repeat(" ", next-col))
			col = next
			s = s[1:]
			continue
		}
		if c < utf8.RuneSelf {
			if c < 0x20 || c == 0x7f {
				b.WriteString(diffStyle(diffSGRReverse, fmt.Sprintf(`\x%02x`, c)))
				col += 4
				escaped = true
			} else {
				b.WriteByte(c)
				col++
			}
			s = s[1:]
			continue
		}
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError && size <= 1 {
			b.WriteString(diffStyle(diffSGRReverse, fmt.Sprintf(`\x%02x`, s[0])))
			col += 4
			escaped = true
			s = s[1:]
			continue
		}
		switch {
		case r >= 0x80 && r <= 0x9f:
			b.WriteString(diffStyle(diffSGRReverse, fmt.Sprintf(`\x%02x`, r)))
			col += 4
			escaped = true
		case diffEscapeU(r):
			b.WriteString(diffStyle(diffSGRReverse, fmt.Sprintf(`\u%04x`, r)))
			col += 6
			escaped = true
		default:
			b.WriteString(s[:size])
			col += diffWidth(s[:size])
		}
		s = s[size:]
	}
	return b.String(), escaped
}

// diffEscapeU reports whether r is a bidi control or zero-width character
// that must render as a \uNNNN sequence.
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

// diffAtom is one unbreakable wrapping unit: either a single grapheme
// cluster or one whole \xNN / \uNNNN escape. Spaces are their own atoms so
// the wrapper can prefer breaking after them.
type diffAtom struct {
	text  string // escaped, unstyled
	style string // styled form of text
	width int
	space bool
}

// diffEscapeSpans returns the byte ranges of the \xNN and \uNNNN sequences
// open, so a styled sequence is found by its opener; a literal backslash in
// the source text never carries one.
func diffEscapeSpans(styled string) [][2]int {
	const open = "\x1b[7m"
	const close = "\x1b[27m"
	var spans [][2]int
	for i := 0; i+len(open) < len(styled); {
		if !strings.HasPrefix(styled[i:], open) {
			i++
			continue
		}
		start := i + len(open)
		end := strings.Index(styled[start:], close)
		if end < 0 {
			break
		}
		spans = append(spans, [2]int{start, start + end})
		i = start + end + len(close)
	}
	return spans
}

// diffWrapLine wraps one styled escaped line to at most limit cells per
// row. The break prefers the last space within the limit, kept at the end
// of the row so the text round-trips; otherwise it breaks at an atom
// boundary (one grapheme cluster, or one whole escape sequence). A limit
// below 1 returns the line unchanged.
func diffWrapLine(styled string, limit int) []string {
	if limit < 1 {
		return []string{styled}
	}
	plain := ansi.Strip(styled)
	return diffWrapAtoms(diffAtomizeStyled(plain, styled, diffEscapeSpans(styled)), limit)
}

// diffAtomizeStyled pairs the plain text with its styled form. plain is the
// SGR-stripped escaped line; styled carries the reverse spans at the same
// byte positions only where diffEscape put them, so spans (byte ranges into
// plain) locate each sequence and the styled bytes between spans are the
// plain bytes verbatim.
func diffAtomizeStyled(plain, styled string, spans [][2]int) []diffAtom {
	var atoms []diffAtom
	pos := 0
	styledPos := 0
	for _, span := range spans {
		if span[0] > pos {
			chunk := plain[pos:span[0]]
			// The styled text between spans equals the plain text.
			styledPos += len(chunk)
			for len(chunk) > 0 {
				cluster, w := ansi.FirstGraphemeCluster(chunk, ansi.GraphemeWidth)
				atoms = append(atoms, diffAtom{text: cluster, style: cluster, width: w, space: cluster == " "})
				chunk = chunk[len(cluster):]
			}
		}
		seq := plain[span[0]:span[1]]
		// Find the styled sequence: reverse-open + seq + reverse-off.
		styledSeq := styled[styledPos : styledPos+len("\x1b[7m")+len(seq)+len("\x1b[27m")]
		atoms = append(atoms, diffAtom{text: seq, style: styledSeq, width: len(seq), space: false})
		styledPos += len(styledSeq)
		pos = span[1]
	}
	if pos < len(plain) {
		chunk := plain[pos:]
		for len(chunk) > 0 {
			cluster, w := ansi.FirstGraphemeCluster(chunk, ansi.GraphemeWidth)
			atoms = append(atoms, diffAtom{text: cluster, style: cluster, width: w, space: cluster == " "})
			chunk = chunk[len(cluster):]
		}
	}
	return atoms
}

// diffWrapAtoms greedily packs atoms into rows of at most limit cells.
func diffWrapAtoms(atoms []diffAtom, limit int) []string {
	var out []string
	var line []diffAtom
	width := 0
	spaceIdx := -1
	emit := func(upto int) {
		var sb strings.Builder
		for _, a := range line[:upto] {
			sb.WriteString(a.style)
		}
		out = append(out, sb.String())
		rest := append([]diffAtom(nil), line[upto:]...)
		line = rest
		width = 0
		spaceIdx = -1
		for i, a := range line {
			width += a.width
			if a.space {
				spaceIdx = i
			}
		}
	}
	for _, a := range atoms {
		if width+a.width > limit && len(line) > 0 {
			if spaceIdx >= 0 {
				emit(spaceIdx + 1)
			} else {
				emit(len(line))
			}
		}
		// An atom wider than the limit still gets its own row.
		line = append(line, a)
		width += a.width
		if a.space {
			spaceIdx = len(line) - 1
		}
	}
	if len(line) > 0 {
		emit(len(line))
	}
	if len(out) == 0 {
		out = append(out, "")
	}
	return out
}
