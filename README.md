# teakit

Bubble Tea components with one set of key conventions, shared by my terminal tools. It is built for my own projects: the API may change at any time, and versions stay at v0.

Start with the **leave question**: embed it in a full-screen model or run it as an inline wizard step. A wizard that runs each screen as its own program can run its lists, forms, and waits through the **step runner**. See [using and updating teakit](docs/consumers.md) for integration, version selection, and update PRs. Try the [example](examples/leave/main.go) from a checkout with `go run ./examples/leave`. The [diff example](examples/diff/main.go) runs an override review as an inline step with `go run ./examples/diff`.

Tagged versions, migration notes, and CI-built source archives are on [GitHub Releases](https://github.com/undervoke/teakit/releases). Maintainers: [cutting a release](docs/releasing.md).

## Scope

teakit fixes what must behave the same in every tool, and each tool supplies what differs.

| teakit fixes | Each tool supplies |
|---|---|
| The key meanings below | Keys it adds to cancel, such as `q`, and keys of its own |
| The leave question's answers, highlighted option, and default view | When to ask (always, or only with unsaved changes), the question's wording, and its layout, styles, and key hints in the tool |
| The diff view's layout, key meanings, and confirmation guards | The diff's content and wording, when cancelling asks first, whether Back exists, and whether an extra acknowledgement gates Enter |
| The static scrollback render left after a confirmed inline diff | Whether the host prints its own summary on top of it |
| Numbered lists, cancellable waits, and text wrapping | What its screens show |
| A list's typed field: sized to its text, with the unit as a fixed suffix, and without a unit the user types after the number | The unit, the placeholder, validation, and the form in which it stores the value |
| An optional runner that runs each screen as its own program and erases the screen when the user leaves it | The flow between screens, when leaving loses something, whether to use the runner or a full-screen model, validation of its own values, and the wording of its screens and key hints |

A behavior becomes a tool-supplied setting only when two tools actually need it to differ; until then it stays fixed here.

## Keys

| Key | Meaning |
|---|---|
| Up/Down, k/j | Move between options, wrapping around at both ends (only Up/Down in a list's typed field); scroll the diff one row |
| PgUp/PgDn, Home/End, g/G | Scroll the diff a page; top/bottom (diff only, `]`/`[` jump between sections) |
| Enter | Choose the highlighted option, or move to the next field; on a complete, visible, armed diff, confirm it |
| A number | Choose that option; with more than nine options, a digit waits for a second digit or Enter. The typed field's number moves to the field instead |
| Esc on a choice screen | Leave, asking first when the tool says leaving needs confirmation |
| Esc while typing text, in a text form or a list's typed field | Return to the previous screen; with none, leave as on a choice screen |
| Esc on the leave question | Confirm leaving immediately |
| Esc in the diff view | Cancel; asks the configured leave question first when the tool set one |
| y / n in the diff view | Arm / disarm the extra acknowledgement, when the tool configured one; Enter alone never confirms while it is unarmed |
| Esc during a wait | Cancel the wait; the tool decides what follows |
| Backspace on a choice screen | Return to the previous screen; while typing text it edits the text |
| Shift+Tab | Move to the previous field; on the first field, return to the previous screen |
| Ctrl+C | Leave at once from any screen, including during a wait |

The leave question starts on the option to stay, so Enter keeps the user where they were.
