package teakit

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// optionList is a Select or a MultiSelect as it is drawn.
type optionList struct {
	title   string
	body    func(width int) string
	options []string
	heads   map[int]string
	checked []bool // nil when the options cannot be checked
	input   *Input // drawn in the last option's place
	cursor  int
	invalid string // why the answer was rejected
	hint    string
}

func (l optionList) draw(width int) rows {
	r := rows{cursor: l.cursor}
	r.above = strings.Split(bold.Render(Wrap(l.title, width)), "\n")
	if l.body != nil {
		r.above = append(r.above, strings.Split(l.body(width), "\n")...)
	}
	if len(l.options) > 0 {
		r.above = append(r.above, "")
	}
	for i := range l.options {
		head, ok := l.heads[i]
		if ok && i > 0 {
			r.items = append(r.items, "")
		}
		if head != "" {
			r.items = append(r.items, strings.Split(head, "\n")...)
		}
		if i == l.cursor {
			r.row = len(r.items)
		}
		r.add(strings.Split(l.option(i, width), "\n"))
	}
	r.below = []string{""}
	if l.invalid != "" {
		r.below = append(r.below, strings.Split(alert.Render(Wrap(l.invalid, width)), "\n")...)
		r.below = append(r.below, "")
	}
	r.below = append(r.below, strings.Split(dim.Render(Wrap(l.hint, width)), "\n")...)
	return r
}

// option draws option i.
func (l optionList) option(i, width int) string {
	number := fmt.Sprintf("%*d. ", len(strconv.Itoa(len(l.options))), i+1)
	switch {
	case l.checked == nil:
	case l.checked[i]:
		number += "[x] "
	default:
		number += "[ ] "
	}
	indent := 2 + len(number)
	if l.input != nil && i == len(l.options)-1 {
		prefix := "  " + number
		if i == l.cursor {
			prefix = bold.Render("❯ " + number)
		}
		input := prefix + l.input.view()
		if invalid := l.input.text.invalid; invalid != "" {
			pad := strings.Repeat(" ", indent)
			input += "\n" + alert.Render(pad+strings.ReplaceAll(Wrap(invalid, max(width-indent, 1)), "\n", "\n"+pad))
		}
		return input
	}
	o := l.options[i]
	if room := width - indent; room > 0 && lipgloss.Width(o) > room {
		o = strings.ReplaceAll(Wrap(o, room), "\n", "\n"+strings.Repeat(" ", indent))
	}
	if i == l.cursor {
		return bold.Render("❯ " + number + o)
	}
	return "  " + number + o
}
