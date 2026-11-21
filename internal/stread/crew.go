package stread

import "strings"

// Entry is what one line of `st crew` tells us about an agent. Every field is
// READ from st, never re-derived: the bar, the picker, and st's own tier must
// agree about who is busy, or the bar becomes a second, disagreeing opinion.
type Entry struct {
	Role     string // worker / lead / administrator
	State    string // work verdict cell: busy / idle / waiting / saturated·948k / ...
	Currency string // settings currency: current / STALE / unknown
}

// roleWords is st's closed role vocabulary (shantytown tier.py: VALID_ROLES).
// Matching a closed set is what lets the parse stay position-tolerant — see
// ParseCrew.
var roleWords = map[string]bool{
	"worker": true, "lead": true, "administrator": true,
}

// currencyWords is st's closed settings-currency vocabulary.
var currencyWords = map[string]bool{
	"current": true, "STALE": true, "unknown": true,
}

// roleCell reads st's ROLE cell, which is a comma-joined STACK rather than one
// word — `worker,normal`, `escalation-target,keeper,worker`, `lead,support`.
//
// The tree position (worker / lead / administrator) is the part this bar shows,
// and it is what the closed vocabulary above can recognise; the rest of the stack
// is the deployment's own trait vocabulary and is deliberately open, so it cannot
// be matched against anything. Returning the tree position means a stacked card
// renders as its position instead of vanishing — which is what happened before,
// because an exact match on `worker,normal` finds nothing and the row was then
// dropped for having no role.
//
// Empty means "this cell is not a role", which is also how a header row and a
// free-text note get skipped.
func roleCell(cell string) string {
	for _, part := range strings.Split(cell, ",") {
		if roleWords[part] {
			return part
		}
	}
	return ""
}

// StateRank orders st's work verdicts by how much they need a human's eyes. The
// ones an operator must look at FIRST — blocked on a question, wedged, a stalled
// send, a context wall — sort before the ones that are fine. These are st's own
// words (shantytown triage.py), matched, never recomputed.
var StateRank = map[string]int{
	"waiting":   0, // BLOCKED on a question in the pane — needs a person, will not time out
	"wedged":    1, // dead / stuck
	"queued":    2, // unsubmitted text in the box — a stalled dispatch, or a human mid-sentence
	"saturated": 3, // over the context limit — looks free, is a wall
	"?":         4, // st could not tell
	"busy":      5, // mid-flight — working, leave it
	"idle":      6, // free
}

// StateWord extracts the leading verdict word from a work cell, so
// "saturated·948k" and "busy+1sh" map to "saturated"/"busy". Returns "" if the
// cell matches no known verdict.
func StateWord(cell string) string {
	if cell == "" {
		return ""
	}
	if strings.HasPrefix(cell, "?") {
		return "?"
	}
	// Only one verdict can prefix a given cell (the words are disjoint), so map
	// iteration order does not affect the result.
	for word := range StateRank {
		if word != "?" && strings.HasPrefix(cell, word) {
			return word
		}
	}
	return ""
}

// RankOf maps a work cell to its attention rank; an unrecognized cell sorts after
// every judged one but before nothing.
func RankOf(cell string) int {
	if r, ok := StateRank[StateWord(cell)]; ok {
		return r
	}
	return 90
}

// Busy reports whether a work cell means the agent is mid-flight. Used to catch a
// contradiction the bar must not hide: an agent st calls busy whose plate reads
// empty is not idle, it is unreadable.
func Busy(cell string) bool {
	switch StateWord(cell) {
	case "busy", "saturated", "queued":
		return true
	}
	return false
}

// ParseCrew turns `st crew`'s table into agent -> Entry.
//
// It reads position-tolerantly on purpose — shanty does not own st's column order
// and must not break when it changes. The NAME is always the first field; the
// ROLE, the CURRENCY, and the STATE are each identified by their own closed
// vocabulary.
//
// A row must carry BOTH a role and a verdict to count. Requiring only a verdict is
// not enough, and the reason is a live trap: st follows its table with summary
// prose of the form
//
//	9 busy: bond, felix, goodnight
//
// whose second field "busy:" is verdict-shaped, so a verdict-only filter reads
// that line as an agent literally named "9" and puts it in the crew. Every real
// row names a role; no summary line does.
func ParseCrew(out string) map[string]Entry {
	res := map[string]Entry{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// THE AGENT IS THE FIELD BEFORE ITS ROLE, not the first field on the line.
		//
		// This used to read `fields[0]`, which was right when a row began with the
		// agent. st later grew a HOST column, so every row now starts with the host
		// and `fields[0]` was the MACHINE's name — so a two-agent host stored both
		// rows under one key, the later overwriting the earlier, and a lookup by
		// agent name found nothing at all. The visible symptom was this bar
		// rendering a permanent "st?" beside a pane st could describe perfectly
		// well when asked directly, which read as st being unable to tell.
		//
		// Anchoring on the role keeps the parse position-tolerant, which is the
		// property the closed vocabularies above exist to provide: it reads the old
		// two-column shape and the new three-column one without being told which is
		// which, and it will survive the next column added to either end.
		roleAt := -1
		for i, f := range fields {
			if roleCell(f) != "" {
				roleAt = i
				break
			}
		}
		if roleAt < 1 {
			continue // a header, a note, or a row with no role to anchor on
		}
		name := fields[roleAt-1]
		var e Entry
		for _, f := range fields {
			switch {
			case e.Role == "" && roleCell(f) != "":
				e.Role = roleCell(f)
			case currencyWords[f]:
				e.Currency = f
			case e.State == "" && StateWord(f) != "" && StateWord(f) != "?":
				e.State = f
			}
		}
		// A BARE "?" IS THE LAST RESORT, NOT THE FIRST MATCH.
		//
		// `?` is a legal work verdict ("st could not tell"), and it is also what
		// the TREE column prints for a reading it could not verify. Those columns
		// sit next to each other and this parse is position-tolerant by design, so
		// the tree's `?` was winning and every agent with an unverified tree read
		// as "st could not tell" about its WORK — which is a different and much
		// more alarming claim, and one the pane could disprove by simply being
		// busy. Taking `?` only when no real verdict was found anywhere on the row
		// keeps the honest could-not-tell without letting an unrelated column
		// manufacture it.
		if e.State == "" {
			for _, f := range fields {
				if StateWord(f) == "?" {
					e.State = f
					break
				}
			}
		}
		if e.State == "" || e.Role == "" {
			continue
		}
		res[name] = e
	}
	return res
}

// Crew reads and parses `st crew`. A nil map with a non-nil error means we could
// not ask; a nil map with a nil error means st answered with no judgeable crew.
func Crew() (map[string]Entry, error) {
	out, err := Run("crew")
	if err != nil {
		return nil, err
	}
	return ParseCrew(out), nil
}

// DisplayCrew shares the roster reuse window used by the crew-count segment.
// This observation is for status segments; interactive pickers use Crew.
func DisplayCrew() (map[string]Entry, error) {
	out, err := RunTTL(RosterTTL, "crew")
	if err != nil {
		return nil, err
	}
	return ParseCrew(out), nil
}
