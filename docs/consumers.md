# Using and updating teakit

Add teakit to the Go module that builds your terminal UI. Its module path is `github.com/undervoke/teakit`; it requires Go 1.26.0 and Bubble Tea v2.1.2. Go keeps a higher Bubble Tea version if another dependency in your build requires one.

## Adopt the leave question

| Application | Integration |
|---|---|
| Between wizard steps | Call `teakit.AskLeave(ctx, cfg)` when the preceding program has returned. It runs its own inline program and clears its frame on answer. Handle the returned error, then resume on `teakit.Stay`, leave on `teakit.Leave`, or end the wizard on `teakit.Interrupt`. |
| Inside a running step or full-screen UI | Create a fresh question with `teakit.NewLeave(cfg)` and route input to it until answered. The parent program owns the terminal. Use `question.View()` for the default list and hint, or draw the question in the parent layout. |

Each tool decides **when** to open the question: every time the user asks to leave, or only if the draft has changed. The title, explanatory text, and labels belong to the tool. Start by replacing one existing prompt; the rest of the screen flow can keep using its current components. See the [runnable example](../cmd/demo-tui/leave.go).

For a native list, pass `question.Cursor()` to the same renderer as the surrounding screens: index 0 is the stay row and index 1 is the leave row. In a compact footer, use that index to show which action Enter takes in the existing hint line. With either layout, the tool can keep its wording in the renderer and supply only extra answer keys in `LeaveConfig`.

For example, inside a parent model's `Update`, store the value returned by the question so cursor moves carry through to the next key:

```go
if m.asking {
    var cmd tea.Cmd
    m.question, cmd = m.question.Update(msg)
    switch m.question.Decision() {
    case teakit.Stay:
        m.asking = false
    case teakit.Leave:
        return m.leaveScreen() // the application's normal exit from this screen
    case teakit.Interrupt:
        return m, tea.Quit
    }
    return m, cmd
}
```

## Adopt the step runner

For a wizard that runs each screen as its own program, `teakit.Step` runs one screen and erases it when the user leaves. `Run` returns `nil` when an option was picked or the form ended, `teakit.ErrBack` when the user went back, and `teakit.ErrLeave` when they left the flow.

| Screen | Integration |
|---|---|
| A question that takes one option | Run `&teakit.Step{Select: q, ...}`; after `nil`, `q.Cursor` is the option picked. For a value the options do not offer, set `q.Input = &teakit.Input{...}` with the unit, placeholder, and check, and read `q.Input.Value`, which never includes the unit. |
| A question that takes any number of options | Run `&teakit.Step{MultiSelect: q, ...}`; `q.Checked`, one entry per option, changes in place, so it also keeps the choices of a user who goes back. |
| A form | Run `&teakit.Step{Form: f, ...}` with `InputField`s, each holding the same `teakit.Input` a select takes, and `SelectField`s in `f.Fields`; each `Value` or `Choice` changes in place. Set `Choice` to -1 for a question with no answer yet, which the form does not submit. An `Accept` that reads other fields is sound, since submitting runs every one again. |
| Changes to review | Put one `DiffField` per change to decide in a form, beside any other fields; its `Apply` is the answer, and `false` skips it. When skipping is not an option, reject it in `Accept`. The tool computes the lines in `Files`, and applies what was chosen after the form is submitted. Print `teakit.RenderDiff(files, width)` to keep a change in the scrollback. |
| The leave question alone | Run a `Step` with no question or form, e.g. for an Exit row; `nil` means the user stayed. |
| A wait | `teakit.Wait(ctx, cfg, work)` shows a spinner beside the title and returns what `work` returns, `context.Canceled` on Esc, or `teakit.ErrLeave` on Ctrl+C. |

Each tool sets `Back` when a previous screen exists, `Unsaved` when leaving a screen other than a form loses something (for example, a changed draft or setup answers past the first step), and `Leave` with the question's wording and hint. Hints such as "shift+tab to go back" are the tool's wording too.

To hide the shell's earlier text while the wizard runs, wrap the flow in `teakit.AltScreen(ctx, os.Stderr, func(ctx context.Context) error { ... })` and run every screen inside with the flow's `ctx`. The flow then runs as one Bubble Tea program on the alternate screen: each runner shows its screen in that program and ignores its own program options, so pass options such as `tea.WithInput` to `AltScreen`. Leave out `tea.WithFilter` there: the program switches screens with its own messages, which the filter would see instead of what the screens receive. The program owns the terminal until the flow returns, so show work in progress with `teakit.Wait` rather than printing. Between screens the last one stays drawn but takes no keys: clearing it would make terminals such as iTerm2 keep it in the scrollback.

Ctrl+C pressed between screens, while the flow works, cancels the flow's context with `teakit.ErrLeave` as its `context.Cause`, so that work should stop on it. The next screen then answers as if Ctrl+C was pressed on it: `ErrLeave` from `Step.Run` and `Wait`, and `teakit.Interrupt` from `AskLeave`. A termination signal gives the same answers, with or without `AltScreen`.

A full-screen model embeds a step as `teakit.NewStepModel(step)`: call `SetSize` with its box, pass every message to `Update` and run every command it returns, and draw `View`. Once it is `Done`, read `Result`. Ask the terminal for key event types, as `tea.View.KeyboardEnhancements.ReportEventTypes`, so that a held key is known as one. The model can move its own cursor with `teakit.Move`, so its options wrap like a teakit select's.

## Style text by its role

teakit styles the parts it draws, so a screen's title, headings, and hints need no style of their own. Give your own text a role instead of a color, so it looks the same in every tool: `teakit.Muted.Render("not installed")`. `go run github.com/undervoke/teakit/cmd/demo-tui@latest styles` shows them all.

A role also works inside text teakit styles, over all of it or a part. `Title: teakit.Warning.Render("Delete the VM?")` makes the title yellow and keeps it bold, and the leave question's title is a warning already.

## Install or update a version

1. Choose an exact tag from [Releases](https://github.com/undervoke/teakit/releases). Read the notes between your current version and the target; a v0 minor release can change the API or key behavior. `go list -m -versions github.com/undervoke/teakit` lists published Go versions.
2. Add the import or make the migration described in the notes, then run the following from the directory containing the consuming `go.mod`. Here `v0.2.0` is an example target:

   ```sh
   go get github.com/undervoke/teakit@v0.2.0
   go mod tidy
   go list -m github.com/undervoke/teakit
   go test ./...
   ```

3. In one PR, review the calling code, `go.mod` and `go.sum`, including any change to the Go minimum or transitive versions. Exercise the affected screen in the app: open its question, press Enter to stay, reopen it, and press Esc to leave. Each app ships the update on its own release cycle. Revert the whole update PR to roll back the code and dependency versions together.

For automated update PRs, enable Dependabot's `gomod` ecosystem **in the consuming repository**. For a root module, add this entry under `updates` in `.github/dependabot.yml` (or use the whole file for a new configuration):

```yaml
version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: weekly
    allow:
      - dependency-name: github.com/undervoke/teakit
```

Set `directory` to the directory of the consuming `go.mod` for a nested module. If a Go update entry already covers that directory, use that entry and include teakit in its allow list if it has one.

The release archive is for browsing or taking a source snapshot. To verify a download, put the archive and its `SHA256SUMS` in the same directory and run `shasum -a 256 -c SHA256SUMS`. Go installations resolve the tag through the module system and record their own checksums in `go.sum`.
