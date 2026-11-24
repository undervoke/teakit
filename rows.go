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
	// scrolls is set while the cursor's item is scrolled. When it does not fit,
	// its first pinned rows stay, and the others show from offset on.
	scrolls        bool
	pinned, offset int
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
	above, items, below := slices.Concat(head, r.above), r.items, r.below
	if cutAbove, cutBelow, room := r.fit(len(head), height); height > 0 && room < len(items) {
		above, below, items = above[cutAbove:], below[:len(below)-cutBelow], r.window(room)
	}
	return strings.Join(slices.Concat(above, items, below), "\n")
}

// fit returns the rows the items have under head rows of other text. When
// they would have fewer than three, the text above is cut by its top rows
// first, then the hint by its bottom ones.
func (r rows) fit(head, height int) (cutAbove, cutBelow, room int) {
	room = height - head - len(r.above) - len(r.below)
	if height <= 0 || room >= len(r.items) {
		return 0, 0, room
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
	if r.scrolls && !fits(r.to[r.cursor]-r.from[r.cursor], room) {
		return r.scrolled(room)
	}
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

// scroll is how far an item scrolls, and how many of its rows show at once.
type scroll struct{ last, page int }

// scroll returns how the cursor's item scrolls in room rows.
func (r rows) scroll(room int) scroll {
	if !r.scrolls || room >= len(r.items) || fits(r.to[r.cursor]-r.from[r.cursor], room) {
		return scroll{}
	}
	n := r.to[r.cursor] - r.from[r.cursor]
	pinned, shown := scrollRoom(r.pinned, n, room)
	return scroll{n - pinned - shown, shown}
}

// scrolled draws the cursor's item alone in room rows: its pinned rows, then
// those from offset on, between counts of the rows hidden above and below
// them. At the end, the last count is of the items below.
func (r rows) scrolled(room int) []string {
	item := r.items[r.from[r.cursor]:r.to[r.cursor]]
	pinned, shown := scrollRoom(r.pinned, len(item), room)
	head, body := item[:pinned], item[pinned:]
	top := max(min(r.offset, len(body)-shown), 0)
	if pinned == 0 {
		return body[top : top+shown]
	}
	out := slices.Clone(head)
	if top > 0 {
		out = append(out, Muted.Render("  ↑ "+moreLines(top)))
	} else {
		shown++ // in the count's place
	}
	end := min(top+shown, len(body))
	out = append(out, body[top:end]...)
	last := ""
	if rest := len(body) - end; rest > 0 {
		last = Muted.Render("  ↓ " + moreLines(rest))
	} else if after := len(r.from) - 1 - r.cursor; after > 0 {
		last = Muted.Render(fmt.Sprintf("  ↓ %d more", after))
	}
	return append(out, last)
}

func moreLines(n int) string {
	if n == 1 {
		return "1 more line"
	}
	return fmt.Sprintf("%d more lines", n)
}

// fits reports whether an item's rows fit between the counts of the items
// hidden above and below it.
func fits(rows, room int) bool { return rows <= room-2 }

// scrollRoom returns how many of an item's rows stay pinned in room rows, and
// how many of the others show between the two counts. Where they would not
// have a row, nothing is pinned or counted.
func scrollRoom(pinned, rows, room int) (int, int) {
	if shown := room - pinned - 2; shown > 0 {
		return pinned, min(shown, rows-pinned)
	}
	return 0, min(room, rows)
}
