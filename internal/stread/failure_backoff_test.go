package stread

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFailureBackoffRemainsUnknownAndExpires(t *testing.T) {
	d := t.TempDir()
	count := filepath.Join(d, "calls")
	bin := filepath.Join(d, "st")
	fail := "#!/bin/sh\necho call >> \"$SHANTY_TEST_CALLS\"\necho partial-answer\necho board-UNKNOWN >&2\nexit 2\n"
	if err := os.WriteFile(bin, []byte(fail), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHANTY_ST_BIN", bin)
	t.Setenv("SHANTY_TEST_CALLS", count)
	t.Setenv("SHANTY_SEG_NOCACHE", "")
	t.Setenv("XDG_CACHE_HOME", d)
	ResetBinForTest()
	defer ResetBinForTest()
	args := []string{"inbox", "--count", "fixture"}
	var first string
	for i := 0; i < 3; i++ {
		out, err := Run(args...)
		if out != "partial-answer" || err == nil || !strings.Contains(err.Error(), "board-UNKNOWN") {
			t.Fatalf("out=%q err=%v", out, err)
		}
		if i == 0 {
			first = err.Error()
		} else if err.Error() != first {
			t.Fatalf("failure changed: %v", err)
		}
	}
	b, _ := os.ReadFile(count)
	if strings.Count(string(b), "call") != 1 {
		t.Fatalf("backoff calls=%q", b)
	}
	t.Log("three failed renders: subprocess calls=1; UNKNOWN and partial output preserved")
	key := cacheKey(Bin(), workDir(), args)
	failurePath := filepath.Join(cacheDir(), key+".failure")
	before, err := os.Stat(failurePath)
	if err != nil {
		t.Fatal(err)
	}
	Run(args...)
	after, _ := os.Stat(failurePath)
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("cache hit extended deadline")
	}
	// A different command must still be attempted, even while this one backs off.
	Run("inbox", "--count", "other")
	b, _ = os.ReadFile(count)
	if strings.Count(string(b), "call") != 2 {
		t.Fatalf("argv isolation calls=%q", b)
	}
	// Expiry retries and recovers immediately, without waiting in the test.
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho call >> \"$SHANTY_TEST_CALLS\"\necho recovered\n"), 0700); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-CacheTTL - time.Second)
	if err := os.Chtimes(failurePath, old, old); err != nil {
		t.Fatal(err)
	}
	out, err := Run(args...)
	if err != nil || out != "recovered" {
		t.Fatalf("recovery out=%q err=%v", out, err)
	}
	if _, err := os.Stat(failurePath); !os.IsNotExist(err) {
		t.Fatalf("failure entry survived recovery: %v", err)
	}
	out, err = Run(args...)
	if err != nil || out != "recovered" {
		t.Fatal(out, err)
	}
	b, _ = os.ReadFile(count)
	if strings.Count(string(b), "call") != 3 {
		t.Fatalf("successful cache calls=%q", b)
	}
}
