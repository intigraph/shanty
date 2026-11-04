package segments

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/intigraph/shanty/internal/stread"

	"github.com/intigraph/shanty/internal/crewid"
)

// fakeST installs a stub `st` that answers from the given canned replies, keyed by
// the joined argument list. Anything unlisted exits nonzero, so a segment reaching
// for a call the test did not anticipate shows up as a failure instead of quietly
// passing.
//
// The renderings below are the acceptance criteria for this bar: a busy agent, an
// idle agent, and an agent whose plate cannot be read must produce three DISTINCT
// outputs and none of them may be blank. Two of the three can be observed on a
// live fleet; a genuinely idle agent with an empty plate cannot be conjured on
// demand, which is why all three are pinned here.
func fakeST(t *testing.T, replies map[string]string) {
	t.Helper()
	dir := t.TempDir()
	var sb strings.Builder
	sb.WriteString("#!/bin/sh\ncase \"$*\" in\n")
	for args, out := range replies {
		fmt.Fprintf(&sb, "  %q) cat <<'REPLY'\n%s\nREPLY\n  ;;\n", args, out)
	}
	sb.WriteString("  *) echo \"unexpected st call: $*\" >&2; exit 9 ;;\nesac\n")

	bin := filepath.Join(dir, "st")
	if err := os.WriteFile(bin, []byte(sb.String()), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHANTY_ST_BIN", bin)
	t.Setenv("SHANTY_SEG_NOCACHE", "1")
	t.Setenv("SHANTY_AGENT", "villiers")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // never touch the real mark registry
	stread.ResetBinForTest()
	t.Cleanup(stread.ResetBinForTest)
}

func harnessRoot(t *testing.T, defaultName string) {
	t.Helper()
	root := t.TempDir()
	data := []byte("[harness]\ndefault = \"" + defaultName + "\"\n")
	if err := os.WriteFile(filepath.Join(root, "shantytown.toml"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHANTY_ROOT", root)
}

func TestHarnessHidesDeploymentDefault(t *testing.T) {
	fakeST(t, map[string]string{"anchor villiers --harness": "codex"})
	harnessRoot(t, "codex")
	if got := (Harness{}).Render(); got != "" {
		t.Errorf("default harness rendered %q, want silent", plain(got))
	}
}

func TestHarnessNamesDeviationWithoutTimerGlyph(t *testing.T) {
	fakeST(t, map[string]string{"anchor villiers --harness": "claude"})
	harnessRoot(t, "codex")
	got := plain((Harness{}).Render())
	if got != "agent claude" {
		t.Errorf("harness deviation rendered %q, want %q", got, "agent claude")
	}
	if strings.Contains(got, "⏱") {
		t.Errorf("harness deviation still looks like a timer: %q", got)
	}
}

func TestHarnessIsLoudWhenDefaultCannotBeRead(t *testing.T) {
	fakeST(t, map[string]string{"anchor villiers --harness": "claude"})
	t.Setenv("SHANTY_ROOT", t.TempDir())
	got := plain((Harness{}).Render())
	if got != "⚠ harness?" {
		t.Errorf("unreadable harness config rendered %q, want loud failure", got)
	}
}

// plain strips tmux colour codes so assertions read as what an operator sees.
func plain(s string) string {
	var out strings.Builder
	for {
		i := strings.Index(s, "#[")
		if i < 0 {
			out.WriteString(s)
			return out.String()
		}
		out.WriteString(s[:i])
		j := strings.Index(s[i:], "]")
		if j < 0 {
			return out.String()
		}
		s = s[i+j+1:]
	}
}

const busyCrew = "  villiers    worker    up   current  busy    st-villiers"

func TestTaskRendersAHeldItem(t *testing.T) {
	fakeST(t, map[string]string{
		"anchor villiers --short": "ss-5x22f",
		"anchor villiers": "  You are villiers — worker, reports to moneypenny.\n" +
			"\n  ON YOUR PLATE\n" +
			"    ▶ ss-5x22f  status display: missing stats and crew name        (in_progress)\n",
		"crew": busyCrew,
	})
	got := plain(Task{}.Render())
	if !strings.Contains(got, "ss-5x22f") {
		t.Errorf("render %q is missing the item id", got)
	}
	if !strings.Contains(got, "status display") {
		t.Errorf("render %q is missing the task summary", got)
	}
	if strings.Contains(got, "⚠") {
		t.Errorf("render %q warns about a plate it read fine", got)
	}
}

func TestTaskRendersAnIdleAgentWithAnEmptyPlate(t *testing.T) {
	// st legitimately names no item and calls the agent idle. That is an ANSWER, so
	// it gets words — an empty segment here would be indistinguishable from the bar
	// being broken.
	fakeST(t, map[string]string{
		"anchor villiers --short": "",
		"crew":                    "  villiers    worker    up   current  idle    st-villiers",
	})
	got := plain(Task{}.Render())
	if got == "" {
		t.Fatal("an idle agent rendered a blank segment")
	}
	if strings.Contains(got, "⚠") {
		t.Errorf("render %q warns about a plate that is simply empty", got)
	}
	if !strings.Contains(got, "nothing held") {
		t.Errorf("render %q does not say the plate is empty", got)
	}
}

func TestTaskIsLoudWhenABusyAgentsPlateCannotBeRead(t *testing.T) {
	// THE bug this bar was rebuilt for. `st anchor --short` answers a lookup it
	// cannot resolve with empty output and a ZERO exit, so a naive bar renders
	// blank — consistently, silently, and looking exactly like an idle agent.
	// Cross-checking st's own verdict turns it into a visible contradiction.
	fakeST(t, map[string]string{
		"anchor villiers --short": "",
		"crew":                    busyCrew,
	})
	got := plain(Task{}.Render())
	if !strings.Contains(got, "⚠") {
		t.Errorf("render %q must warn: st calls this agent busy but names no item", got)
	}
	if strings.Contains(got, "nothing held") {
		t.Errorf("render %q claims an empty plate for a busy agent", got)
	}
}

func TestTheThreePlateRenderingsAreDistinct(t *testing.T) {
	// The acceptance criterion, stated as a test: three states, three renderings,
	// no blanks.
	renders := map[string]string{}
	t.Run("held", func(t *testing.T) {
		fakeST(t, map[string]string{
			"anchor villiers --short": "ss-1",
			"anchor villiers":         "  ON YOUR PLATE\n    ▶ ss-1  a real item        (open)\n",
			"crew":                    busyCrew,
		})
		renders["held"] = plain(Task{}.Render())
	})
	t.Run("idle", func(t *testing.T) {
		fakeST(t, map[string]string{
			"anchor villiers --short": "",
			"crew":                    "  villiers    worker    up   current  idle    st-villiers",
		})
		renders["idle"] = plain(Task{}.Render())
	})
	t.Run("unreadable", func(t *testing.T) {
		fakeST(t, map[string]string{
			"anchor villiers --short": "",
			"crew":                    busyCrew,
		})
		renders["unreadable"] = plain(Task{}.Render())
	})

	seen := map[string]string{}
	for state, out := range renders {
		if strings.TrimSpace(out) == "" {
			t.Errorf("state %q rendered blank", state)
		}
		if other, dup := seen[out]; dup {
			t.Errorf("states %q and %q render identically as %q", other, state, out)
		}
		seen[out] = state
	}
}

func TestCrewIDShowsRoleAndStateAndNotTheNameAgain(t *testing.T) {
	// THE DEDUPLICATION. The session pill at the other end of the bar already
	// renders `<mark> <agent>`; this segment used to open with the same two
	// things, so a pane read "🦊 villiers … 🦊 villiers wkr busy" — the name and
	// the mark twice, spending the scarcest thing a status bar has to repeat what
	// the eye had already taken in.
	fakeST(t, map[string]string{"crew": busyCrew})
	got := plain(CrewID{}.Render())
	for _, want := range []string{"wkr", "busy"} {
		if !strings.Contains(got, want) {
			t.Errorf("render %q is missing %q", got, want)
		}
	}
	if strings.Contains(got, "villiers") {
		t.Errorf("render %q repeats the agent name, which the session pill "+
			"already shows at the other end of the bar", got)
	}
}

func TestCrewIDStillAssignsAMarkOnFirstSight(t *testing.T) {
	// The mark left the RENDER, not the segment. Assignment is a persistent side
	// effect the session pill depends on, and first sight of a pane can happen
	// through either segment — so dropping the assignment with the display would
	// have left whichever segment ran second to mint the mark, silently making
	// which one you looked at first matter.
	fakeST(t, map[string]string{"crew": busyCrew})
	_ = CrewID{}.Render()
	if crewid.EmojiFor("villiers") == "" {
		t.Error("no mark was assigned on first sight, so the session pill has " +
			"none to show")
	}
}

func TestCrewIDFlagsStaleSettings(t *testing.T) {
	// st reports the agent is running settings older than the file on disk. The
	// pane looks healthy and its hooks are whatever the file said at launch, so
	// this is exactly the kind of thing a bar must not swallow.
	fakeST(t, map[string]string{
		"crew": "  villiers    worker    up   STALE  busy    st-villiers",
	})
	got := plain(CrewID{}.Render())
	if !strings.Contains(got, "STALE") {
		t.Errorf("render %q hides that the agent's settings are stale", got)
	}
}

func TestCrewIDSaysOnlyThatStateIsUnknowable(t *testing.T) {
	// st answered, but not about us. The identity half is still true and still
	// useful; dropping the whole segment would throw it away.
	fakeST(t, map[string]string{
		"crew": "  bond    worker    up   current  busy    st-bond",
	})
	got := plain(CrewID{}.Render())
	if !strings.Contains(got, "⚠") {
		t.Errorf("render %q does not flag the missing crew state", got)
	}
	// The identity is NOT repeated here even on this path. It is on the session
	// pill, which does not depend on st answering at all — so the only thing this
	// segment has left to contribute when st is silent is that st was silent.
	if strings.Contains(got, "villiers") {
		t.Errorf("render %q repeats the agent name the session pill shows", got)
	}
}

func TestStatsRendersTheNumbers(t *testing.T) {
	fakeST(t, map[string]string{
		"agent stats villiers": "st stats — last 24h\n" +
			"  villiers    events=412  files=17  stops=6  tokens_in=180000 tokens_out=42000" +
			" usage_known=1 usage_in=180000 usage_out=42000 cache_read=90000 usage=claude_tokens=222000",
	})
	got := plain(Stats{}.Render())
	for _, want := range []string{"412", "17f", "in180k", "out42k", "cache50%"} {
		if !strings.Contains(got, want) {
			t.Errorf("render %q is missing %q", got, want)
		}
	}
}

func TestStatsFallsBackToLegacySplitBeforeUsageWireUpgrade(t *testing.T) {
	fakeST(t, map[string]string{
		"agent stats villiers": "st stats — last 24h\n" +
			"  villiers events=2 files=0 stops=1 tokens_in=12000 tokens_out=3400",
	})
	got := plain(Stats{}.Render())
	for _, want := range []string{"in12k", "out3.4k"} {
		if !strings.Contains(got, want) {
			t.Errorf("legacy render %q is missing %q", got, want)
		}
	}
	if strings.Contains(got, "cache") {
		t.Errorf("legacy render %q invented cache data", got)
	}
}

func TestStatsSaysOffRatherThanZeroWhenNobodyIsCounting(t *testing.T) {
	// `st stats` is wired on every deployment but its numbers come from a local
	// capture store that only exists once the harness hooks feeding it are
	// installed. Rendering zeros would claim a crew did nothing when in fact
	// nobody was counting — and it is the live state of this deployment today.
	dir := t.TempDir()
	bin := filepath.Join(dir, "st")
	script := "#!/bin/sh\n" +
		"echo 'st stats — no capture store yet (.shanty/stats.sqlite absent).'\nexit 1\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHANTY_ST_BIN", bin)
	t.Setenv("SHANTY_SEG_NOCACHE", "1")
	t.Setenv("SHANTY_AGENT", "villiers")
	stread.ResetBinForTest()
	t.Cleanup(stread.ResetBinForTest)

	got := plain(Stats{}.Render())
	if !strings.Contains(got, "off") {
		t.Errorf("render %q should say the counting is off, not show numbers", got)
	}
	if strings.Contains(got, "0⚡") {
		t.Errorf("render %q reports zero activity when nothing is being measured", got)
	}
}

func TestReadsAreNonMarking(t *testing.T) {
	// The bar polls every few seconds and must never consume state. st documents
	// `inbox --count` and `anchor --events` as reads that mark nothing; the bar has
	// to use exactly those, never the listing or draining forms.
	calls := filepath.Join(t.TempDir(), "calls")
	dir := t.TempDir()
	bin := filepath.Join(dir, "st")
	script := "#!/bin/sh\necho \"$*\" >> " + calls + "\necho 0\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHANTY_ST_BIN", bin)
	t.Setenv("SHANTY_SEG_NOCACHE", "1")
	t.Setenv("SHANTY_AGENT", "villiers")
	stread.ResetBinForTest()
	t.Cleanup(stread.ResetBinForTest)

	Events{}.Render()
	Inbox{}.Render()

	b, err := os.ReadFile(calls)
	if err != nil {
		t.Fatalf("reading call log: %v", err)
	}
	log := string(b)
	if !strings.Contains(log, "anchor villiers --events") {
		t.Errorf("events segment did not use the non-marking --events read: %q", log)
	}
	if !strings.Contains(log, "inbox --count villiers") {
		t.Errorf("inbox segment did not use the non-marking --count read: %q", log)
	}
	for _, forbidden := range []string{"--read", "drain"} {
		if strings.Contains(log, forbidden) {
			t.Errorf("the bar called a state-consuming form %q: %q", forbidden, log)
		}
	}
}

func TestAdministratorHoldingNothingIsNotAFault(t *testing.T) {
	// The administrator role exists to stay free to coordinate and is never assigned
	// implementation work, so its empty plate is the EXPECTED state. Warning about it
	// produces a red that is known-false every time it is drawn, and a status surface
	// carrying one permanent false red teaches its reader to discount every red on it.
	fakeST(t, map[string]string{
		"anchor moneypenny --short": "",
		"crew": "  moneypenny  administrator  up  current  busy  st-moneypenny\n" +
			"  bond        worker         up  current  idle  st-bond\n" +
			"  q           worker         up  current  busy  st-q",
	})
	t.Setenv("SHANTY_AGENT", "moneypenny")

	got := plain(Task{}.Render())
	if strings.Contains(got, "⚠") {
		t.Errorf("render %q warns about an administrator's empty plate", got)
	}
	if strings.Contains(got, "no item") {
		t.Errorf("render %q reports an absence for a role that never holds one: %q", got, got)
	}
	// It must say something TRUE instead of nothing — the slot is not wasted.
	if strings.TrimSpace(got) == "" {
		t.Error("administrator render is blank; it should show dispatch state")
	}
	if !strings.Contains(got, "free") {
		t.Errorf("render %q does not report the dispatch state (1 agent is idle)", got)
	}
}

func TestAdministratorRuleIsKeyedOnRoleNotName(t *testing.T) {
	// A future administrator must inherit the treatment, and the current one must not
	// keep it if the role moves. Same agent NAME, different role, opposite rendering.
	for _, tc := range []struct {
		role      string
		wantWarn  bool
		situation string
	}{
		{"administrator", false, "coordinating role: empty plate is expected"},
		{"worker", true, "worker with no item while busy: real signal, stay loud"},
	} {
		fakeST(t, map[string]string{
			"anchor moneypenny --short": "",
			"crew":                      "  moneypenny  " + tc.role + "  up  current  busy  st-moneypenny",
		})
		t.Setenv("SHANTY_AGENT", "moneypenny")
		got := plain(Task{}.Render())
		if warned := strings.Contains(got, "⚠"); warned != tc.wantWarn {
			t.Errorf("role %q (%s): warned=%v, want %v — render was %q",
				tc.role, tc.situation, warned, tc.wantWarn, got)
		}
	}
}

func TestAdministratorSummaryCountsIdleWithLiveWorkSeparately(t *testing.T) {
	// An agent st calls idle whose background work is still running is NOT free, and
	// counting it as free is the single number that most often makes a dispatch
	// decision wrong.
	fakeST(t, map[string]string{
		"anchor moneypenny --short": "",
		"crew": "  moneypenny  administrator  up  current  busy      st-moneypenny\n" +
			"  bond        worker         up  current  idle      st-bond\n" +
			"  q           worker         up  current  idle+1sh  st-q\n" +
			"  r           lead           up  current  waiting   st-r",
	})
	t.Setenv("SHANTY_AGENT", "moneypenny")
	got := plain(Task{}.Render())
	if !strings.Contains(got, "1 free") {
		t.Errorf("render %q: only bond is genuinely free", got)
	}
	if !strings.Contains(got, "1 idle+live") {
		t.Errorf("render %q: q is idle with a live shell and must not count as free", got)
	}
	if !strings.Contains(got, "1 need eyes") {
		t.Errorf("render %q: r is waiting and needs a human", got)
	}
}

func TestCrewIDShowsWhyItThinksSo(t *testing.T) {
	// The coordinator's ask: expose WHY the bar believes an agent is busy or idle, so
	// the verdict can be weighed against a competing idle signal rather than just
	// believed or ignored.
	fakeST(t, map[string]string{"crew": "  villiers  worker  up  current  busy+2sh  st-villiers"})
	got := plain(CrewID{}.Render())
	if !strings.Contains(got, "2 shells live") {
		t.Errorf("render %q does not explain the verdict", got)
	}
}

func TestCrewIDDoesNotPaintIdleWithLiveWorkAsCalm(t *testing.T) {
	fakeST(t, map[string]string{"crew": "  villiers  worker  up  current  idle+1sh  st-villiers"})
	got := CrewID{}.Render()
	if strings.Contains(got, colGreen) {
		t.Errorf("idle-with-live-work rendered in the calm green of a free agent: %q", got)
	}
	if !strings.Contains(plain(got), "1 shell live") {
		t.Errorf("render %q hides that work is still running", plain(got))
	}
}

// TestCrewIDReadsACycleBlockedAdministratorsRealVerdict is the regression test
// for aegis-64q1m0: on macbookair-stiwi, `SHANTY_AGENT=hammond shanty seg crewid
// st-hammond` rendered `admin ? (st could not tell)` while `st crew` showed
// hammond's WORK cell as `busy+1sh`.
//
// The row below is copied verbatim from the bead's own repro (STATE=cycle-blocked,
// TREE=?, WORK=busy+1sh, POSTURE=?) rather than composed, because the whole point
// is to pin the real shape st sends, TREE-`?` and POSTURE-`?` both included.
//
// `cycle-blocked` is st's STATE column — a request-level disposition (the agent's
// cycle was refused; shantytown cli.py ~6072) — and it is NOT a WORK verdict: a
// cycle-blocked agent's pane is up by construction, so st still captures and
// reports a real WORK cell (busy/idle/…) alongside it. StateRank/StateWord
// classify the WORK cell only, and Entry does not even retain the STATE column, so
// `cycle-blocked` reaching this parser is inert by design — it must not need an
// entry in StateRank to be ignored correctly. What must NOT happen is TREE's or
// POSTURE's bare `?` winning over the real WORK cell two fields to its right, and
// TestParseCrewReadsTheRealFleetShape (in stread) already guards the TREE-`?`
// case with a `busy` WORK cell; this test guards the same row end-to-end through
// CrewID.Render(), with a `busy+1sh` WORK cell and a `?` POSTURE on top.
func TestCrewIDReadsACycleBlockedAdministratorsRealVerdict(t *testing.T) {
	fakeST(t, map[string]string{
		"crew": "  macbookair-stiwi       hammond     administrator  cycle-blocked —        ?         busy+1sh         ?       st-hammond",
	})
	t.Setenv("SHANTY_AGENT", "hammond")
	got := plain(CrewID{}.Render())
	if strings.Contains(got, "st could not tell") {
		t.Errorf("render %q read the TREE or POSTURE column's bare ? as the WORK verdict", got)
	}
	if !strings.Contains(got, "busy") {
		t.Errorf("render %q lost hammond's real WORK verdict busy+1sh", got)
	}
	if !strings.Contains(got, "1 shell live") {
		t.Errorf("render %q dropped the live-shell evidence behind busy+1sh", got)
	}
}

// TestCrewIDReadsTheLiveMacHammondRow pins the exact row captured live from `st
// crew` on macbookair-stiwi on 2026-09-24 while verifying aegis-64q1m0 — a second
// real-world shape (WORK=idle, POSTURE=MANUAL) alongside the bead's own repro
// above, since a parser fixed for one shape has broken on a sibling before
// (aegis-hvnqyb's HOST-column / stacked-role / TREE-`?` trio).
func TestCrewIDReadsTheLiveMacHammondRow(t *testing.T) {
	fakeST(t, map[string]string{
		"crew": "  macbookair-stiwi       hammond     administrator  cycle-blocked —        ?         idle             MANUAL  st-hammond",
	})
	t.Setenv("SHANTY_AGENT", "hammond")
	got := plain(CrewID{}.Render())
	if !strings.Contains(got, "admin") || !strings.Contains(got, "idle") {
		t.Errorf("render %q lost hammond's role or WORK verdict", got)
	}
	if strings.Contains(got, "st could not tell") {
		t.Errorf("render %q read a neighboring ? column as the WORK verdict", got)
	}
}
