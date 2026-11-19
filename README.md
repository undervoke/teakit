# teakit

Bubble Tea components with one set of key conventions, shared by my terminal tools. It is built for my own projects: the API may change at any time, and versions stay at v0.

Start with the **leave question**: embed it in a full-screen model or run it as an inline wizard step. See [using and updating teakit](docs/consumers.md) for integration, version selection, and update PRs. Try the [example](examples/leave/main.go) from a checkout with `go run ./examples/leave`.

Tagged versions, migration notes, and CI-built source archives are on [GitHub Releases](https://github.com/undervoke/teakit/releases). Maintainers: [cutting a release](docs/releasing.md).

## Scope

The leave question is the first component. The following design also covers future components: teakit fixes what must behave the same in every tool, and each tool supplies what differs.

| teakit fixes | Each tool supplies |
|---|---|
| The key meanings below | Keys it adds to cancel, such as `q`, and keys of its own |
| How the leave question answers | When to ask (always, or only with unsaved changes) and the question's title, options, and body |
| Numbered lists, cancellable waits, text wrapping, and styles | What its screens show |
| An optional runner that runs each screen as its own program and erases the screen when the user leaves it | The flow between screens, whether to use the runner or a full-screen model, and validation of its own values |

A behavior becomes a tool-supplied setting only when two tools actually need it to differ; until then it stays fixed here.

## Keys

| Key | Meaning |
|---|---|
| Up/Down, k/j | Move between options |
| Enter | Choose the highlighted option, or move to the next field |
| A number | Choose that option; with more than nine options, a digit waits for a second digit or Enter |
| Esc on a choice screen | Leave, asking first when the tool says leaving needs confirmation |
| Esc in a text form | Return to the previous screen |
| Esc on the leave question | Confirm leaving immediately |
| Esc during a wait | Cancel the wait; the tool decides what follows |
| Backspace on a choice screen | Return to the previous screen; in a text field it edits the text |
| Shift+Tab | Move to the previous field; on the first field, return to the previous screen |
| Ctrl+C | Leave at once from any screen, including during a wait |

The leave question starts on the option to stay, so Enter keeps the user where they were.
