package segments

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/intigraph/shanty/internal/crewid"
	"github.com/intigraph/shanty/internal/stread"
)

// sessionName is the tmux session the segment is being drawn in, set by the seg
// command from tmux's #{session_name}. It is the per-pane identity the SHARED
// fleet bar needs: one tmux server has one $SHANTY_AGENT env, but many sessions,
// so a global status-right rendered from $SHANTY_AGENT alone is blank for
// everyone. Deriving the agent from the session each segment is drawn in lets ONE
// bar show each pane its OWN identity, plate, and stats.
var sessionName string

// SetSession records the session the current render is for (the seg command
// passes it from #{session_name}).
func SetSession(s string) {
	sessionName = s
}

// agentName resolves this agent's shantytown identity.
//
// $SHANTY_AGENT wins — it is the same variable st itself uses, so a pane that
// exports its own identity is authoritative. Note that tmux runs status commands
// from the SERVER's environment, not the pane's, so on a real fleet this is
// usually unset and the session name is what identifies the pane.
//
// An empty result means "we cannot tell who we are". Guessing would put another
// agent's plate on this bar, so segments say so instead.
func agentName() string {
	if a := os.Getenv("SHANTY_AGENT"); a != "" {
		return a
	}
	// Both shanty's own `shanty-` prefix and shantytown's `st-` are recognized —
	// see stread.SessionPrefixes. Knowing only the former is what blanked every
	// per-agent segment on the fleet st was driving.
	return stread.AgentFromSession(sessionName)
}

// Dracula palette, named so the render sites read as intent.
const (
	colFG     = "#f8f8f2"
	colDim    = "#6272a4"
	colRed    = "#ff5555"
	colOrange = "#ffb86c"
	colGreen  = "#50fa7b"
	colCyan   = "#8be9fd"
	colPurple = "#bd93f9"
	colYellow = "#f1fa8c"
)

func paint(color, s string) string {
	return fmt.Sprintf("#[fg=%s]%s#[default]", color, s)
}

// hidden is the answer for "shantytown is not installed here".
//
// It is the ONLY silent-empty this file permits, and it is deliberate: shanty is
// usable without shantytown, and that user must not see warnings about a tool
// they never installed. Every other failure renders something.
const hidden = ""

// loud renders a failure the operator can act on. A status bar that goes blank
// when it breaks is worse than one that says it broke: blank looks like "nothing
// to report", so it gets believed.
func loud(s string) string { return paint(colRed, "⚠ "+s) }

// identity is the per-render agent resolution shared by the per-agent segments.
// It returns the agent name, or a rendered explanation of why there is none.
func identity() (agent string, problem string) {
	if !stread.Installed() {
		return "", hidden
	}
	if a := agentName(); a != "" {
		return a, ""
	}
	// st IS here and we still cannot say whose pane this is. That is a real
	// misconfiguration — an unprefixed session name and no exported identity — and
	// it is worth a word, because the alternative is a bar that looks fine while
	// showing nobody's work.
	return "", loud("no agent")
}

// stateColor maps st's work verdict to how much it wants an eye. These are st's
// own words (shantytown triage.py), matched, never recomputed.
func stateColor(cell string) string {
	v := stread.ParseVerdict(cell)
	// Idle-with-live-work is not the calm green that plain idle earns: st says the
	// agent is free AND that work is still running in it, which is the combination a
	// coordinator dispatches over.
	if v.MisleadinglyIdle() {
		return colOrange
	}
	switch v.Word {
	case "waiting", "wedged":
		return colRed
	case "queued", "saturated":
		return colOrange
	case "idle":
		return colGreen
	case "busy":
		return colCyan
	default:
		return colDim
	}
}

// crewEntry reads this agent's row from `st crew`.
func crewEntry(agent string) (stread.Entry, error) {
	crew, err := stread.Crew()
	if err != nil {
		return stread.Entry{}, err
	}
	e, ok := crew[agent]
	if !ok {
		return stread.Entry{}, fmt.Errorf("agent %q is not in st's crew", agent)
	}
	return e, nil
}

// --- CrewID ----------------------------------------------------------------

// CrewID renders WHO this pane is: the agent's stored mark, its name, its role,
// and st's verdict on what it is doing. It is the segment that makes a wall of
// panes readable without reading.
type CrewID struct{}

func (CrewID) Name() string { return "crewid" }

// IT DOES NOT REPEAT THE NAME (Stiwi, 2026-09-21: "there is an st segment that
// just says 'hammond' which is a duplicate of the tmux header, so we can remove
// it").
//
// The session pill on the LEFT of the bar already renders `<mark> <agent>`, and
// this segment used to open with exactly the same two things on the right. On a
// one-agent pane that made the bar say "🦊 hammond … 🦊 hammond ⚠ st?" — the
// name twice and the mark twice, spending the scarcest thing a status bar has
// (width) to say nothing the eye had not already read at the other end.
//
// WHAT IS KEPT IS WHAT IS NOT SAID ANYWHERE ELSE: the role, st's verdict on this
// pane, the evidence behind that verdict, and a stale-settings flag. So this is a
// deduplication and not a removal — the identity half moved out, the diagnostic
// half stayed. The `st?` path keeps working for the same reason: it now says only
// that st could not answer, which was always the informative part of that line.
//
// The mark is still ASSIGNED here when the agent has none. Assignment is a
// persistent side effect the session pill also depends on, and first sight of a
// pane can happen through either segment.
func (CrewID) Render() string {
	agent, problem := identity()
	if agent == "" {
		return problem
	}

	if crewid.EmojiFor(agent) == "" {
		// First sight of this agent. Assign now and persist, so the mark this pane
		// gets today is the mark it keeps — the session pill reads the same store.
		_, _ = crewid.Assign([]string{agent})
	}

	e, err := crewEntry(agent)
	if err != nil {
		// We know who we are but not what st thinks of us. The identity is on the
		// session pill already, so all this has to add is that the other half is
		// missing.
		return loud("st?")
	}

	out := ""
	if e.Role != "" {
		out = paint(colFG, shortRole(e.Role))
	}

	v := stread.ParseVerdict(e.State)
	// `strings.TrimSpace` at the end rather than a conditional separator here: a
	// card with no role leaves `out` empty, and a bar segment that begins with a
	// stray space renders as a visible gap tmux will not collapse.
	out += " " + paint(stateColor(e.State), v.Word)
	// The evidence behind the verdict, in words. A coordinator deciding whether to
	// dispatch is choosing between this bar and some other idle signal; a verdict
	// with its evidence attached can be trusted or argued with, where a bare word
	// can only be believed or ignored.
	if why := v.Why(); why != "" {
		colour := colDim
		if v.MisleadinglyIdle() {
			// st calls this agent idle AND says work is still running in it. That is
			// the shape that gets dispatched over, so it does not get to look calm.
			colour = colOrange
		}
		out += " " + paint(colour, "("+why+")")
	}
	if e.Currency == "STALE" {
		// st reports this agent is running settings older than the file on disk.
		// The pane looks healthy and its hooks are whatever the file said at
		// launch, so this is exactly the kind of thing a bar should not hide.
		out += " " + paint(colOrange, "settings:STALE")
	}
	return strings.TrimSpace(out)
}

// withMark prefixes the mark when there is one. A missing mark is not an error
// (the palette is finite) so it costs a space, not a warning.
func withMark(mark, s string) string {
	if mark == "" {
		return s
	}
	return mark + " " + s
}

// shortRole abbreviates st's roles to keep the bar narrow while staying
// unambiguous — st's role vocabulary is closed, so these cover it.
func shortRole(role string) string {
	switch role {
	case "administrator":
		return "admin"
	case "worker":
		return "wkr"
	default:
		return role
	}
}

// --- Task ------------------------------------------------------------------

// TitleWidth is how much of a work item's title the bar shows. Long enough to
// recognize the work, short enough to leave room for everything else.
const TitleWidth = 34

// Task renders WHAT this agent is working on: the held item's id and a clipped
// title.
//
// It has three distinct renderings and none of them is blank:
//
//	holding an item  ⚓ ss-1234 rework the widget cache
//	empty plate      ⚓ — nothing held
//	no item, busy    ⚓ ⚠ busy, no item
//
// The third one is the whole reason this segment does not just print what st
// returns. `st anchor <agent> --short` answers a lookup it cannot resolve with
// EMPTY output and a zero exit, so a bar built naively on it renders blank —
// consistently, silently, and looking exactly like an idle agent. Cross-checking
// against st's own busy/idle verdict turns that into a visible contradiction.
//
// It says "busy, no item" and NOT "unreadable", because two different situations
// produce it and shanty cannot tell them apart:
//
//   - the tracker is unreachable from where st ran, so the plate reads empty;
//   - the tracker is fine and the agent really is working on something nobody
//     put on a plate — untracked work.
//
// Both want a human. Naming a cause we have not established would be the same
// species of error as rendering blank: an unearned claim that reads as fact. So the
// segment states the two things it knows — st calls this agent busy, and no item is
// named — and leaves the diagnosis to the person who can check.
type Task struct{}

func (Task) Name() string { return "task" }

func (Task) Render() string {
	agent, problem := identity()
	if agent == "" {
		return problem
	}

	plate, err := stread.Anchor(agent)
	if err != nil {
		return paint(colDim, "⚓ ") + loud("st?")
	}
	if !plate.Empty() {
		out := paint(colYellow, "⚓ "+plate.ID)
		if plate.Title != "" {
			out += " " + paint(colFG, clip(plate.Title, TitleWidth))
		}
		return out
	}

	// No item named. Ask st what it thinks this agent is doing before calling it
	// idle. A crew read that itself fails leaves us unable to judge — which is a
	// warning, not an idle pane.
	e, cerr := crewEntry(agent)
	if cerr != nil {
		return paint(colDim, "⚓ ") + loud("no item, state unknown")
	}

	// An ADMINISTRATOR holding nothing is the expected state, not a fault. The role
	// exists to stay free to coordinate and is never assigned implementation work, so
	// warning about its empty plate produces a red that is known-false every time it
	// is drawn — and a status surface that permanently shows one false red teaches
	// its reader to discount every red on it. Keyed on the ROLE, never on a name, so
	// whoever holds the role next inherits the same treatment.
	if e.Role == roleAdministrator {
		return dispatchSummary()
	}

	if stread.Busy(e.State) {
		return paint(colDim, "⚓ ") + loud(stread.StateWord(e.State)+", no item")
	}
	return paint(colDim, "⚓ — nothing held")
}

// roleAdministrator is st's name for the coordinating role (shantytown tier.py:
// VALID_ROLES). It is matched as a role, deliberately never as an agent name.
const roleAdministrator = "administrator"

// dispatchSummary is what the item slot shows for an administrator: the state that
// role actually acts on, instead of an absence rendered as a fault.
//
// Every number is counted from the same `st crew` read the identity segment already
// made — st's own verdicts, not a second opinion, and no extra process. "needs eyes"
// is st's own attention vocabulary: blocked on a question, wedged, a stalled send, a
// context wall. Those are the agents a coordinator must look at before anything else.
//
// A crew read that fails here says so rather than reporting a confident zero: "no
// crew is free" and "I could not count the crew" are opposite instructions.
func dispatchSummary() string {
	crew, err := stread.Crew()
	if err != nil || len(crew) == 0 {
		return paint(colDim, "⚑ ") + loud("crew unreadable")
	}
	var free, attention, liveShells int
	for _, e := range crew {
		v := stread.ParseVerdict(e.State)
		switch {
		case v.Word == "idle" && !v.WorkStillRunning():
			free++
		case v.Word == "waiting", v.Word == "wedged", v.Word == "queued", v.Word == "saturated":
			attention++
		}
		// Counted separately and NOT as free: an agent st calls idle whose background
		// work is still running is not available, and this is the single number that
		// most often makes a dispatch decision wrong.
		if v.MisleadinglyIdle() {
			liveShells++
		}
	}
	var parts []string
	if free > 0 {
		parts = append(parts, fmt.Sprintf("%d free", free))
	}
	if attention > 0 {
		parts = append(parts, fmt.Sprintf("%d need eyes", attention))
	}
	if liveShells > 0 {
		parts = append(parts, fmt.Sprintf("%d idle+live", liveShells))
	}
	if len(parts) == 0 {
		// Nobody free, nobody stuck: every agent has work and none of it is stalled.
		return paint(colGreen, "⚑ crew fed")
	}
	colour := colGreen
	if attention > 0 || liveShells > 0 {
		colour = colOrange
	}
	return paint(colour, "⚑ "+strings.Join(parts, " · "))
}

// clip shortens a title to n cells, marking the cut. It counts runes, not bytes: a
// byte slice through a multi-byte character would emit a broken glyph into the
// status line.
func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimRight(string(r[:n-1]), " ") + "…"
}

// --- Stats -----------------------------------------------------------------

// Stats renders what this agent actually did, from st's local capture store:
// activity, files touched, token traffic.
//
// When no capture store exists it says so ("Σ off") rather than rendering zeros.
// `st stats` is wired as a command on every deployment but its numbers only exist
// once the harness hooks that feed it are installed, and "nobody is counting" must
// not look like "this agent did nothing".
type Stats struct{}

func (Stats) Name() string { return "stats" }

func (Stats) Render() string {
	agent, problem := identity()
	if agent == "" {
		return problem
	}
	s, err := stread.Stats(agent)
	if err != nil {
		if errors.Is(err, stread.ErrNotInstalled) {
			return hidden
		}
		return paint(colDim, "Σ ") + loud("st?")
	}
	if !s.Captured {
		return paint(colDim, "Σ off")
	}
	parts := []string{fmt.Sprintf("%d⚡", s.Events)}
	if s.Files > 0 {
		parts = append(parts, fmt.Sprintf("%df", s.Files))
	}
	if s.UsageKnown {
		parts = append(parts, "in"+human(s.UsageIn), "out"+human(s.UsageOut))
		if s.UsageIn > 0 {
			pct := int(float64(s.CacheRead)/float64(s.UsageIn)*100 + 0.5)
			parts = append(parts, fmt.Sprintf("cache%d%%", pct))
		}
	} else if t := s.Tokens(); t > 0 {
		// Backward-compatible with an older st writer: preserve the requested
		// split even before transcript/cache dimensions appear on the wire.
		parts = append(parts, "in"+human(s.TokensIn), "out"+human(s.TokensOut))
	}
	return paint(colPurple, "Σ "+strings.Join(parts, " "))
}

// human abbreviates a count for a status bar: 950, 12k, 3.4M.
func human(n int) string {
	switch {
	case n >= 1_000_000:
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/1e6), ".0") + "M"
	case n >= 1_000:
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/1e3), ".0") + "k"
	default:
		return fmt.Sprintf("%d", n)
	}
}

// --- Anchor / Crew / Events / Inbox / Harness ------------------------------

// Anchor renders just the held item's id. It predates Task and stays for anyone
// whose bar is configured with it; Task is the fuller answer.
type Anchor struct{}

func (Anchor) Name() string { return "anchor" }

func (Anchor) Render() string {
	agent, problem := identity()
	if agent == "" {
		return problem
	}
	plate, err := stread.Anchor(agent)
	if err != nil {
		return loud("st?")
	}
	if plate.Empty() {
		return paint(colDim, "⚓ —")
	}
	return paint(colFG, "⚓ "+plate.ID)
}

// Crew renders the busy/total worker count.
type Crew struct{}

func (Crew) Name() string { return "crew" }

func (Crew) Render() string {
	if !stread.Installed() {
		return hidden
	}
	count, err := stread.Run("crew", "--count")
	if err != nil {
		return loud("crew?")
	}
	// Suppress only "0/0" — st's own "nothing judgeable" answer. A crew of "0/9"
	// is NOT hidden: a fully idle crew is real information for a coordinator, and
	// hiding it would make "no crew configured" and "every worker idle" look
	// identical on the bar.
	if count == "" || count == "0/0" {
		return hidden
	}
	return paint(colGreen, "⚙ "+count)
}

// Events renders the count of undelivered stop events addressed to this agent.
type Events struct{}

func (Events) Name() string { return "events" }

func (Events) Render() string {
	agent, problem := identity()
	if agent == "" {
		return problem
	}
	// --events is a READ: it never marks anything delivered, so polling it from
	// the status bar cannot destroy pending state.
	count, err := stread.Run("anchor", agent, "--events")
	if err != nil {
		return loud("events?")
	}
	if count == "" || count == "0" {
		return hidden
	}
	return paint(colRed, "⚠ "+count)
}

// Inbox renders this agent's unread message count.
type Inbox struct{}

func (Inbox) Name() string { return "inbox" }

func (Inbox) Render() string {
	agent, problem := identity()
	if agent == "" {
		return problem
	}
	// --count is a pure read: it never marks anything read, so polling it from the
	// status bar cannot destroy unread state.
	count, err := stread.RunInboxCount(agent)
	if err != nil {
		return loud("inbox?")
	}
	if count == "" || count == "0" {
		return hidden
	}
	return paint(colPurple, "✉ "+count)
}

// Harness renders the name of the agent runtime backing this agent.
type Harness struct{}

func (Harness) Name() string { return "harness" }

func (Harness) Render() string {
	agent, problem := identity()
	if agent == "" {
		return problem
	}
	name, err := stread.Run("anchor", agent, "--harness")
	if err != nil {
		return loud("harness?")
	}
	if name == "" {
		return loud("harness?")
	}
	defaultName, err := stread.DefaultHarness()
	if err != nil {
		return loud("harness?")
	}
	if name == defaultName {
		return hidden
	}
	// A harness is a program, not elapsed time. Name only deviations from the
	// deployment default, using font-safe text instead of a stopwatch glyph that
	// reads as a timer.
	return paint(colCyan, "agent "+name)
}
