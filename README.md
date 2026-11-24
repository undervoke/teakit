# teakit

Bubble Tea components with one set of key conventions, shared by my terminal tools. It is built for my own projects: the API may change at any time, and versions stay at v0.

Start with the **leave question**: embed it in a full-screen model or run it as an inline wizard step. A wizard that runs each screen as its own program can run its questions, forms, diffs, and waits through the **step runner**, optionally on the alternate screen so the shell's earlier text stays hidden. See [using and updating teakit](docs/consumers.md) for integration, version selection, and update PRs.

Try every component and see the code behind it. Name a component, such as `select`, to go straight to it, or `styles` to see each role's look. A last argument of `altscreen` runs it on the alternate screen.

```sh
go run github.com/undervoke/teakit/cmd/demo-tui@latest   # the latest release
go tool demo-tui                                         # this checkout
```

Tagged versions, migration notes, and CI-built source archives are on [GitHub Releases](https://github.com/undervoke/teakit/releases). Maintainers: [cutting a release](docs/releasing.md).

## Scope

teakit fixes what must behave the same in every tool, and each tool supplies what differs.

| teakit fixes | Each tool supplies |
|---|---|
| The key meanings below | Keys it adds to cancel, such as `q`, and keys of its own |
| The leave question's answers, highlighted option, and default view | When to ask (always, or only with unsaved changes), the question's wording, and its layout, styles, and key hints in the tool |
| A diff's layout, file after file, as a screen to read (`Diff`) and as text to print | The diff's content, how the user chooses what goes into it, and what going on from it does |
| Questions with numbered options that take one option (`Select`) or any number of them (`MultiSelect`), forms (`Form`) of text and single-choice fields, all of which scroll on a short terminal; cancellable waits; and text wrapping | What its screens show, and which answers it accepts |
| An input, in a select or a form: sized to its text, with its placeholder only until there is text, the unit as a fixed suffix, and without a unit the user types after the number | The unit, the placeholder, validation, and the form in which it stores the value |
| The roles a text can have (`Header`, `Subhead`, `Focus`, `Chosen`, `Muted`, `Success`, `Warning`, `Error`), each one's look, and the role of each part of its screens | Which of its own words have a role |
| When a form checks its fields: each when the user locks it, and all of them on submit | The check, which may read the other fields |
| An optional runner that runs each screen as its own program and erases the screen when the user leaves it | The flow between screens, when leaving loses something, whether to use the runner or a full-screen model, whether the flow runs on the alternate screen, validation of its own values, and the wording of its screens and key hints |

A behavior becomes a tool-supplied setting only when two tools actually need it to differ; until then it stays fixed here.

## Keys

| Key | Meaning |
|---|---|
| Up/Down, k/j | Move between options, wrapping around at both ends. Only Up/Down in a select's input |
| Up/Down in a form | Move between the fields and the Submit row, stopping at both ends; inside an open select field, move between its options |
| Up/Down, k/j in a diff | Scroll it one row |
| PgUp/PgDn, Home/End, g/G in a diff | Scroll it a page; to its top or bottom |
| Left/Right in a diff | Go to the file before or after |
| Enter | Choose the highlighted option, or answer a multi-select with the checked ones |
| Enter in a form | Open the field at the cursor; lock an open field's answer and open the next field; submit from the Submit row. A form of one field stays open, and Enter locks and submits it |
| Enter in a diff | Go on to what the tool does next, such as applying it. A held key does not |
| Typing, or a number, in a form | Open the text field, or the select field, at the cursor; the number also locks its option |
| Space | Check or uncheck the option at the cursor of a multi-select |
| A number | Choose that option; with more than nine options, a digit waits for a second digit or Enter. The number of a select's input, and any number in a multi-select, moves there instead |
| Esc on a choice screen or in a diff | Leave, asking first when the tool says leaving needs confirmation |
| Esc in a form | Undo the open field's change and close it; with none open, ask the leave question |
| Esc in a select's input or a form of one field | Return to the previous screen; with none, the same as on a choice screen or in a form |
| Esc on the leave question | Confirm leaving immediately |
| Esc during a wait | Cancel the wait; the tool decides what follows |
| Tab | In a form, move to the next field; from an open field, the same as Enter |
| Shift+Tab | Return to the previous screen; in a form, move to the previous field until the first |
| Ctrl+C | Leave at once from any screen, including during a wait and, on the alternate screen, between screens |

The leave question starts on the option to stay, so Enter keeps the user where they were.
