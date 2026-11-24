package main

import (
	"context"
	_ "embed"

	"github.com/undervoke/teakit"
)

//go:embed diff.go
var diffSource string

func demoDiff(ctx context.Context) (string, error) {
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
	d, err := teakit.ReviewDiff(ctx, teakit.DiffConfig{
		Title:       "Review overrides",
		Body:        "Applying replaces these existing values. Nothing changes until you press Enter.",
		ApplyLabel:  "apply",
		Acknowledge: "Discard the data listed above",
	}, files)
	if err != nil {
		return "", err
	}
	switch d {
	case teakit.DiffInterrupt:
		return "interrupted", nil
	case teakit.DiffConfirm:
		return "apply", nil
	case teakit.DiffBack:
		return "back", nil
	}
	return "cancelled", nil
}
