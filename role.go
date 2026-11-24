package teakit

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Role is what a text is for, which fixes its look in every tool. The screens
// give their own parts a role; a tool gives one to its own text with Render.
type Role int

const (
	Header  Role = iota // a screen's title, or that of the field at the cursor
	Subhead             // a heading among options, or a field's title
	Focus               // the option at the cursor
	Chosen              // the answer locked in a field
	Muted               // hints and help
	Success             // what worked
	Warning             // what to check first
	Error               // what failed
)

// The ANSI colors leave the shades to the terminal's theme.
var roleStyles = [...]lipgloss.Style{
	Header:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan),
	Subhead: lipgloss.NewStyle().Foreground(lipgloss.Cyan),
	Focus:   lipgloss.NewStyle().Bold(true),
	Chosen:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Green),
	Muted:   lipgloss.NewStyle().Faint(true),
	Success: lipgloss.NewStyle().Foreground(lipgloss.Green),
	Warning: lipgloss.NewStyle().Foreground(lipgloss.Yellow),
	Error:   lipgloss.NewStyle().Foreground(lipgloss.Red),
}

// Render draws text in the role. A part of text that has a style already
// keeps it over the role's, so a Warning in a title is yellow and bold.
func (r Role) Render(text string) string {
	on, off, _ := strings.Cut(roleStyles[r].Render("x"), "x")
	// Where such a part ends, it resets every style, the role's too.
	resume := strings.NewReplacer("\x1b[m", "\x1b[m"+on, "\x1b[0m", "\x1b[0m"+on)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		// Wrap reads the indent, so it stays plain.
		if words := strings.TrimLeft(line, " "); words != "" {
			lines[i] = line[:len(line)-len(words)] + on + resume.Replace(words) + off
		}
	}
	return strings.Join(lines, "\n")
}
