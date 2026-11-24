package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strconv"

	"github.com/undervoke/teakit"
)

//go:embed form.go
var formSource string

func demoForm(ctx context.Context) (string, error) {
	name := &teakit.InputField{
		Title: "Name",
		Input: teakit.Input{
			Placeholder: "what jobs ask for with runs-on",
			Accept: func(text string) error {
				if text == "" {
					return errors.New("Enter a name.")
				}
				return nil
			},
		},
	}
	image := &teakit.SelectField{
		Title:   "Image",
		Options: []string{"base", "xcode", "xcode with simulators"},
		Choice:  -1, // none yet
	}
	disk := &teakit.InputField{
		Title: "Disk",
		Input: teakit.Input{
			Value:       "64",
			Placeholder: "size",
			Unit:        "GiB",
			Accept: func(text string) error {
				if n, err := strconv.Atoi(text); err != nil || n < 1 {
					return errors.New("Enter a whole number, such as 64.")
				}
				return nil
			},
		},
	}
	fallback := &teakit.InputField{
		Title: "Fallback name",
		Input: teakit.Input{
			Placeholder: "takes the jobs while this one is off",
			Accept: func(text string) error {
				if text == name.Value {
					return errors.New("Choose a name other than the first.")
				}
				return nil
			},
		},
	}
	service := &teakit.DiffField{
		Title: "Start at login",
		Files: []teakit.DiffFile{{
			Path: "~/.config/runner/config.toml",
			Sections: []teakit.DiffSection{{
				Ops: []teakit.DiffOp{
					{Kind: teakit.OpEqual, Text: "[service]"},
					{Kind: teakit.OpDelete, Text: "autostart = false"},
					{Kind: teakit.OpInsert, Text: "autostart = true"},
				},
			}},
		}},
		Hint: "enter apply · ↑/↓ scroll, then next · backspace skip · esc close",
	}
	f := &teakit.Form{
		Title:    "Add a runner",
		Fields:   []teakit.Field{name, image, disk, fallback, service},
		Hint:     "↑/↓ move · enter open or submit · shift+tab back · esc leave",
		OpenHint: "enter lock · esc undo",
	}
	if err := (&teakit.Step{Form: f, Back: true}).Run(ctx); err != nil {
		return "", err
	}
	return fmt.Sprintf("%q on %s with %s%s, then %q, at login: %t",
		name.Value, image.Options[image.Choice], disk.Value, disk.Unit, fallback.Value, service.Apply), nil
}
