# Using and updating teakit

Add teakit to the Go module that builds your terminal UI. Its module path is `github.com/undervoke/teakit`; the first release targets Go 1.25.0 and Bubble Tea v2.0.2. It does not require huh. Go keeps a higher Bubble Tea version if another dependency in your build requires one.

## Adopt the leave question

| Application | Integration |
|---|---|
| Between wizard steps | Call `teakit.AskLeave(ctx, cfg)` when the preceding program has returned. It runs its own inline program and clears its frame on answer. Handle the returned error, then resume on `teakit.Stay`, leave on `teakit.Leave`, or end the wizard on `teakit.Interrupt`. |
| Inside a running step or full-screen UI | Create a fresh question with `teakit.NewLeave(cfg)` and route input to it until answered. The parent program owns the terminal. Use `question.View()` for the default list and hint, or draw the question in the parent layout. |

Each tool decides **when** to open the question: every time the user asks to leave, or only if the draft has changed. The title, explanatory text, and labels belong to the tool. Start by replacing one existing prompt; the rest of the screen flow can keep using its current components. See the [runnable example](../examples/leave/main.go).

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

## Adopt the diff view

| Application | Integration |
|---|---|
| On a wizard review step | When the plan has overrides, change the Apply button to open a diff stage hosting `teakit.NewDiff(cfg)`: route every key to the diff model on that stage and call `SetSize` with the box below the title bar. The diff's Enter is the single final submission; Backspace answers `teakit.DiffBack` only when the config enables it, which returns to the review step. |
| As an inline confirmation | Call `teakit.ReviewDiff(ctx, cfg, files, tea.WithOutput(stderr))` after the preceding program has returned. On `teakit.DiffConfirm` the whole static diff stays in the scrollback above the frame, so the host needs no summary of its own; on any other answer the frame clears and the host prints the outcome, e.g. `Cancelled; nothing was changed.` Handle the returned error, then act on `teakit.DiffConfirm` or `teakit.DiffAlternate`, or resume the previous screen otherwise. |

Each tool decides **when** cancelling asks the leave question (pass `Leave` only when the selection would be lost), whether Back exists, whether an extra acknowledgement gates the answers (pass `Acknowledge`, e.g. for irreversible data loss), and whether a second answer sits beside Enter. For the second answer, set `AlternateKey` and `AlternateLabel`, e.g. `c` and `keep current`: the key answers `teakit.DiffAlternate` under the same guards as Enter, the acknowledgement included. The diff's own keys win, so choose one it does not bind; `k`, for example, scrolls. The host checks for a TTY before an inline run; `ReviewDiff` does not decide interactivity. The model never quits the program: the owner reads `Decision()` and navigates. See the [runnable example](../examples/diff/main.go).

## Adopt the step runner

For a wizard that runs each screen as its own program, `teakit.Step` runs one screen and erases it when the user leaves. `Run` returns `nil` when an option was picked or the form ended, `teakit.ErrBack` when the user went back, and `teakit.ErrLeave` when they left the flow.

| Screen | Integration |
|---|---|
| A list | Run `&teakit.Step{List: l, ...}`; after `nil`, `l.Cursor` is the option picked. For a value the options do not offer, set `l.Field = teakit.NewField(cfg)` with the unit, placeholder, and check, and read `l.Field.Value()`, which never includes the unit. |
| A form | Pass the form through a small adapter implementing `teakit.Form`, and set `Text` when it is for typing text. teakit does not import huh. |
| The leave question alone | Run a `Step` with neither `List` nor `Form`, e.g. for an Exit row; `nil` means the user stayed. |
| A wait | `teakit.Wait(ctx, view, work)` returns what `work` returns, `context.Canceled` on Esc, or `teakit.ErrLeave` on Ctrl+C. |

Each tool sets `Back` when a previous screen exists, `Unsaved` when leaving loses something (for example, a changed draft or setup answers past the first step), and `Leave` with the question's wording and hint. Hints such as "esc to go back" are the tool's wording too. For a huh form, the adapter is:

```go
type huhForm struct {
    *huh.Form
    first huh.Field // Shift+Tab here goes back
}

func (f huhForm) Update(msg tea.Msg) tea.Cmd {
    _, cmd := f.Form.Update(msg)
    return cmd
}

func (f huhForm) Done() bool    { return f.State != huh.StateNormal }
func (f huhForm) AtFirst() bool { return f.GetFocusedField() == f.first }
```

To hide the shell's earlier text while the wizard runs, wrap the flow in `teakit.AltScreen(ctx, os.Stderr, func(ctx context.Context) error { ... })`, run every screen inside with the flow's `ctx`, and give each program the same writer with `tea.WithOutput(os.Stderr)`. The programs still draw inline, on the alternate screen, and everything printed there is gone when the flow returns, including the diff a confirmed `ReviewDiff` leaves; print `teakit.RenderDiff(files, width)` after `AltScreen` returns to keep it.

Between screens, Ctrl+C arrives as a signal rather than a key. `AltScreen` catches it, and the next screen answers as if Ctrl+C was pressed on it: `ErrLeave` from `Step.Run` and `Wait`, `teakit.Interrupt` from `AskLeave`, and `teakit.DiffInterrupt` from `ReviewDiff`. Other work in the flow sees the context cancelled, with `teakit.ErrLeave` as its `context.Cause`. Only the first signal is caught, so a second Ctrl+C before the flow returns gets the process's default handling, which exits without leaving the alternate screen.

A full-screen model can move its cursor with `teakit.Move`, so its options wrap like teakit lists.

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
