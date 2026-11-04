package config

import "fmt"

// Keybinding represents a tmux key binding.
type Keybinding struct {
	Key     string
	Command string
	Comment string
}

// DefaultKeybindings returns byobu-compatible keybindings with ctrl-a prefix.
func DefaultKeybindings() []Keybinding {
	return []Keybinding{
		{Key: "F2", Command: "new-window", Comment: "New window"},
		{Key: "F3", Command: "previous-window", Comment: "Previous window"},
		{Key: "F4", Command: "next-window", Comment: "Next window"},
		{Key: "F5", Command: "source-file ~/.config/shanty/tmux.conf", Comment: "Reload config"},
		{Key: "F6", Command: "detach-client", Comment: "Detach"},
		{Key: "F7", Command: "copy-mode", Comment: "Scrollback mode"},
		{Key: "F8", Command: `command-prompt -I "#W" "rename-window '%%'"`, Comment: "Rename window"},
	}
}

// SessionPickerBinding renders prefix+s: a fuzzy, most-recently-used-first
// session switcher in a popup, replacing tmux's stock `choose-tree -Zs`.
//
// Two things about the shape of this line are load-bearing.
//
// The popup runs shanty by ABSOLUTE path, for the same reason the status
// segments do (see selfPath): tmux runs it from the SERVER's environment, whose
// PATH need not contain ~/.local/bin. A bare name here would make prefix+s do
// nothing at all on exactly the hosts where it is hardest to debug.
//
// And the whole thing is guarded by if-shell rather than by deciding at
// generation time whether fzf exists. The config is regenerated on attach but
// SOURCED into a long-lived server, so a generation-time decision freezes the
// answer from whichever moment shanty last ran. The if-shell asks when the key
// is pressed, so installing fzf takes effect without regenerating anything, and
// a host without it keeps working prefix+s — the stock tree — instead of a key
// that silently does nothing.
func SessionPickerBinding() string {
	return "# Session switcher: type to fuzzy-filter, most recently used first.\n" +
		"# Without fzf this falls back to tmux's stock session tree.\n" +
		"# -EE, not -E: close the popup only when pick exits 0 (a pick or Esc). An error\n" +
		"# such as \"no other sessions to switch to\" then stays on screen until a key,\n" +
		"# instead of flashing and vanishing with no explanation.\n" +
		"bind-key s if-shell 'command -v fzf >/dev/null 2>&1' " +
		"{ display-popup -EE -w 60% -h 60% '" + selfPath() + " pick' } " +
		"{ choose-tree -Zs }\n"
}

// RenderKeybindings generates tmux keybinding configuration lines.
func RenderKeybindings(bindings []Keybinding) string {
	var out string
	for _, b := range bindings {
		out += fmt.Sprintf("# %s\nbind-key -n %s %s\n\n", b.Comment, b.Key, b.Command)
	}

	// Prefix-based bindings
	out += "# Split panes\n"
	out += `bind-key | split-window -h -c "#{pane_current_path}"` + "\n"
	out += `bind-key - split-window -v -c "#{pane_current_path}"` + "\n\n"

	// Pane navigation
	out += "# Pane navigation\n"
	out += "bind-key Left select-pane -L\n"
	out += "bind-key Right select-pane -R\n"
	out += "bind-key Up select-pane -U\n"
	out += "bind-key Down select-pane -D\n\n"

	// Last window (byobu/screen convention: ctrl-a a or ctrl-a ctrl-a)
	out += "# Last window\n"
	out += "bind-key a last-window\n"
	out += "bind-key C-a last-window\n\n"

	// Session switcher (overrides tmux's default choose-tree on s)
	out += SessionPickerBinding()

	return out
}
