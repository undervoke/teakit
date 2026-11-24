package main

import (
	"context"
	_ "embed"
	"errors"
	"slices"
	"strings"

	"github.com/undervoke/teakit"
)

//go:embed multiselect.go
var multiSelectSource string

func demoMultiSelect(ctx context.Context) (string, error) {
	m := &teakit.MultiSelect{
		Title:   "Which repositories?",
		Options: []string{"example/web", "example/api", "example/docs", "example/site"},
		Checked: []bool{true, false, false, false},
		Accept: func(checked []bool) error {
			if !slices.Contains(checked, true) {
				return errors.New("Choose at least one repository.")
			}
			return nil
		},
		Hint: "↑/↓ move · space check · enter continue · shift+tab back · esc leave",
	}
	if err := (&teakit.Step{MultiSelect: m, Back: true}).Run(ctx); err != nil {
		return "", err
	}
	var picked []string
	for i, option := range m.Options {
		if m.Checked[i] {
			picked = append(picked, option)
		}
	}
	return strings.Join(picked, ", "), nil
}
