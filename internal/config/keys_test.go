package config

import (
	"strings"
	"testing"
)

// prefix+s must be rebound, or shanty inherits tmux's stock choose-tree and the
// switcher does not exist at all.
func TestKeybindingsRebindPrefixS(t *testing.T) {
	out := RenderKeybindings(DefaultKeybindings())
	if !strings.Contains(out, "bind-key s ") {
		t.Fatalf("expected prefix+s to be rebound, got:\n%s", out)
	}
	if !strings.Contains(out, " pick'") {
		t.Errorf("expected prefix+s to run the shanty picker, got:\n%s", out)
	}
}

// Same reason the status segments use an absolute path (see selfPath): tmux runs
// this from the SERVER's environment, whose PATH need not contain ~/.local/bin. A
// bare name makes prefix+s do nothing on exactly the hosts hardest to debug.
func TestSessionPickerDoesNotDependOnPath(t *testing.T) {
	out := SessionPickerBinding()
	if strings.Contains(out, "'shanty pick'") {
		t.Errorf("picker must not rely on PATH: want an absolute shanty path, got:\n%s", out)
	}
	if !strings.Contains(out, "-EE -w") || !strings.Contains(out, "display-popup") {
		t.Errorf("expected the picker to run in a popup, got:\n%s", out)
	}
}

// Without fzf the key must still do something useful. The guard is an if-shell so
// the question is asked when the key is PRESSED, not frozen at the moment shanty
// last generated the config — the config outlives its generation in a
// long-running server.
func TestSessionPickerFallsBackToChooseTree(t *testing.T) {
	out := SessionPickerBinding()
	if !strings.Contains(out, "if-shell") {
		t.Errorf("expected a runtime fzf guard, got:\n%s", out)
	}
	if !strings.Contains(out, "command -v fzf") {
		t.Errorf("expected the guard to test for fzf, got:\n%s", out)
	}
	if !strings.Contains(out, "choose-tree -Zs") {
		t.Errorf("expected the stock session tree as the fallback, got:\n%s", out)
	}
	// The fallback has to be the FALSE branch. Both branches present but swapped
	// would pass every check above and never open the picker.
	pick := strings.Index(out, "display-popup")
	tree := strings.Index(out, "choose-tree")
	if pick < 0 || tree < 0 || pick > tree {
		t.Errorf("expected the popup as the true branch and choose-tree as the fallback, got:\n%s", out)
	}
}

// A picker error must stay readable. With -E the popup closes the instant pick
// exits, so "no other sessions to switch to" (exit 1) showed as a flash and nothing
// else: Stiwi, 2026-10-01, "it flashes and disappears". -EE closes only on exit 0,
// and pick returns 0 on a successful switch AND on Esc, so the normal paths still
// close by themselves.
func TestSessionPickerKeepsErrorsOnScreen(t *testing.T) {
	out := SessionPickerBinding()
	if !strings.Contains(out, "display-popup -EE ") {
		t.Errorf("expected display-popup -EE so a pick error stays visible, got:\n%s", out)
	}
}
