// Command demo-tui shows teakit's components and the code behind each:
//
//	demo-tui [target] [altscreen]
//
// Without a target it lists them, to try one after another. With altscreen
// it runs on the terminal's alternate screen.
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

// altScreen names the alternate screen in the list and as an argument.
const altScreen = "AltScreen"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err := run(ctx, os.Args[1:])
	if err != nil && !errors.Is(err, teakit.ErrLeave) && ctx.Err() == nil {
		fmt.Fprintln(os.Stderr, "demo-tui:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	last := len(args) - 1
	alt := last >= 0 && strings.EqualFold(args[last], altScreen)
	if alt {
		args = args[:last]
	}
	if len(args) > 0 {
		i := slices.IndexFunc(targets, func(t target) bool { return strings.EqualFold(t.name, args[0]) })
		if i < 0 {
			usage(os.Stderr)
			return fmt.Errorf("no target named %s", args[0])
		}
		return targets[i].try(ctx, alt)
	}
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		usage(os.Stdout)
		return nil
	}
	if alt {
		return teakit.AltScreen(ctx, os.Stdout, browseAlt)
	}
	return browse(ctx)
}

// browse runs the demos the user picks until they leave the list.
func browse(ctx context.Context) error {
	list := newList("esc quit")
	list.Options = append(list.Options, row(altScreen, "try these on the alternate screen"))
	return pick(ctx, list, func() error {
		if list.Cursor == len(targets) {
			return teakit.AltScreen(ctx, os.Stdout, browseAlt)
		}
		defer fmt.Println()
		return targets[list.Cursor].try(ctx, false)
	})
}

// browseAlt is browse on the alternate screen, where the list shows what the
// last demo gave.
func browseAlt(ctx context.Context) error {
	list := newList("esc leave")
	return pick(ctx, list, func() error {
		t := targets[list.Cursor]
		answer, _, err := t.ask(ctx)
		list.Body = func(width int) string { return teakit.Wrap(t.name+" → "+answer, width) }
		return err
	})
}

func newList(leave string) *teakit.Select {
	list := &teakit.Select{Title: "teakit components", Hint: "↑/↓ move · enter try · " + leave}
	for _, t := range targets {
		list.Options = append(list.Options, row(t.name, t.about))
	}
	return list
}

// pick calls picked for each option the user picks until they leave the list.
func pick(ctx context.Context, list *teakit.Select, picked func() error) error {
	for {
		err := (&teakit.Step{Select: list}).Run(ctx)
		if errors.Is(err, teakit.ErrLeave) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := picked(); err != nil {
			return err
		}
	}
}

// try runs the demo, then prints what it gave and, when it was answered, the
// code it ran.
func (t target) try(ctx context.Context, alt bool) error {
	var answer string
	var answered bool
	ask := func(ctx context.Context) (err error) {
		answer, answered, err = t.ask(ctx)
		return err
	}
	var err error
	if alt {
		err = teakit.AltScreen(ctx, os.Stdout, ask)
	} else {
		err = ask(ctx)
	}
	if err != nil {
		return err
	}
	fmt.Printf("%s → %s\n", t.name, answer)
	if answered {
		fmt.Printf("\n%s", t.code())
	}
	return nil
}

// ask runs the demo and returns what it gave, and whether it was answered.
func (t target) ask(ctx context.Context) (answer string, answered bool, err error) {
	answer, err = t.demo(ctx)
	switch {
	case ctx.Err() != nil:
		return "", false, context.Cause(ctx)
	case err == nil:
		return answer, true, nil
	case errors.Is(err, teakit.ErrBack):
		return "went back", false, nil
	case errors.Is(err, teakit.ErrLeave):
		return "left", false, nil
	case errors.Is(err, context.Canceled):
		return "cancelled", false, nil
	}
	return "", false, err
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
	fmt.Fprintln(w, "usage: demo-tui [target] [altscreen]")
	fmt.Fprintln(w)
	for _, t := range targets {
		fmt.Fprintln(w, row(strings.ToLower(t.name), t.about))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, row(strings.ToLower(altScreen), "run on the alternate screen"))
}

func row(name, about string) string {
	width := 0
	for _, t := range targets {
		width = max(width, len(t.name))
	}
	return fmt.Sprintf("%-*s  %s", width, name, about)
}
