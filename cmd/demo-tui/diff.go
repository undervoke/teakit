package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"slices"

	"github.com/undervoke/teakit"
)

//go:embed diff.go
var diffSource string

func demoDiff(ctx context.Context) (string, error) {
	pick := &teakit.MultiSelect{
		Title: "Update what?",
		Accept: func(checked []bool) error {
			if !slices.Contains(checked, true) {
				return errors.New("Check at least one.")
			}
			return nil
		},
		Hint: "space check · enter review · shift+tab back · esc leave",
	}
	for _, e := range diffExamples {
		pick.Options = append(pick.Options, e.name)
		pick.Checked = append(pick.Checked, true)
	}
	for {
		if err := (&teakit.Step{MultiSelect: pick, Back: true}).Run(ctx); err != nil {
			return "", err
		}
		var files []teakit.DiffFile
		for i, e := range diffExamples {
			if pick.Checked[i] {
				files = append(files, e.files...)
			}
		}
		review := &teakit.Diff{
			Title: "Review the update",
			Files: files,
			Hint:  "↑/↓ scroll · ←/→ file · enter apply · shift+tab back · esc leave",
		}
		err := (&teakit.Step{Diff: review, Back: true}).Run(ctx)
		if !errors.Is(err, teakit.ErrBack) {
			return fmt.Sprintf("changed files: %d", len(files)), err
		}
	}
}
