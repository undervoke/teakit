package teakit

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

// diffKeys holds the effective bindings shared by input routing and help.
type diffKeys struct {
	top, bottom, next, prev        key.Binding
	enter, alternate, accept, undo key.Binding
	back, cancel, extra, interrupt key.Binding
	view                           viewport.KeyMap
}

// diffBinding uses the first key name in help and accepts all listed aliases.
func diffBinding(desc string, names ...string) key.Binding {
	return key.NewBinding(key.WithKeys(names...), key.WithHelp(names[0], desc))
}

// newDiffKeys combines viewport and diff bindings before adding host shortcuts.
func newDiffKeys(cfg DiffConfig, view viewport.KeyMap) diffKeys {
	k := diffKeys{
		top: diffBinding("top", "g", "home"), bottom: diffBinding("bottom", "G", "end"),
		next: diffBinding("next section", "]"), prev: diffBinding("previous section", "["),
		enter: diffBinding(cfg.ApplyLabel, "enter"), accept: diffBinding("accept", "y", "Y"), undo: diffBinding("undo", "n", "N"),
		back: diffBinding("back", "shift+tab"), cancel: diffBinding("cancel", "esc"), interrupt: diffBinding("exit", "ctrl+c"),
		extra: key.NewBinding(key.WithKeys(cfg.CancelKeys...)), view: view,
	}
	if cfg.AlternateKey != "" && !k.reserved(cfg.AlternateKey) {
		k.alternate = diffBinding(cfg.AlternateLabel, cfg.AlternateKey)
	}
	return k
}

// reserved reports whether a built-in key takes precedence over a host shortcut.
func (k diffKeys) reserved(name string) bool {
	for _, b := range []key.Binding{k.top, k.bottom, k.next, k.prev, k.enter, k.accept, k.undo,
		k.back, k.cancel, k.interrupt, k.view.Up, k.view.Down, k.view.PageUp, k.view.PageDown} {
		if slices.Contains(b.Keys(), name) {
			return true
		}
	}
	return false
}

// scrolls matches viewport movement and the diff's section and edge jumps.
func (k diffKeys) scrolls(msg tea.KeyPressMsg) bool {
	return key.Matches(msg, k.top, k.bottom, k.next, k.prev, k.view.Up, k.view.Down, k.view.PageUp, k.view.PageDown)
}

// diffGroupedHint combines related bindings into a single help entry.
func diffGroupedHint(desc string, bindings ...key.Binding) key.Binding {
	names := make([]string, 0, len(bindings))
	for _, b := range bindings {
		names = append(names, b.Help().Key)
	}
	return diffBinding(desc, strings.Join(names, "/"))
}

// hintLine prioritizes the current consent action and Esc as width shrinks.
func (m DiffModel) hintLine(width int, visibleData bool) string {
	k := m.keys
	var actions []key.Binding
	if visibleData {
		if m.cfg.Acknowledge != "" && !m.acked {
			actions = append(actions, k.accept)
		} else {
			actions = append(actions, k.enter, k.alternate)
		}
	}
	page := diffGroupedHint("page", k.view.PageUp, k.view.PageDown)
	move := diffGroupedHint("scroll", k.view.Up, k.view.Down)
	lastKey := func(b key.Binding) string {
		if names := b.Keys(); len(names) > 0 {
			return names[len(names)-1]
		}
		return ""
	}
	briefMove := diffBinding("scroll", lastKey(k.view.Up)+"/"+lastKey(k.view.Down))
	briefCancel := diffBinding("", k.cancel.Help().Key)
	sections := diffGroupedHint("section", k.next, k.prev)
	full := append([]key.Binding{move, page, diffGroupedHint("top/bottom", k.top, k.bottom), sections}, actions...)
	if m.cfg.Acknowledge != "" && m.acked {
		full = append(full, k.undo)
	}
	if m.cfg.Back {
		full = append(full, k.back)
	}
	full = append(full, k.cancel)
	short := append(append([]key.Binding{move, page, sections}, actions...), k.cancel)
	mini := append(append([]key.Binding{briefMove}, actions...), briefCancel)
	compact := append(append([]key.Binding{}, actions...), k.cancel)
	// Bubbles can truncate a suffix, but the consent action and an escape
	// route should survive before scroll details, optional actions or exit.
	h := help.Model{ShortSeparator: " · "}
	for _, tier := range [][]key.Binding{full, short, mini, compact} {
		if s := strings.TrimSpace(h.ShortHelpView(tier)); diffWidth(s) <= width {
			return Muted.Render(s)
		}
	}
	if len(actions) > 0 {
		if s := h.ShortHelpView([]key.Binding{actions[0], k.cancel}); diffWidth(s) <= width {
			return Muted.Render(s)
		}
	}
	return Muted.Render(h.ShortHelpView([]key.Binding{k.cancel}))
}
