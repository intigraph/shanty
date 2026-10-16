package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateConfig(t *testing.T) {
	// Use a temp dir so we don't pollute the real config
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	confPath, err := GenerateConfig()
	if err != nil {
		t.Fatalf("GenerateConfig() error: %v", err)
	}

	data, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("reading generated config: %v", err)
	}
	conf := string(data)

	// Verify header
	if !strings.Contains(conf, "# Shanty") {
		t.Error("expected header comment")
	}

	// Verify prefix key
	if !strings.Contains(conf, "unbind-key C-b") {
		t.Error("expected C-b unbind")
	}
	if !strings.Contains(conf, "set-option -g prefix C-a") {
		t.Error("expected ctrl-a prefix")
	}
	// ctrl-a a and ctrl-a ctrl-a should switch to last window (byobu/screen convention)
	if !strings.Contains(conf, "bind-key a last-window") {
		t.Error("expected bind-key a last-window")
	}
	if !strings.Contains(conf, "bind-key C-a last-window") {
		t.Error("expected bind-key C-a last-window")
	}

	// Verify terminal settings
	if !strings.Contains(conf, "default-terminal 'tmux-256color'") {
		t.Error("expected tmux-256color terminal")
	}
	if !strings.Contains(conf, "xterm-256color:Tc") {
		t.Error("expected xterm-256color:Tc override")
	}
	if !strings.Contains(conf, "mouse on") {
		t.Error("expected mouse on")
	}
	if !strings.Contains(conf, "base-index 1") {
		t.Error("expected base-index 1")
	}
	if !strings.Contains(conf, "history-limit 50000") {
		t.Error("expected history-limit 50000")
	}

	// Verify theme colors (Dracula defaults)
	if !strings.Contains(conf, "#50fa7b") {
		t.Error("expected active border color #50fa7b")
	}
	if !strings.Contains(conf, "#6272a4") {
		t.Error("expected inactive border color #6272a4")
	}

	// Verify status bar uses segment calls. The binary in the call is an ABSOLUTE
	// path, not the bare name — tmux runs status commands from the server's
	// environment, whose PATH may not include shanty at all, and a bar that cannot
	// exec renders empty rather than failing. So assert the call shape, and assert
	// separately that it is absolute.
	for _, seg := range []string{"session", "cpu", "mem", "host", "clock"} {
		if !strings.Contains(conf, " seg "+seg) {
			t.Errorf("expected %s segment call, got:\n%s", seg, conf)
		}
	}
	// KEPT FROM 03b8b49, and deliberately separate from the loop above: the
	// session segment must be PASSED #{session_name}. Without it the segment asks
	// tmux for the name itself, which fails on a fleet socket and rendered the
	// literal brand "shanty" on every agent's bar. The loop only proves the call
	// exists; this proves it is fed.
	if !strings.Contains(conf, " seg session #{session_name})") {
		t.Error("expected session segment call to be passed #{session_name}")
	}
	if strings.Contains(conf, "#(shanty seg ") {
		t.Error("segment calls must not rely on PATH: want an absolute shanty path")
	}
	if !strings.Contains(conf, "#(/") {
		t.Error("expected segment calls to invoke an absolute path")
	}
	if !strings.Contains(conf, "status-interval 5") {
		t.Error("expected status-interval 5")
	}

	// Verify keybindings
	if !strings.Contains(conf, "F2") {
		t.Error("expected F2 binding")
	}
	if !strings.Contains(conf, "source-file ~/.config/shanty/tmux.conf") {
		t.Error("expected F5 to source shanty config")
	}

	// Verify pane borders
	if !strings.Contains(conf, "pane-active-border-style") {
		t.Error("expected pane-active-border-style")
	}
	if !strings.Contains(conf, "pane-border-style") {
		t.Error("expected pane-border-style")
	}
}

func TestGenerateConfigWritesFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	confPath, err := GenerateConfig()
	if err != nil {
		t.Fatalf("GenerateConfig() error: %v", err)
	}

	if !strings.HasSuffix(confPath, "shanty/tmux.conf") {
		t.Errorf("expected config path to end with shanty/tmux.conf, got %q", confPath)
	}

	info, err := os.Stat(confPath)
	if err != nil {
		t.Fatalf("stat config file: %v", err)
	}
	if info.Size() == 0 {
		t.Error("expected non-empty config file")
	}
}

func TestGenerateConfigPreservesUserSegmentsAcrossRegeneration(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir, err := configDir()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.toml")
	body := "[status]\nleft=['session','crewid']\nright=['task','cpu','clock']\n"
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	for cycle := 0; cycle < 2; cycle++ {
		generated, err := GenerateConfig()
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(generated)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), " seg stats ") {
			t.Fatal("regeneration restored stats")
		}
		if !strings.Contains(string(data), " seg crewid #{session_name})") {
			t.Fatal("lost role control")
		}
		actual, err := os.ReadFile(path)
		if err != nil || string(actual) != body {
			t.Fatal("regeneration changed user config")
		}
	}
}

func TestInvalidUserLayoutDoesNotTruncateGeneratedConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir, err := configDir()
	if err != nil {
		t.Fatal(err)
	}
	generated := filepath.Join(dir, "tmux.conf")
	if err := os.WriteFile(generated, []byte("retain existing config"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[status]\nright=['bad;command']"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := GenerateConfig(); err == nil {
		t.Fatal("invalid layout accepted")
	}
	data, err := os.ReadFile(generated)
	if err != nil || string(data) != "retain existing config" {
		t.Fatal("invalid layout truncated prior output")
	}
}
