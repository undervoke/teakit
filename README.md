# teakit

Bubble Tea components with one set of key conventions, shared by my terminal tools. It is built for my own projects: the API may change at any time, and versions stay at v0.

Start with the **leave question**: embed it in a full-screen model or run it as an inline wizard step. A wizard that runs each screen as its own program can run its questions, forms, and waits through the **step runner**, optionally on the alternate screen so the shell's earlier text stays hidden. See [using and updating teakit](docs/consumers.md) for integration, version selection, and update PRs.

Try every component and see the code behind it. Name a component, such as `select`, to go straight to it.

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
| The diff view's layout, key meanings, and confirmation guards | The diff's content and wording, whether a second answer sits beside Enter and on which key, when cancelling asks first, whether Back exists, and whether an extra acknowledgement gates the answers |
| The static scrollback render left after a confirmed inline diff | Whether the host prints its own summary on top of it |
| Questions with numbered options that take one option (`Select`) or any number of them (`MultiSelect`), forms (`Form`) of text and single-choice fields, all of which scroll on a short terminal; cancellable waits; and text wrapping | What its screens show, and which answers it accepts |
| A select's input: sized to its text, with the unit as a fixed suffix, and without a unit the user types after the number | The unit, the placeholder, validation, and the form in which it stores the value |
| When a form checks its fields: each when the user moves on from it, and all of them on submit | The check, which may read the other fields |
| An optional runner that runs each screen as its own program and erases the screen when the user leaves it | The flow between screens, when leaving loses something, whether to use the runner or a full-screen model, whether the flow runs on the alternate screen, validation of its own values, and the wording of its screens and key hints |

A behavior becomes a tool-supplied setting only when two tools actually need it to differ; until then it stays fixed here.

## Keys

| Key | Meaning |
|---|---|
| Up/Down, k/j | Move between options, wrapping around at both ends (only Up/Down in a select's input); scroll the diff one row |
| PgUp/PgDn, Home/End, g/G | Scroll the diff a page; top/bottom (diff only, `]`/`[` jump between sections) |
| Enter | Choose the highlighted option, answer a multi-select with the checked ones, move to the next field, or submit the form from its last; on a complete, visible, armed diff, confirm it |
| Space | Check or uncheck the option at the cursor of a multi-select |
| A number | Choose that option; with more than nine options, a digit waits for a second digit or Enter. The number of a select's input, and any number in a multi-select, moves there instead |
| Esc on a choice screen | Leave, asking first when the tool says leaving needs confirmation |
| Esc in a form or a select's input | Return to the previous screen; with none, leave as on a choice screen |
| Esc on the leave question | Confirm leaving immediately |
| Esc in the diff view | Cancel; asks the configured leave question first when the tool set one |
| The tool's second answer key in the diff view | Choose that answer under the same conditions as Enter; the diff's own keys win over it |
| y / n in the diff view | Arm / disarm the extra acknowledgement, when the tool configured one; neither Enter nor the second answer key acts while it is unarmed |
| Esc during a wait | Cancel the wait; the tool decides what follows |
| Backspace on a choice screen | Return to the previous screen; while typing text it edits the text |
| Tab | Move to the next field |
| Shift+Tab | Move to the previous field; on the first field, return to the previous screen |
| Ctrl+C | Leave at once from any screen, including during a wait and, on the alternate screen, between screens |

The leave question starts on the option to stay, so Enter keeps the user where they were.
