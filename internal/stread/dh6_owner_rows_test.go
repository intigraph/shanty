package stread

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeST installs a fake st that logs each invocation's arguments and answers
// `anchor X --short` with the id in $SHANTY_TEST_ID and `anchor X` with a plate.
func fakeST(t *testing.T, body string) (calls func(substr string) int) {
	t.Helper()
	d := t.TempDir()
	log := filepath.Join(d, "calls")
	bin := filepath.Join(d, "st")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho \"$*\" >> \"$SHANTY_TEST_CALLS\"\n"+body), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHANTY_ST_BIN", bin)
	t.Setenv("SHANTY_TEST_CALLS", log)
	t.Setenv("SHANTY_SEG_NOCACHE", "")
	t.Setenv("XDG_CACHE_HOME", d)
	ResetBinForTest()
	t.Cleanup(ResetBinForTest)
	return func(substr string) int {
		b, _ := os.ReadFile(log)
		n := 0
		for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			if line == substr {
				n++
			}
		}
		return n
	}
}

const plateST = `case "$*" in
  "anchor boss --short") echo "$SHANTY_TEST_ID" ;;
  "anchor boss") printf 'You are boss\n  ▶ %s  The held item title   (in_progress)\n' "$SHANTY_TEST_ID" ;;
esac
`

func ageKey(t *testing.T, args []string, by time.Duration) {
	t.Helper()
	p := filepath.Join(cacheDir(), cacheKey(Bin(), workDir(), args))
	old := time.Now().Add(-by)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
}

// aegis-dh6: the full anchor (9 owner-rows reads for an administrator) runs
// once per held item, not once per SlowTTL.
func TestAnchorTitleReusedWhileTheSameItemIsHeld(t *testing.T) {
	calls := fakeST(t, plateST)
	t.Setenv("SHANTY_TEST_ID", "aegis-1")
	p, err := Anchor("boss")
	if err != nil || p.ID != "aegis-1" || p.Title != "The held item title" || p.Status != "in_progress" {
		t.Fatalf("first read %+v %v", p, err)
	}
	// Both st answers expire (past SlowTTL), the item is unchanged.
	ageKey(t, []string{"anchor", "boss", "--short"}, SlowTTL+time.Second)
	ageKey(t, []string{"anchor", "boss"}, SlowTTL+time.Second)
	p, err = Anchor("boss")
	if err != nil || p.Title != "The held item title" || p.Status != "in_progress" {
		t.Fatalf("reused read %+v %v", p, err)
	}
	if calls("anchor boss --short") != 2 || calls("anchor boss") != 1 {
		t.Fatalf("short=%d full=%d, want short=2 full=1", calls("anchor boss --short"), calls("anchor boss"))
	}
	// A new item misses the title at once.
	t.Setenv("SHANTY_TEST_ID", "aegis-2")
	ageKey(t, []string{"anchor", "boss", "--short"}, SlowTTL+time.Second)
	p, _ = Anchor("boss")
	if p.ID != "aegis-2" || calls("anchor boss") != 2 {
		t.Fatalf("changed item %+v full=%d", p, calls("anchor boss"))
	}
	// Past TitleTTL the same item's title is read again.
	ageKey(t, []string{"anchor-title", "boss", "aegis-2"}, TitleTTL+time.Second)
	ageKey(t, []string{"anchor", "boss", "--short"}, SlowTTL+time.Second)
	ageKey(t, []string{"anchor", "boss"}, SlowTTL+time.Second)
	Anchor("boss")
	if calls("anchor boss") != 3 {
		t.Fatalf("title must expire at TitleTTL: full=%d", calls("anchor boss"))
	}
}

func TestAnchorIDNeverRunsTheFullPlate(t *testing.T) {
	calls := fakeST(t, plateST)
	t.Setenv("SHANTY_TEST_ID", "aegis-1")
	id, err := AnchorID("boss")
	if err != nil || id != "aegis-1" || calls("anchor boss") != 0 {
		t.Fatalf("id=%q err=%v full=%d", id, err, calls("anchor boss"))
	}
}

// aegis-dh6: segment processes that miss together send ONE st read.
func TestConcurrentMissesSingleFlight(t *testing.T) {
	calls := fakeST(t, "sleep 0.3\necho roster\n")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if out, err := RunTTL(RosterTTL, "crew"); err != nil || out != "roster" {
				t.Errorf("out=%q err=%v", out, err)
			}
		}()
	}
	wg.Wait()
	if n := calls("crew"); n != 1 {
		t.Fatalf("8 concurrent misses ran st %d times, want 1", n)
	}
}
