// Command diff runs an override review as an inline step: it shows what
// applying would change and answers only Enter on the complete diff.
package main

import (
	"context"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/undervoke/teakit"
)

func main() {
	files := []teakit.DiffFile{
		{
			Path: "~/.claude/settings.json",
			Sections: []teakit.DiffSection{
				{
					Label: "/statusLine",
					Notes: []string{"replaces a value this tool did not write"},
					Ops: []teakit.DiffOp{
						{Kind: teakit.OpEqual, Text: "{"},
						{Kind: teakit.OpDelete, Text: `  "command": "ccstatusline",`},
						{Kind: teakit.OpInsert, Text: `  "command": "coa statusline --compact",`},
						{Kind: teakit.OpEqual, Text: `  "type": "command"`},
						{Kind: teakit.OpEqual, Text: "}"},
					},
				},
				{
					Label:        "/experimental",
					BeforeAbsent: true,
					Ops: []teakit.DiffOp{
						{Kind: teakit.OpInsert, Text: `"experimental": true`},
					},
				},
			},
		},
	}
	d, err := teakit.ReviewDiff(context.Background(), teakit.DiffConfig{
		Title:       "Review overrides",
		Body:        "Applying replaces these existing values. Nothing changes until you press Enter.",
		ApplyLabel:  "apply",
		Acknowledge: "Discard the data listed above",
	}, files, tea.WithOutput(os.Stderr))
	if err != nil {
		fmt.Fprintln(os.Stderr, "diff:", err)
		os.Exit(1)
	}
	switch d {
	case teakit.DiffInterrupt:
		fmt.Println("interrupted; nothing was changed")
		os.Exit(130)
	case teakit.DiffConfirm:
		fmt.Println("applying the overrides shown above")
	case teakit.DiffBack:
		fmt.Println("back; nothing was changed")
	default:
		fmt.Println("cancelled; nothing was changed")
	}
}
