package stread

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func inboxFixture(t *testing.T) (string, string) {
	t.Helper()
	d := t.TempDir()
	bin := filepath.Join(d, "st")
	calls := filepath.Join(d, "calls")
	script := "#!/bin/sh\necho call >> \"$SHANTY_TEST_CALLS\"\n[ -z \"$SHANTY_TEST_SLEEP\" ] || sleep \"$SHANTY_TEST_SLEEP\"\necho \"$SHANTY_TEST_COUNT\"\nexit \"${SHANTY_TEST_EXIT:-0}\"\n"
	if err := os.WriteFile(bin, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHANTY_ST_BIN", bin)
	t.Setenv("SHANTY_TEST_CALLS", calls)
	t.Setenv("SHANTY_TEST_COUNT", "7")
	t.Setenv("SHANTY_TEST_EXIT", "0")
	t.Setenv("SHANTY_TEST_SLEEP", "")
	t.Setenv("SHANTY_SEG_NOCACHE", "")
	t.Setenv("XDG_CACHE_HOME", d)
	t.Setenv("SHANTY_ST_CWD", d)
	ResetBinForTest()
	t.Cleanup(ResetBinForTest)
	return d, calls
}

func callCount(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(b), "call\n")
}

func inboxPath(t *testing.T) string {
	t.Helper()
	key, err := inboxKey(Bin(), workDir(), []string{"inbox", "--count", "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(cacheDir(), key+".inbox")
}

func TestInboxMinuteCacheExpiryAndUnknown(t *testing.T) {
	_, calls := inboxFixture(t)
	for _, want := range []string{"7", "0"} {
		t.Setenv("SHANTY_TEST_COUNT", want)
		os.Remove(inboxPath(t))
		before := callCount(t, calls)
		for i := 0; i < 12; i++ {
			got, err := RunInboxCount("fixture")
			if got != want || err != nil {
				t.Fatalf("%q %v", got, err)
			}
		}
		if callCount(t, calls) != before+1 {
			t.Fatal("12 renders did not share one refresh")
		}
	}
	p := inboxPath(t)
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	RunInboxCount("fixture")
	after, _ := os.Stat(p)
	if !fi.ModTime().Equal(after.ModTime()) {
		t.Fatal("hit extended expiry")
	}
	old := time.Now().Add(-InboxCacheTTL - time.Second)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHANTY_TEST_EXIT", "2")
	before := callCount(t, calls)
	for i := 0; i < 12; i++ {
		if _, err := RunInboxCount("fixture"); err == nil {
			t.Fatal("UNKNOWN became success")
		}
	}
	if callCount(t, calls) != before+1 {
		t.Fatal("error amplified reads")
	}
	os.Chtimes(p, old, old)
	t.Setenv("SHANTY_TEST_EXIT", "0")
	t.Setenv("SHANTY_TEST_COUNT", "9")
	if got, err := RunInboxCount("fixture"); got != "9" || err != nil {
		t.Fatal(got, err)
	}
	t.Log("12 success/zero/error renders each use one command; expiry recovers and hits never extend TTL")
}

func TestInboxScopeAndInvalidAnswers(t *testing.T) {
	d, calls := inboxFixture(t)
	RunInboxCount("fixture")
	RunInboxCount("other")
	for _, name := range []string{"SHANTY_SD_STORE", "SHANTY_SD_QUIPU", "SHANTY_SD_PREFIX", "QUIPU_SERVER", "SHANTY_ROOT", "SHANTY_ONTO_NS"} {
		t.Setenv(name, "synthetic-route")
		if _, err := RunInboxCount("fixture"); err != nil {
			t.Fatal(err)
		}
	}
	other := filepath.Join(d, "cwd")
	os.Mkdir(other, 0700)
	t.Setenv("SHANTY_ST_CWD", other)
	RunInboxCount("fixture")
	if callCount(t, calls) != 9 {
		t.Fatalf("route isolation calls=%d", callCount(t, calls))
	}
	for _, value := range []string{"", "partial", "-1", "1\n2", "18446744073709551616"} {
		t.Setenv("SHANTY_TEST_COUNT", value)
		t.Setenv("SHANTY_SEG_NOCACHE", "1")
		if _, err := RunInboxCount("fixture"); err == nil {
			t.Fatalf("invalid %q accepted", value)
		}
	}
}

func TestInboxUnavailableCacheDoesNotRead(t *testing.T) {
	d, calls := inboxFixture(t)
	bad := filepath.Join(d, "file")
	os.WriteFile(bad, []byte("not a directory"), 0600)
	t.Setenv("XDG_CACHE_HOME", bad)
	if _, err := RunInboxCount("fixture"); err == nil {
		t.Fatal("unavailable cache hidden")
	}
	if callCount(t, calls) != 0 {
		t.Fatal("unavailable cache called backend")
	}
}

func TestInboxProcessHelper(t *testing.T) {
	if os.Getenv("SHANTY_INBOX_HELPER") != "1" {
		return
	}
	_, err := RunInboxCount("fixture")
	// A competing refresh is deliberately UNKNOWN, not a test failure.
	if err != nil && !strings.Contains(err.Error(), "already in progress") {
		t.Fatal(err)
	}
}

func TestInboxCrossProcessRefresh(t *testing.T) {
	_, calls := inboxFixture(t)
	t.Setenv("SHANTY_TEST_SLEEP", "0.2")
	var cmds []*exec.Cmd
	for i := 0; i < 12; i++ {
		cmd := exec.Command(os.Args[0], "-test.run=^TestInboxProcessHelper$")
		cmd.Env = append(os.Environ(), "SHANTY_INBOX_HELPER=1")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		cmds = append(cmds, cmd)
	}
	for _, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	if callCount(t, calls) != 1 {
		t.Fatalf("simultaneous refresh calls=%d", callCount(t, calls))
	}
	if got, err := RunInboxCount("fixture"); got != "7" || err != nil {
		t.Fatal(got, err)
	}
	t.Log("12 real processes: one backend call; contending renderers remain UNKNOWN")
}

func TestInboxFutureAndCorruptEntriesRefresh(t *testing.T) {
	_, calls := inboxFixture(t)
	RunInboxCount("fixture")
	p := inboxPath(t)
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(p, future, future); err != nil {
		t.Fatal(err)
	}
	RunInboxCount("fixture")
	if callCount(t, calls) != 2 {
		t.Fatal("future entry extended freshness")
	}
	if err := os.WriteFile(p, []byte(`{"output":""}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := RunInboxCount("fixture"); got != "7" || err != nil {
		t.Fatal(got, err)
	}
	if callCount(t, calls) != 3 {
		t.Fatal("corrupt empty success accepted")
	}
	// An executable replacement invalidates the old answer even at the same path.
	fi, err := os.Stat(Bin())
	if err != nil {
		t.Fatal(err)
	}
	changed := fi.ModTime().Add(time.Second)
	if err := os.Chtimes(Bin(), changed, changed); err != nil {
		t.Fatal(err)
	}
	RunInboxCount("fixture")
	if callCount(t, calls) != 4 {
		t.Fatal("binary replacement reused old answer")
	}
}

func TestInboxLargeFailureStillBacksOff(t *testing.T) {
	_, calls := inboxFixture(t)
	t.Setenv("SHANTY_TEST_COUNT", strings.Repeat("x", 70000))
	t.Setenv("SHANTY_TEST_EXIT", "2")
	for i := 0; i < 3; i++ {
		if _, err := RunInboxCount("fixture"); err == nil {
			t.Fatal("large failure accepted")
		}
	}
	if callCount(t, calls) != 1 {
		t.Fatal("large error bypassed backoff")
	}
	fi, err := os.Stat(inboxPath(t))
	if err != nil || fi.Size() > 1024 {
		t.Fatal("unbounded cache entry", err)
	}
}
