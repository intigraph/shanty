package stread

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeST installs a fake st that logs each invocation's arguments.
func fakeST(t *testing.T, body string) (calls func(args string) int) {
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
	return func(args string) int {
		b, _ := os.ReadFile(log)
		n := 0
		for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			if line == args {
				n++
			}
		}
		return n
	}
}

// The held item comes from $SHANTY_TEST_ID; any other anchor form is refused so
// a test fails if the bar falls back to the expensive prose rendering.
const jsonST = `case "$*" in
  "anchor boss --json")
    if [ -n "$SHANTY_TEST_ID" ]; then
      printf '{"schema":"st.task-context/1","agent":"boss","item":{"id":"%s","title":"Title of %s","status":"in_progress","priority":0}}\n' "$SHANTY_TEST_ID" "$SHANTY_TEST_ID"
    else
      printf '{"schema":"st.task-context/1","agent":"boss","item":null}\n'
    fi ;;
  *) echo "unexpected st call: $*" >&2; exit 9 ;;
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

// aegis-dh6: one --json read gives id, title and status; the prose anchor
// (9 owner-rows reads for an administrator) is never run.
func TestAnchorReadsIDTitleAndStatusFromOneJSONRead(t *testing.T) {
	calls := fakeST(t, jsonST)
	t.Setenv("SHANTY_TEST_ID", "aegis-1")
	p, err := Anchor("boss")
	if err != nil || p.ID != "aegis-1" || p.Title != "Title of aegis-1" || p.Status != "in_progress" {
		t.Fatalf("read %+v %v", p, err)
	}
	id, err := AnchorID("boss")
	if err != nil || id != "aegis-1" {
		t.Fatalf("AnchorID %q %v", id, err)
	}
	if calls("anchor boss --json") != 1 {
		t.Fatalf("anchor + id segments must share one read: calls=%d", calls("anchor boss --json"))
	}
}

// The P2 from the #25 review: after an item change the title must never be
// the previous item's (or a fragment of another line) while the id is new.
func TestItemChangeMovesIDAndTitleTogether(t *testing.T) {
	fakeST(t, jsonST)
	t.Setenv("SHANTY_TEST_ID", "aegis-1")
	Anchor("boss")
	t.Setenv("SHANTY_TEST_ID", "aegis-2")
	ageKey(t, []string{"anchor", "boss", "--json"}, SlowTTL+time.Second)
	p, err := Anchor("boss")
	if err != nil || p.ID != "aegis-2" || p.Title != "Title of aegis-2" {
		t.Fatalf("after item change %+v %v", p, err)
	}
}

func TestNullItemIsAnEmptyPlateAndBadJSONIsLoud(t *testing.T) {
	fakeST(t, jsonST)
	t.Setenv("SHANTY_TEST_ID", "")
	if p, err := Anchor("boss"); err != nil || !p.Empty() {
		t.Fatalf("null item %+v %v", p, err)
	}
	fakeST(t, "echo 'not json'\n")
	if _, err := Anchor("boss"); err == nil {
		t.Fatal("an unreadable answer must be an error, never an empty plate")
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
