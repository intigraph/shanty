package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var knownStatusSegments = []string{"session", "crewid", "task", "events", "inbox", "crew", "usage", "stats", "harness", "cpu", "mem", "host", "clock", "load", "disk", "anchor"}

func TestPersistentStatusLayout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg, err := LoadStatusBar(path, knownStatusSegments)
	if err != nil || !reflect.DeepEqual(cfg, DefaultStatusBar()) {
		t.Fatalf("missing file must keep defaults: %v %v", cfg, err)
	}
	for _, tc := range []struct {
		name, body  string
		left, right []string
	}{
		{"custom", "[status]\nleft=['session','crewid']\nright=['mem','clock']", []string{"session", "crewid"}, []string{"mem", "clock"}},
		{"partial", "[status]\nright=['clock']", []string{"session"}, []string{"clock"}},
		{"move identity", "[status]\nleft=['crewid','session']", []string{"crewid", "session"}, DefaultStatusBar().Right},
		{"empty", "[status]\nleft=[]\nright=[]", []string{}, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadStatusBar(path, knownStatusSegments)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(cfg.Left, tc.left) || !reflect.DeepEqual(cfg.Right, tc.right) {
				t.Fatalf("got %v want %v/%v", cfg, tc.left, tc.right)
			}
		})
	}
}

func TestInvalidStatusLayoutRefuses(t *testing.T) {
	for _, body := range []string{
		"[status]\nright=['not-a-segment']", "[status]\nright=['cpu','cpu']",
		"[status]\nright=['cpu; touch marker']", "[status]\nright=['#{shell}']",
		"[status]\nright='clock'", "[status]\nright=[", "[status]\nrigth=['clock']",
		"[other]\nright=[]",
	} {
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadStatusBar(path, knownStatusSegments); err == nil {
			t.Fatalf("accepted %q", body)
		}
	}
}

func TestUnreadablePresentStatusLayoutRefuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.Symlink(filepath.Join(t.TempDir(), "absent"), path); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadStatusBar(path, knownStatusSegments); err == nil {
		t.Fatal("dangling user config silently defaulted")
	}
}

func TestCrewIDProtectedFromRightClippingWithoutMutatingConfig(t *testing.T) {
	cfg := StatusBarConfig{Left: []string{"session", "cpu"}, Right: []string{"stats", "crewid", "clock"}}
	out := RenderStatusBar(DefaultTheme(), cfg)
	var left, right string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "set-option -g status-left '") {
			left = line
		}
		if strings.HasPrefix(line, "set-option -g status-right '") {
			right = line
		}
	}
	if strings.Contains(right, " seg crewid ") || !strings.Contains(left, " seg crewid ") || strings.Index(left, " seg crewid ") > strings.Index(left, " seg session ") {
		t.Fatalf("unprotected layout: %s", out)
	}
	if !reflect.DeepEqual(cfg.Right, []string{"stats", "crewid", "clock"}) {
		t.Fatal("render mutated persistent choice")
	}
	if !strings.Contains(out, "status-left-length 60") {
		t.Fatal("missing left identity budget")
	}
}
