package main

import (
	"context"
	_ "embed"
	"errors"
	"strconv"

	"github.com/undervoke/teakit"
)

//go:embed select.go
var selectSource string

func demoSelect(ctx context.Context) (string, error) {
	s := &teakit.Select{
		Title:   "How much memory?",
		Options: []string{"8GiB", "16GiB", ""}, // the input takes the last one's place
		Hint:    "↑/↓ move · enter choose · backspace back · esc leave",
		Input: &teakit.Input{
			Placeholder: "another size",
			Unit:        "GiB",
			Accept: func(text string) error {
				if n, err := strconv.Atoi(text); err != nil || n < 1 {
					return errors.New("Enter a whole number, such as 12.")
				}
				return nil
			},
		},
		InputHint: "↑/↓ move · enter choose · esc back",
	}
	if err := (&teakit.Step{Select: s, Back: true}).Run(ctx); err != nil {
		return "", err
	}
	if s.Cursor == len(s.Options)-1 {
		return s.Input.Value + s.Input.Unit, nil
	}
	return s.Options[s.Cursor], nil
}
