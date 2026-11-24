package teakit

import (
	"fmt"
	"slices"
	"strings"
)

// rows is a question as it is drawn: the rows of its items, which are options
// or a form's fields, and those above and below them. Item i's own rows are
// items[from[i]:to[i]].
type rows struct {
	above, items, below []string
	from, to            []int
	cursor              int // the item the cursor is on
	row                 int // the row of items the cursor is on
}

// add appends the next item.
func (r *rows) add(item []string) {
	r.from = append(r.from, len(r.items))
	r.items = append(r.items, item...)
	r.to = append(r.to, len(r.items))
}

// view draws the question under head, in at most height rows once the height
// is known. On a short terminal the items scroll; when they would have fewer
// than three rows, the text above loses its top rows first, then the hint its
// bottom ones.
func (r rows) view(head []string, height int) string {
	above, items, below := slices.Concat(head, r.above), r.items, r.below
	room := height - len(above) - len(below)
	if height > 0 && room < len(items) {
		least := min(len(items), 5) // three rows between the two counts
		cut := min(max(least-room, 0), len(above))
		above, room = above[cut:], room+cut
		cut = min(max(least-room, 0), len(below))
		below, room = below[:len(below)-cut], room+cut
		if room < len(items) {
			items = r.window(room)
		}
	}
	return strings.Join(slices.Concat(above, items, below), "\n")
}

// window returns room of the items' rows, those around the cursor's item, or
// around the cursor's row when the item does not fit. With three rows or
// more, the first and last of them count the items that begin outside the
// rows between.
func (r rows) window(room int) []string {
	count := 0
	if room >= 3 {
		count = 1
	}
	top := (r.from[r.cursor] + r.to[r.cursor] - room) / 2
	top = max(min(top, r.row-count), r.row-room+1+count)
	top = max(min(top, len(r.items)-room), 0)
	end := top + room
	shown := slices.Clone(r.items[top:end])
	if count == 0 {
		return shown
	}
	var before, after int
	for i, row := range r.from {
		switch {
		case i == r.cursor:
		case row <= top:
			before++
		case row >= end-1:
			after++
		}
	}
	if top > 0 && before > 0 {
		shown[0] = Muted.Render(fmt.Sprintf("  ↑ %d more", before))
	}
	if end < len(r.items) && after > 0 {
		shown[room-1] = Muted.Render(fmt.Sprintf("  ↓ %d more", after))
	}
	return shown
}
