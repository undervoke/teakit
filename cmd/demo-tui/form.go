package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"github.com/undervoke/teakit"
)

//go:embed form.go
var formSource string

func demoForm(ctx context.Context) (string, error) {
	name := &teakit.InputField{
		Title: "Name",
		HelpFunc: func(text string) string {
			return "Jobs ask for it with runs-on: " + text
		},
		Value: "build",
		Accept: func(text string) error {
			if text == "" {
				return errors.New("Enter a name.")
			}
			return nil
		},
	}
	image := &teakit.SelectField{
		Title:   "Image",
		Options: []string{"base", "xcode"},
	}
	fallback := &teakit.InputField{
		Title: "Fallback name",
		Help:  "Takes the jobs while this one is off.",
		Accept: func(text string) error {
			if text == name.Value {
				return errors.New("Choose a name other than the first.")
			}
			return nil
		},
	}
	f := &teakit.Form{
		Title:  "Add a runner",
		Fields: []teakit.Field{name, image, fallback},
		Hint:   "tab next · shift+tab previous · enter continue · esc back",
	}
	if err := (&teakit.Step{Form: f, Back: true}).Run(ctx); err != nil {
		return "", err
	}
	return fmt.Sprintf("%q on %s, then %q", name.Value, image.Options[image.Cursor], fallback.Value), nil
}
