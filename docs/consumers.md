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
