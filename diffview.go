package teakit

import (
	"fmt"
	"slices"
	"strings"
)

// Diff is a screen that shows changes, one file after another. Up and down
// scroll it, left and right go to the file before or after, and enter goes on.
// The head of the file at the top stays there while its lines scroll. Run it
// with Step.
type Diff struct {
	// Title is shown above the changes; empty for none.
	Title string
	// Body is the explanation under the title; nil for none.
	Body  func(width int) string
	Files []DiffFile
	Hint  string

	rows  []string
	heads []span // the rows of each file's head
	top   int    // the row at the top, when they do not all fit
	past  bool   // that is below the last page, to show a file from its head
}

func (d *Diff) start() { d.top, d.past = 0, false }

func (d *Diff) resize(width int) { d.rows, d.heads = diffRows(d.Files, width) }

// press scrolls the changes, which show in room rows.
func (d *Diff) press(key string, room int) {
	last := len(d.rows) - room
	if last <= 0 {
		return
	}
	top := d.at(room)
	file, page := d.file(top), room-1 // without the count
	for _, head := range d.heads {
		page = min(page, room-1-(head.to-head.from))
	}
	page = max(page, 1)
	switch key {
	case "up", "k":
		top--
	case "down", "j":
		top = max(top, min(top+1, last))
	case "pgup":
		top -= page
	case "pgdown":
		top = max(top, min(top+page, last))
	case "home", "g":
		top = 0
	case "end", "G":
		top = last
	case "left":
		top = d.heads[max(file-1, 0)].from
	case "right":
		if file+1 < len(d.heads) {
			top = d.heads[file+1].from
		}
	}
	d.top, d.past = max(top, 0), top > last
}

// at returns the row at the top of room rows, which the terminal's size may
// have moved.
func (d *Diff) at(room int) int {
	last := len(d.rows) - room
	if d.past {
		last = max(last, d.heads[len(d.heads)-1].from)
	}
	return min(d.top, last)
}

// file returns the file that row is of.
func (d *Diff) file(row int) int {
	file := 0
	for i, head := range d.heads {
		if head.from <= row {
			file = i
		}
	}
	return file
}

// window draws the rows from the one at the top on. The file there keeps its
// head until the next file pushes it out, and the last row counts the lines
// below, where both leave a row between them.
func (d *Diff) window(room int) []string {
	top := d.at(room)
	file, next := d.file(top), len(d.rows)
	if file+1 < len(d.heads) {
		next = d.heads[file+1].from
	}
	head := d.heads[file]
	pinned := min(head.to-head.from, next-top)
	if pinned > room-2 {
		pinned = 0
	}
	end := min(top+room, len(d.rows))
	shown := slices.Concat(d.rows[head.to-pinned:head.to], d.rows[top+pinned:end])
	if rest := len(d.rows) - end; rest > 0 && room >= 3 {
		shown[room-1] = Muted.Render(fmt.Sprintf("  ↓ %d more lines", rest+1))
	}
	// Past the last page, the rows do not fill the room.
	return append(shown, make([]string, room-len(shown))...)
}

func (d *Diff) draw(width int) rows {
	r := rows{items: d.rows, part: d.window}
	if d.Title != "" {
		r.above = strings.Split(Header.Render(Wrap(d.Title, width)), "\n")
		if len(d.Files) > 1 {
			r.above = beside(r.above, diffCounts(d.Files...), width)
		}
	}
	if d.Body != nil {
		r.above = append(r.above, strings.Split(d.Body(width), "\n")...)
	}
	if len(r.above) > 0 && len(r.items) > 0 {
		r.above = append(r.above, "")
	}
	if d.Hint != "" {
		r.below = append([]string{""}, strings.Split(Muted.Render(Wrap(d.Hint, width)), "\n")...)
	}
	return r
}
