package stread

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// aegis-dnw: the plate and roster reads reuse an answer for SlowTTL, every
// other read still for CacheTTL, and a failure still backs off only CacheTTL.
func TestSlowTTLReusesPlateReadsLongerThanOtherReads(t *testing.T) {
	d := t.TempDir()
	count := filepath.Join(d, "calls")
	bin := filepath.Join(d, "st")
	ok := "#!/bin/sh\necho call >> \"$SHANTY_TEST_CALLS\"\necho answer\n"
	if err := os.WriteFile(bin, []byte(ok), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHANTY_ST_BIN", bin)
	t.Setenv("SHANTY_TEST_CALLS", count)
	t.Setenv("SHANTY_SEG_NOCACHE", "")
	t.Setenv("XDG_CACHE_HOME", d)
	ResetBinForTest()
	defer ResetBinForTest()
	calls := func() int { b, _ := os.ReadFile(count); return strings.Count(string(b), "call") }
	age := func(args []string, by time.Duration) {
		p := filepath.Join(cacheDir(), cacheKey(Bin(), workDir(), args))
		old := time.Now().Add(-by)
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	slow := []string{"anchor", "fixture", "--short"}
	fast := []string{"inbox", "--count", "fixture"}
	for _, args := range [][]string{slow, fast} {
		if out, err := RunTTL(SlowTTL, args...); args[0] == "anchor" && (err != nil || out != "answer") {
			t.Fatal(out, err)
		}
	}
	if _, err := Run(fast...); err != nil {
		t.Fatal(err)
	}
	if calls() != 2 {
		t.Fatalf("first reads calls=%d", calls())
	}
	// 30s old: past CacheTTL, inside SlowTTL.
	age(slow, 30*time.Second)
	age(fast, 30*time.Second)
	if out, err := RunTTL(SlowTTL, slow...); err != nil || out != "answer" {
		t.Fatal(out, err)
	}
	if calls() != 2 {
		t.Fatalf("slow read re-ran inside SlowTTL: calls=%d", calls())
	}
	if _, err := Run(fast...); err != nil {
		t.Fatal(err)
	}
	if calls() != 3 {
		t.Fatalf("ordinary read must still expire at CacheTTL: calls=%d", calls())
	}
	// Past SlowTTL the plate read runs again.
	age(slow, SlowTTL+time.Second)
	if _, err := RunTTL(SlowTTL, slow...); err != nil {
		t.Fatal(err)
	}
	if calls() != 4 {
		t.Fatalf("slow read must expire at SlowTTL: calls=%d", calls())
	}
	// A failing plate read backs off only CacheTTL, never SlowTTL.
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho call >> \"$SHANTY_TEST_CALLS\"\necho bad >&2\nexit 2\n"), 0700); err != nil {
		t.Fatal(err)
	}
	age(slow, SlowTTL+time.Second)
	if _, err := RunTTL(SlowTTL, slow...); err == nil {
		t.Fatal("failure became success")
	}
	failure := filepath.Join(cacheDir(), cacheKey(Bin(), workDir(), slow)+".failure")
	old := time.Now().Add(-CacheTTL - time.Second)
	if err := os.Chtimes(failure, old, old); err != nil {
		t.Fatal(err)
	}
	before := calls()
	RunTTL(SlowTTL, slow...)
	if calls() != before+1 {
		t.Fatalf("failure backoff exceeded CacheTTL: calls=%d before=%d", calls(), before)
	}
}
