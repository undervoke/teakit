package main

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/undervoke/teakit"
)

//go:embed styles.go
var stylesSource string

func demoStyles(ctx context.Context) (string, error) {
	rows := []struct {
		name   string
		role   teakit.Role
		sample string
		use    string
	}{
		{"Header", teakit.Header, "Which repositories?", "a screen's title"},
		{"Subhead", teakit.Subhead, "Personal account", "a heading among options, a field's title"},
		{"Focus", teakit.Focus, "❯ 2. example/api", "the option at the cursor"},
		{"Chosen", teakit.Chosen, "> 2. xcode", "the answer locked in a field"},
		{"Muted", teakit.Muted, "enter choose · esc back", "hints and help"},
		{"Success", teakit.Success, "GitHub App verified", "what worked"},
		{"Warning", teakit.Warning, "Discard changes?", "what to check first"},
		{"Error", teakit.Error, "Enter a name.", "what failed"},
	}
	s := &teakit.Select{
		Title: "Styles",
		Body: func(int) string {
			var b strings.Builder
			for _, r := range rows {
				sample := r.role.Render(fmt.Sprintf("%-25s", r.sample))
				fmt.Fprintf(&b, "\n%-7s  %s  %s", r.name, sample, r.use)
			}
			return b.String()
		},
		Options: []string{"Back"},
		Hint:    "enter back",
	}
	return "back", (&teakit.Step{Select: s}).Run(ctx)
}
