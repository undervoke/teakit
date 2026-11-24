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
	// part returns the rows of items to show when they do not all fit; nil for
	// those around the cursor.
	part func(room int) []string
}

// add appends the next item.
func (r *rows) add(item []string) {
	r.from = append(r.from, len(r.items))
	r.items = append(r.items, item...)
	r.to = append(r.to, len(r.items))
}

// view draws the question under head, in at most height rows once the height
// is known. On a short terminal the items scroll.
func (r rows) view(head []string, height int) string {
	cutAbove, cutBelow, room := r.fit(len(head), height)
	above, items, below := slices.Concat(head, r.above)[cutAbove:], r.items, r.below[:len(r.below)-cutBelow]
	if room < len(items) {
		part := r.window
		if r.part != nil {
			part = r.part
		}
		items = part(room)
	}
	return strings.Join(slices.Concat(above, items, below), "\n")
}

// fit returns how many rows the items show in, under head rows of other text.
// When those would be fewer than three, the text above is cut by its top rows
// first, then the hint by its bottom ones.
func (r rows) fit(head, height int) (cutAbove, cutBelow, room int) {
	room = height - head - len(r.above) - len(r.below)
	if height <= 0 || room >= len(r.items) {
		return 0, 0, len(r.items)
	}
	least := min(len(r.items), 5) // three rows between the two counts
	cutAbove = min(max(least-room, 0), head+len(r.above))
	room += cutAbove
	cutBelow = min(max(least-room, 0), len(r.below))
	return cutAbove, cutBelow, room + cutBelow
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
