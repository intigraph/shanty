package config

import (
	"strings"
	"testing"
)

func TestRenderStatusBarUsesSegmentCalls(t *testing.T) {
	theme := DefaultTheme()
	cfg := DefaultStatusBar()
	result := RenderStatusBar(theme, cfg)

	// Status bar should use shanty seg calls, not hardcoded values. The binary is
	// named by absolute path (see TestSegmentCallsDoNotDependOnPath), so assert the
	// call shape rather than the bare name.
	expected := []string{
		// The binary is an ABSOLUTE path now, so these assert the call shape
		// without the leading "#(shanty".
		//
		// session is per-agent, so it is passed #{session_name} like the other
		// per-agent segments — asking tmux for the name over a hardcoded socket
		// is what made every pane's bar read "shanty" (03b8b49). Keep the
		// #{session_name} here: it is the regression guard, not decoration.
		" seg session #{session_name})",
		" seg cpu)",
		" seg mem)",
		" seg host)",
		" seg clock)",
		"status-interval 15",
	}
	for _, e := range expected {
		if !strings.Contains(result, e) {
			t.Errorf("expected status bar to contain %q, got:\n%s", e, result)
		}
	}
}

func TestRenderStatusBarThemeColors(t *testing.T) {
	theme := DefaultTheme()
	cfg := DefaultStatusBar()
	result := RenderStatusBar(theme, cfg)

	// Status bar bg should be Dracula main bg (#282a36)
	if !strings.Contains(result, "status-style 'bg="+theme.BG) {
		t.Errorf("expected status-style bg to use theme.BG %q, got:\n%s", theme.BG, result)
	}
	if !strings.Contains(result, theme.FG) {
		t.Errorf("expected status-style to contain FG %q", theme.FG)
	}
	if !strings.Contains(result, theme.Highlight) {
		t.Errorf("expected window-status-current-style to contain Highlight %q", theme.Highlight)
	}
	// Inactive window tabs should use StatusBG (#44475a)
	if !strings.Contains(result, "window-status-style 'fg="+theme.FG+",bg="+theme.StatusBG) {
		t.Errorf("expected window-status-style to use StatusBG %q for inactive tabs", theme.StatusBG)
	}
}

func TestDefaultStatusBarSegments(t *testing.T) {
	cfg := DefaultStatusBar()

	if len(cfg.Left) != 1 || cfg.Left[0] != "session" {
		t.Errorf("expected Left=[session], got %v", cfg.Left)
	}
	// The shantytown segments lead: they are the ones that want you to act.
	// The ambient ones follow, and NONE of them may be dropped — adding an
	// integration must not silently remove a feature someone already relies on,
	// which is exactly what this test caught when `host` went missing.
	// `task` supersedes `anchor` here: it shows the same item id PLUS the title, so
	// no information left the bar. `anchor` stays in the Registry for anyone whose
	// bar names it explicitly.
	// `usage` sits next to `crew`: both are FLEET-wide facts (who is busy, how
	// much budget is left) as opposed to the per-agent ones before them, so they
	// read as a pair. It is deliberately NOT in perAgentSegments — the Claude
	// budget belongs to the account, not the pane.
	expected := []string{"crewid", "task", "events", "inbox", "crew", "usage", "stats",
		"harness", "cpu", "mem", "host", "clock"}
	if len(cfg.Right) != len(expected) {
		t.Errorf("expected Right=%v, got %v", expected, cfg.Right)
	}
	for i, e := range expected {
		if cfg.Right[i] != e {
			t.Errorf("expected Right[%d]=%q, got %q", i, e, cfg.Right[i])
		}
	}
}

// The shantytown segments must self-hide when no st binary exists, or including
// them by default would put permanently-blank gaps in every non-shantytown user's
// status bar. (With st present they must instead be LOUD about any failure — see
// the segments package tests.)
func TestShantytownSegmentsAreInTheDefaultBar(t *testing.T) {
	cfg := DefaultStatusBar()
	for _, want := range []string{"crewid", "task", "stats", "events", "inbox", "crew", "harness"} {
		found := false
		for _, got := range cfg.Right {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("shantytown segment %q missing from the default bar", want)
		}
	}
}

func TestRenderKeybindingsContainsFKeys(t *testing.T) {
	bindings := DefaultKeybindings()
	result := RenderKeybindings(bindings)

	expected := []string{
		"F2", "new-window",
		"F3", "previous-window",
		"F4", "next-window",
		"F5", "source-file ~/.config/shanty/tmux.conf",
		"F6", "detach-client",
		"F7", "copy-mode",
		"F8", "rename-window",
	}
	for _, e := range expected {
		if !strings.Contains(result, e) {
			t.Errorf("expected keybindings to contain %q, got:\n%s", e, result)
		}
	}
}

func TestRenderKeybindingsSplitPanes(t *testing.T) {
	bindings := DefaultKeybindings()
	result := RenderKeybindings(bindings)

	if !strings.Contains(result, "split-window -h") {
		t.Error("expected | to split horizontally")
	}
	if !strings.Contains(result, "split-window -v") {
		t.Error("expected - to split vertically")
	}
}

func TestRenderKeybindingsPaneNavigation(t *testing.T) {
	bindings := DefaultKeybindings()
	result := RenderKeybindings(bindings)

	for _, dir := range []string{"Left", "Right", "Up", "Down"} {
		if !strings.Contains(result, "select-pane") {
			t.Errorf("expected pane navigation for %s", dir)
		}
	}
}

func TestRenderKeybindingsLastWindow(t *testing.T) {
	bindings := DefaultKeybindings()
	result := RenderKeybindings(bindings)

	if !strings.Contains(result, "bind-key a last-window") {
		t.Error("expected bind-key a last-window (byobu/screen convention)")
	}
	if !strings.Contains(result, "bind-key C-a last-window") {
		t.Error("expected bind-key C-a last-window (ctrl-a ctrl-a)")
	}
}

func TestDefaultThemeDracula(t *testing.T) {
	theme := DefaultTheme()

	if theme.Name != "dracula" {
		t.Errorf("expected theme name 'dracula', got %q", theme.Name)
	}
	if theme.BG != "#282a36" {
		t.Errorf("expected BG '#282a36', got %q", theme.BG)
	}
	if theme.FG != "#f8f8f2" {
		t.Errorf("expected FG '#f8f8f2', got %q", theme.FG)
	}
	if theme.StatusBG != "#44475a" {
		t.Errorf("expected StatusBG '#44475a', got %q", theme.StatusBG)
	}
	if theme.Highlight != "#bd93f9" {
		t.Errorf("expected Highlight '#bd93f9', got %q", theme.Highlight)
	}
	if theme.ActiveBorder != "#50fa7b" {
		t.Errorf("expected ActiveBorder '#50fa7b', got %q", theme.ActiveBorder)
	}
}
