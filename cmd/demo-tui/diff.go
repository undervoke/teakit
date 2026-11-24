package main

import (
	"context"
	_ "embed"
	"strings"

	"github.com/undervoke/teakit"
)

//go:embed diff.go
var diffSource string

func demoDiff(ctx context.Context) (string, error) {
	pick := &teakit.Select{
		Title: "Which changes?",
		Hint:  "↑/↓ move · enter choose · shift+tab back · esc leave",
	}
	for _, e := range diffExamples {
		pick.Options = append(pick.Options, e.name)
	}
	if err := (&teakit.Step{Select: pick, Back: true}).Run(ctx); err != nil {
		return "", err
	}
	e := diffExamples[pick.Cursor]
	f := &teakit.Form{
		Title: e.title,
		Hint:  "↑/↓ move · enter open or submit · shift+tab back · esc leave",
	}
	for _, file := range e.files {
		f.Fields = append(f.Fields, &teakit.DiffField{
			Files:      []teakit.DiffFile{file},
			ApplyLabel: e.apply,
			SkipLabel:  e.skip,
			Accept:     e.accept,
			Hint:       "enter apply · ↑/↓ scroll, then next · backspace skip · esc close",
		})
	}
	if len(f.Fields) == 1 {
		f.Hint = "enter apply · ↑/↓ scroll · shift+tab back"
		f.Fields[0].(*teakit.DiffField).Hint = f.Hint
	}
	if err := (&teakit.Step{Form: f, Back: true}).Run(ctx); err != nil {
		return "", err
	}
	var applied []string
	for i, field := range f.Fields {
		if field.(*teakit.DiffField).Apply {
			applied = append(applied, e.files[i].Path)
		}
	}
	return "applied " + strings.Join(applied, ", "), nil
}
