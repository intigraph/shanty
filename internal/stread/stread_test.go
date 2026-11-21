package stread

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCrewReadsNameRoleStateCurrency(t *testing.T) {
	// Real `st crew` shape: name role up currency state pane. The parse must be
	// position-tolerant — shanty does not own st's column order — so each field is
	// identified by its own closed vocabulary, never by index.
	out := `
  arnold      worker         up       unknown  saturated·948k   sess-arnold
  billy       administrator  up       STALE    busy             sess-billy
  franklin    worker         up       STALE    busy+1sh         sess-franklin
  kelly       lead           up       current  idle             sess-kelly
  noise line with no verdict at all

  1 free: kelly
  3 busy: arnold, billy, franklin

  ⚠ 1 agent(s) are running settings OLDER than the file on disk: billy
`
	crew := ParseCrew(out)
	if len(crew) != 4 {
		t.Fatalf("ParseCrew got %d entries, want 4 (prose lines dropped): %+v", len(crew), crew)
	}
	// The summary line's second field ("busy:") is verdict-shaped, so a
	// verdict-only filter admitted a crew member literally named "3". Requiring a
	// role too is what keeps st's own prose out of the roster.
	for _, prose := range []string{"1", "3", "⚠", "noise"} {
		if e, ok := crew[prose]; ok {
			t.Errorf("prose token %q parsed as a crew member: %+v", prose, e)
		}
	}
	for name, want := range map[string]Entry{
		"arnold":   {Role: "worker", State: "saturated·948k", Currency: "unknown"},
		"billy":    {Role: "administrator", State: "busy", Currency: "STALE"},
		"franklin": {Role: "worker", State: "busy+1sh", Currency: "STALE"},
		"kelly":    {Role: "lead", State: "idle", Currency: "current"},
	} {
		if got := crew[name]; got != want {
			t.Errorf("crew[%q] = %+v, want %+v", name, got, want)
		}
	}
}

func TestStateWord(t *testing.T) {
	for cell, want := range map[string]string{
		"busy":           "busy",
		"idle":           "idle",
		"waiting":        "waiting",
		"saturated·948k": "saturated", // a stat rides in the cell; the verdict stands
		"busy+1sh":       "busy",
		"?":              "?",
		"":               "",
		"worker":         "", // a role is not a verdict
		"current":        "", // a currency is not a verdict
	} {
		if got := StateWord(cell); got != want {
			t.Errorf("StateWord(%q) = %q, want %q", cell, got, want)
		}
	}
}

func TestBusy(t *testing.T) {
	// Busy exists to catch a contradiction the bar must not hide: an agent st calls
	// busy whose plate reads empty is not idle, it is unreadable. saturated and
	// queued count as busy for that purpose — all three mean work is in flight.
	for cell, want := range map[string]bool{
		"busy": true, "saturated·948k": true, "queued": true,
		"idle": false, "waiting": false, "wedged": false, "?": false, "": false,
	} {
		if got := Busy(cell); got != want {
			t.Errorf("Busy(%q) = %v, want %v", cell, got, want)
		}
	}
}

func TestParseStatsReadsTheAgentRow(t *testing.T) {
	out := `st stats — last 24h
  villiers       events=412    files=17  stops=6    tokens_in=180000 tokens_out=42000
  bond           events=88     files=3   stops=1    tokens_in=9000 tokens_out=2000
  skills: writing-plans×3
  tools:  Bash×120, Read×98`
	s := parseStats(out, "villiers")
	if !s.Captured {
		t.Fatal("Captured should be true when st reported a store")
	}
	if s.Events != 412 || s.Files != 17 || s.Stops != 6 {
		t.Errorf("counts = %+v", s)
	}
	if s.TokensIn != 180000 || s.TokensOut != 42000 {
		t.Errorf("tokens = %+v", s)
	}
	if got := s.Tokens(); got != 222000 {
		t.Errorf("Tokens() = %d, want 222000", got)
	}
}

func TestParseStatsAgentWithNoActivity(t *testing.T) {
	// A store exists but holds nothing for this agent in the window. Zeros here are
	// a MEASUREMENT, so Captured stays true — unlike the no-store case, where zeros
	// would claim a crew did nothing when nobody was counting.
	s := parseStats("st stats — last 24h\n  (no activity captured in the window)", "villiers")
	if !s.Captured {
		t.Error("Captured should be true: st has a store, this agent was just quiet")
	}
	if s.Events != 0 {
		t.Errorf("events = %d, want 0", s.Events)
	}
}

func TestStatsReportsNoStoreAsAnAnswerNotAnError(t *testing.T) {
	// `st agent stats` exits NONZERO to say the capture store does not exist yet, and
	// prints the explanation on stdout. Run must therefore hand back stdout
	// alongside the error, and Stats must turn this into Captured=false with a nil
	// error — it is a real, renderable state ("nobody is counting"), not a failure.
	dir := t.TempDir()
	fake := filepath.Join(dir, "st")
	script := "#!/bin/sh\n" +
		"[ \"$1 $2 $3\" = 'agent stats villiers' ] || exit 2\n" +
		"echo 'st stats — no capture store yet (.shanty/stats.sqlite absent).'\n" +
		"exit 1\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHANTY_ST_BIN", fake)
	t.Setenv("SHANTY_SEG_NOCACHE", "1")
	ResetBinForTest()
	defer ResetBinForTest()

	s, err := Stats("villiers")
	if err != nil {
		t.Fatalf("Stats returned an error for the no-store case: %v", err)
	}
	if s.Captured {
		t.Error("Captured should be false when st has no store")
	}
}

func TestRunReportsNotInstalled(t *testing.T) {
	// An override that does not resolve is a configuration error, not a licence to
	// fall back to some other binary the operator did not name.
	t.Setenv("SHANTY_ST_BIN", filepath.Join(t.TempDir(), "absent"))
	ResetBinForTest()
	defer ResetBinForTest()

	if Installed() {
		t.Fatal("Installed() true with an unresolvable override")
	}
	if _, err := Run("crew"); !errors.Is(err, ErrNotInstalled) {
		t.Errorf("Run error = %v, want ErrNotInstalled", err)
	}
}

func TestRunSurfacesAFailure(t *testing.T) {
	// A failing st must produce an ERROR, not an empty string. Collapsing failure
	// to "" is what let a broken bar render as a blank one and read as healthy.
	dir := t.TempDir()
	fake := filepath.Join(dir, "st")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho boom >&2\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHANTY_ST_BIN", fake)
	t.Setenv("SHANTY_SEG_NOCACHE", "1")
	ResetBinForTest()
	defer ResetBinForTest()

	out, err := Run("crew")
	if err == nil {
		t.Fatalf("Run returned no error for an st that exited 3 (out=%q)", out)
	}
	if errors.Is(err, ErrNotInstalled) {
		t.Error("a failing st must not be reported as an absent one")
	}
}

func TestRunUsesTheConfiguredWorkingDirectory(t *testing.T) {
	// st resolves its tracker by walking up from its working directory, so the
	// directory it runs in changes the ANSWER. A wrong cwd yields empty output with
	// a zero exit — the silent-blank this knob exists to fix.
	dir := t.TempDir()
	fake := filepath.Join(dir, "st")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\npwd\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	wd := t.TempDir()
	t.Setenv("SHANTY_ST_BIN", fake)
	t.Setenv("SHANTY_ST_CWD", wd)
	t.Setenv("SHANTY_SEG_NOCACHE", "1")
	ResetBinForTest()
	defer ResetBinForTest()

	out, err := Run("anchor")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// macOS resolves TempDir through a symlink, so compare resolved paths.
	want, _ := filepath.EvalSymlinks(wd)
	got, _ := filepath.EvalSymlinks(out)
	if got != want {
		t.Errorf("st ran in %q, want %q", got, want)
	}
}

func TestStGetsItsOwnDirectoryOnPath(t *testing.T) {
	// st is not self-contained: it shells out to its tracker's CLI, installed
	// alongside it. Under a tmux server whose PATH lacks that directory, `st
	// anchor` failed with "No such file or directory: 'bd'" even though st itself
	// ran fine. Prepending st's own directory is what makes its companions
	// resolvable.
	dir := t.TempDir()
	bin := filepath.Join(dir, "st")
	// The stub reports whether a sibling tool installed next to it is reachable.
	if err := os.WriteFile(bin, []byte("#!/bin/sh\ncompanion\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	companion := filepath.Join(dir, "companion")
	if err := os.WriteFile(companion, []byte("#!/bin/sh\necho reachable\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "/usr/bin:/bin") // deliberately excludes dir
	t.Setenv("SHANTY_ST_BIN", bin)
	t.Setenv("SHANTY_SEG_NOCACHE", "1")
	ResetBinForTest()
	defer ResetBinForTest()

	out, err := Run("anchor")
	if err != nil {
		t.Fatalf("st could not reach a tool installed beside it: %v", err)
	}
	if out != "reachable" {
		t.Errorf("Run = %q, want %q", out, "reachable")
	}
}

func TestChildEnvDoesNotGrowPathOnRepeatedCalls(t *testing.T) {
	// The prepend has to be idempotent: a segment makes several st calls per render
	// and a PATH that gains a copy of the same directory each time is a slow leak.
	t.Setenv("PATH", "/opt/tools:/usr/bin")
	first := childEnv("/opt/tools/st")
	var got string
	for _, kv := range first {
		if name, val, ok := strings.Cut(kv, "="); ok && name == "PATH" {
			got = val
		}
	}
	if got != "/opt/tools:/usr/bin" {
		t.Errorf("PATH = %q, want it left alone when the dir already leads", got)
	}
}

func envValue(env []string, key string) (string, bool) {
	for _, kv := range env {
		if name, val, ok := strings.Cut(kv, "="); ok && name == key {
			return val, true
		}
	}
	return "", false
}

func TestChildEnvMarksStCallsBackgroundPriority(t *testing.T) {
	// aegis-3a99.1: the status bar is never critical, so its st reads declare
	// background priority and back off first under Quipu read pressure. This
	// holds for an st found by path and for a bare name with no directory.
	t.Setenv(QuipuPriorityEnv, "")
	os.Unsetenv(QuipuPriorityEnv)
	for _, bin := range []string{"/opt/tools/st", "st"} {
		if got, ok := envValue(childEnv(bin), QuipuPriorityEnv); !ok || got != "background" {
			t.Errorf("childEnv(%q) %s = %q (set %v), want background", bin, QuipuPriorityEnv, got, ok)
		}
	}
}

func TestChildEnvKeepsAnExplicitPriority(t *testing.T) {
	t.Setenv(QuipuPriorityEnv, "interactive")
	env := childEnv("/opt/tools/st")
	if got, _ := envValue(env, QuipuPriorityEnv); got != "interactive" {
		t.Errorf("%s = %q, want the operator's explicit value kept", QuipuPriorityEnv, got)
	}
	n := 0
	for _, kv := range env {
		if strings.HasPrefix(kv, QuipuPriorityEnv+"=") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d %s entries, want exactly one", n, QuipuPriorityEnv)
	}
}

func TestStChildReceivesBackgroundPriority(t *testing.T) {
	// End to end through the real command path: the st process itself sees it.
	t.Setenv("SHANTY_SEG_NOCACHE", "1")
	os.Unsetenv(QuipuPriorityEnv)
	dir := t.TempDir()
	fake := filepath.Join(dir, "st")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s' \"$QUIPU_PRIORITY\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := runCommand(fake, dir, []string{"anchor"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "background" {
		t.Errorf("st saw QUIPU_PRIORITY=%q, want background", out)
	}
}

func TestChildEnvDeclaresStatusAgent(t *testing.T) {
	// aegis-t14: status-bar reads are attributable in Quipu's request log as
	// their own caller, not as the generic session-less "shantytown" producer.
	os.Unsetenv(QuipuAgentEnv)
	if got, ok := envValue(childEnv("/opt/tools/st"), QuipuAgentEnv); !ok || got != StatusAgent {
		t.Errorf("%s = %q (set %v), want %q", QuipuAgentEnv, got, ok, StatusAgent)
	}
}

func TestChildEnvKeepsAnExplicitAgent(t *testing.T) {
	t.Setenv(QuipuAgentEnv, "hammond")
	env := childEnv("/opt/tools/st")
	n := 0
	for _, kv := range env {
		if strings.HasPrefix(kv, QuipuAgentEnv+"=") {
			n++
			if kv != QuipuAgentEnv+"=hammond" {
				t.Errorf("got %q, want the explicit agent kept", kv)
			}
		}
	}
	if n != 1 {
		t.Errorf("%d %s entries, want exactly one", n, QuipuAgentEnv)
	}
}

func TestStChildReceivesStatusAgent(t *testing.T) {
	t.Setenv("SHANTY_SEG_NOCACHE", "1")
	os.Unsetenv(QuipuAgentEnv)
	dir := t.TempDir()
	fake := filepath.Join(dir, "st")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s' \"$QUIPU_AGENT\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := runCommand(fake, dir, []string{"crew", "--count"})
	if err != nil {
		t.Fatal(err)
	}
	if out != StatusAgent {
		t.Errorf("st saw QUIPU_AGENT=%q, want %q", out, StatusAgent)
	}
}

func TestRosterTTLOutlastsSlowTTL(t *testing.T) {
	if RosterTTL < 300*1e9 || RosterTTL <= SlowTTL {
		t.Errorf("RosterTTL = %v, want >= 5m and longer than SlowTTL %v", RosterTTL, SlowTTL)
	}
}

func TestCacheIsSharedAcrossCalls(t *testing.T) {
	// Each `#(shanty seg …)` is its own process, so a process-local cache saved
	// nothing and every render paid full price for every segment. The cache is on
	// disk for that reason: a second call must not re-run st.
	dir := t.TempDir()
	fake := filepath.Join(dir, "st")
	counter := filepath.Join(dir, "calls")
	script := "#!/bin/sh\nprintf x >> " + counter + "\necho answer\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHANTY_ST_BIN", fake)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	os.Unsetenv("SHANTY_SEG_NOCACHE")
	ResetBinForTest()
	defer ResetBinForTest()

	for i := 0; i < 3; i++ {
		got, err := Run("crew", "--count")
		if err != nil {
			t.Fatalf("Run %d: %v", i, err)
		}
		if got != "answer" {
			t.Fatalf("Run %d = %q, want answer", i, got)
		}
	}
	b, err := os.ReadFile(counter)
	if err != nil {
		t.Fatalf("reading call counter: %v", err)
	}
	if len(b) != 1 {
		t.Errorf("st ran %d times across 3 calls, want 1 (the rest cached)", len(b))
	}
}

func TestCacheKeyedByArguments(t *testing.T) {
	// Two different questions must not share one answer.
	dir := t.TempDir()
	fake := filepath.Join(dir, "st")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHANTY_ST_BIN", fake)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	os.Unsetenv("SHANTY_SEG_NOCACHE")
	ResetBinForTest()
	defer ResetBinForTest()

	a, err := Run("anchor", "bond", "--short")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Run("anchor", "villiers", "--short")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Errorf("two different st calls returned the same cached answer %q", a)
	}
}

func TestParseVerdictDecodesStsNotation(t *testing.T) {
	// st packs three facts into one column. The decode must be exact, because the
	// most consequential case is the least obvious one — see MisleadinglyIdle below.
	for cell, want := range map[string]Verdict{
		"busy":           {Word: "busy", Raw: "busy"},
		"busy+1sh":       {Word: "busy", Shells: 1, Raw: "busy+1sh"},
		"busy+2sh":       {Word: "busy", Shells: 2, Raw: "busy+2sh"},
		"idle":           {Word: "idle", Raw: "idle"},
		"idle+1sh":       {Word: "idle", Shells: 1, Raw: "idle+1sh"},
		"saturated·948k": {Word: "saturated", ContextK: 948, Raw: "saturated·948k"},
		"waiting":        {Word: "waiting", Raw: "waiting"},
		"?":              {Word: "?", Raw: "?"},
	} {
		if got := ParseVerdict(cell); got != want {
			t.Errorf("ParseVerdict(%q) = %+v, want %+v", cell, got, want)
		}
	}
}

func TestMisleadinglyIdleIsTheDispatchTrap(t *testing.T) {
	// st's own comment on the +Nsh suffix: `idle+1sh` is "idle AND carrying live
	// work". An agent whose turn ended with a build or a `gh run watch` still live is
	// not finished, and "idle" is exactly the word that gets it dispatched over.
	if !ParseVerdict("idle+1sh").MisleadinglyIdle() {
		t.Error("idle+1sh must be flagged: st says free AND says work is running")
	}
	if ParseVerdict("idle").MisleadinglyIdle() {
		t.Error("plain idle is genuinely free — flagging it would cry wolf")
	}
	if ParseVerdict("busy+1sh").MisleadinglyIdle() {
		t.Error("busy+1sh is not misleading; it already reads as occupied")
	}
	// But live work is still worth knowing about on a busy agent.
	if !ParseVerdict("busy+1sh").WorkStillRunning() {
		t.Error("busy+1sh carries live work")
	}
}

func TestWhyExplainsTheEvidence(t *testing.T) {
	// The answer to "why does the bar think this?". A verdict with its evidence can be
	// trusted or argued with; a bare word can only be believed or ignored.
	for cell, want := range map[string]string{
		"busy":           "",
		"busy+1sh":       "1 shell live",
		"idle+2sh":       "2 shells live",
		"saturated·948k": "948k ctx",
		"waiting":        "blocked on a question",
		"queued":         "unsubmitted text in the box",
		"wedged":         "pane dead or stuck",
		"?":              "st could not tell",
	} {
		if got := ParseVerdict(cell).Why(); got != want {
			t.Errorf("ParseVerdict(%q).Why() = %q, want %q", cell, got, want)
		}
	}
}

// TestParseCrewReadsTheRealFleetShape is the regression test for a bar that
// showed a permanent "st?" beside panes st could describe perfectly well.
//
// Three things about st's real output had drifted past this parser at once, and
// each one alone was enough to drop every row on the floor:
//
//	a HOST column      the agent stopped being the first field
//	stacked roles      "worker,normal" is not the word "worker"
//	a "?" in TREE      matched as the WORK verdict, next column over
//
// The rows below are copied from a live `st crew` rather than composed, because
// every one of these was a thing somebody would have sworn the format did not do.
func TestParseCrewReadsTheRealFleetShape(t *testing.T) {
	out := strings.Join([]string{
		"  HOST                   AGENT       ROLE           STATE         SETTINGS TREE      WORK             POSTURE PANE",
		"  macbookair-stiwi       hammond     administrator  cycle-blocked —        ?         busy             MANUAL  st-hammond",
		"  macbookair-stiwi       lex         worker,normal  up            current  ?         idle             bypass  st-lex",
		"  vati                   arnold      escalation-target,keeper,worker down  —        ?         —                —       shanty-arnold",
	}, "\n")

	crew := ParseCrew(out)

	// THE HOST IS NOT THE AGENT. Before this, both Mac rows were stored under
	// "macbookair-stiwi" and the later one silently replaced the earlier, so a
	// lookup by agent name found nothing for either.
	if _, ok := crew["macbookair-stiwi"]; ok {
		t.Error("the HOST was stored as an agent")
	}
	for _, name := range []string{"hammond", "lex", "arnold"} {
		if _, ok := crew[name]; !ok {
			t.Errorf("agent %q was not parsed out of the row", name)
		}
	}

	// A STACKED ROLE STILL HAS A TREE POSITION. The rest of the stack is the
	// deployment's own open vocabulary and cannot be matched against anything.
	if got := crew["lex"].Role; got != "worker" {
		t.Errorf("lex's role is %q, want the tree position \"worker\"", got)
	}
	if got := crew["arnold"].Role; got != "worker" {
		t.Errorf("arnold's stacked role is %q, want \"worker\"", got)
	}

	// THE TREE COLUMN'S "?" IS NOT THE WORK VERDICT. Both Mac rows carry a "?"
	// in TREE; neither agent is in an unknown work state.
	if got := crew["hammond"].State; got != "busy" {
		t.Errorf("hammond's state is %q — the tree column's \"?\" won", got)
	}
	if got := crew["lex"].State; got != "idle" {
		t.Errorf("lex's state is %q — the tree column's \"?\" won", got)
	}
	if got := crew["lex"].Currency; got != "current" {
		t.Errorf("lex's settings currency is %q, want \"current\"", got)
	}
}

// TestParseCrewStillReadsTheOlderTwoColumnShape is the other half of the
// position-tolerance claim. shanty reads st's HUMAN output, so it meets whatever
// version of st is installed — a parse that only understands today's columns
// breaks the bar on every host that has not upgraded yet.
func TestParseCrewStillReadsTheOlderTwoColumnShape(t *testing.T) {
	crew := ParseCrew("  villiers    worker    up   current  busy    st-villiers")
	e, ok := crew["villiers"]
	if !ok {
		t.Fatal("the pre-HOST-column row no longer parses")
	}
	if e.Role != "worker" || e.State != "busy" || e.Currency != "current" {
		t.Errorf("parsed %+v from the older shape", e)
	}
}

// TestParseCrewKeepsAnHonestQuestionMark — the negative control on the fix
// above. "?" is a REAL verdict ("st could not tell"), and suppressing it would
// trade one wrong answer for another: an agent whose work state genuinely cannot
// be read must not quietly render as something knowable.
func TestParseCrewKeepsAnHonestQuestionMark(t *testing.T) {
	crew := ParseCrew(
		"  macbookair-stiwi  lex  worker,normal  up  current  ok  ?  bypass  st-lex")
	if got := crew["lex"].State; StateWord(got) != "?" {
		t.Errorf("lex's state is %q, but st really could not tell", got)
	}
}
