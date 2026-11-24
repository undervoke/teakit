// Command demo-tui shows teakit's components and the code behind each:
//
//	demo-tui [target]
//
// Without a target it lists them, to try one after another.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"

	"github.com/charmbracelet/x/term"
	"github.com/undervoke/teakit"
)

// target is what a demo shows, the demo, and the file the demo is in.
type target struct {
	name   string
	about  string
	demo   func(context.Context) (string, error)
	source string
}

var targets = []target{
	{"Select", "pick one", demoSelect, selectSource},
	{"MultiSelect", "pick any number", demoMultiSelect, multiSelectSource},
	{"Form", "fill in several fields", demoForm, formSource},
	{"Wait", "show work in progress", demoWait, waitSource},
	{"Leave", "ask before leaving", demoLeave, leaveSource},
	{"Diff", "review changes", demoDiff, diffSource},
	{"Styles", "see what each role looks like", demoStyles, stylesSource},
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "demo-tui:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	switch {
	case len(args) > 0:
		i := slices.IndexFunc(targets, func(t target) bool { return strings.EqualFold(t.name, args[0]) })
		if i < 0 {
			usage(os.Stderr)
			return fmt.Errorf("no target named %s", args[0])
		}
		return targets[i].try(ctx)
	case !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()):
		usage(os.Stdout)
		return nil
	}
	return browse(ctx)
}

// browse runs the demos the user picks until they leave the list.
func browse(ctx context.Context) error {
	list := &teakit.Select{
		Title: "teakit components",
		Hint:  "↑/↓ move · enter try · esc quit",
	}
	for _, t := range targets {
		list.Options = append(list.Options, fmt.Sprintf("%-*s  %s", nameWidth(), t.name, t.about))
	}
	for {
		err := (&teakit.Step{Select: list}).Run(ctx)
		if errors.Is(err, teakit.ErrLeave) || ctx.Err() != nil {
			return nil
		}
		if err != nil {
			return err
		}
		if err := targets[list.Cursor].try(ctx); err != nil {
			return err
		}
		fmt.Println()
	}
}

// try runs the demo, then prints what it gave and, when it was answered, the
// code it ran.
func (t target) try(ctx context.Context) error {
	answer, err := t.demo(ctx)
	if ctx.Err() != nil {
		return nil
	}
	switch {
	case errors.Is(err, teakit.ErrBack):
		answer = "went back"
	case errors.Is(err, teakit.ErrLeave):
		answer = "left"
	case errors.Is(err, context.Canceled):
		answer = "cancelled"
	case err != nil:
		return err
	}
	fmt.Printf("%s → %s\n", t.name, answer)
	if err == nil {
		fmt.Printf("\n%s", t.code())
	}
	return nil
}

// code returns the body of the demo, the one function in its file.
func (t target) code() string {
	_, fn, _ := strings.Cut(t.source, "\nfunc ")
	_, body, _ := strings.Cut(fn, "\n")
	body = strings.TrimSuffix(body, "}\n")
	var b strings.Builder
	for line := range strings.Lines(body) {
		// The body's own tab becomes the indent.
		if line, ok := strings.CutPrefix(line, "\t"); ok {
			b.WriteString("  " + strings.ReplaceAll(line, "\t", "    "))
			continue
		}
		b.WriteString(line)
	}
	return b.String()
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: demo-tui [target]")
	fmt.Fprintln(w)
	for _, t := range targets {
		fmt.Fprintf(w, "%-*s  %s\n", nameWidth(), strings.ToLower(t.name), t.about)
	}
}

func nameWidth() int {
	width := 0
	for _, t := range targets {
		width = max(width, len(t.name))
	}
	return width
}
