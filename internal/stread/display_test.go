package stread

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDisplayRosterReuseDoesNotExtendInteractiveFreshness(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "st")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho '  operator worker up current idle st-operator'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHANTY_ST_BIN", bin)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("SHANTY_SEG_NOCACHE", "")
	ResetBinForTest()
	t.Cleanup(ResetBinForTest)
	key := cacheKey(bin, workDir(), []string{"crew"})
	cacheSet(key, "  operator worker up current busy st-operator")
	path := filepath.Join(cacheDir(), key)
	old := time.Now().Add(-time.Minute)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	display, err := DisplayCrew()
	if err != nil || display["operator"].State != "busy" {
		t.Fatalf("display did not reuse bounded observation: %v %v", display, err)
	}
	t.Logf("60-second-old observation: display=%s (cache reused)", display["operator"].State)
	live, err := Crew()
	if err != nil || live["operator"].State != "idle" {
		t.Fatalf("interactive reader reused expired observation: %v %v", live, err)
	}
	t.Logf("same observation: interactive=%s (producer refreshed)", live["operator"].State)
	old = time.Now().Add(-RosterTTL - time.Second)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(bin); err != nil {
		t.Fatal(err)
	}
	if _, err := DisplayCrew(); err == nil {
		t.Fatal("expired display observation hid unavailable producer")
	}
	t.Log("expired observation + missing producer: error (no stale success)")
}
