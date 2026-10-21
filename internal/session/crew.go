package session

import (
	"sort"
	"sync"

	"github.com/intigraph/shanty/internal/stread"
)

// CrewRow is one enriched line of the session picker: a session name joined with
// shantytown's own verdict about what that agent is doing. Every judgement here
// is READ from `st`, never re-derived — the picker and the tier must agree on who
// is busy, or the picker becomes a second, disagreeing opinion.
type CrewRow struct {
	Name     string // session/agent display name (shanty- prefix already stripped)
	Item     string // current item held (`st anchor <name> --short`), "" if none/unknown
	State    string // st's work verdict cell (busy / idle / waiting / saturated·948k / ...)
	Currency string // settings currency (current / STALE / unknown), "" if not reported
	Role     string // st's role: worker / lead / administrator. FULL, not abbreviated —
	// the bar shortens roles (shortRole) because it lives in a 30-column budget;
	// this table does not, and "administrator" carries information "admin" drops.
	rank int // attention sort key: lower surfaces first
}

// crewEntry is the slice of `st crew` the picker keeps per agent. The reading is
// delegated to stread, which is shanty's ONE st reader: the picker and the status
// bar must not drift into two parsers of the same table, and stread also owns
// finding the binary and caching the answer.
type crewEntry = stread.Entry

// stCrewAvailable reports whether we can ask st for crew state at all.
func stCrewAvailable() bool { return stread.Installed() }

// runST shells out to st and returns trimmed stdout, or "" on any error — the
// fail-quiet contract the picker wants. A missing binary, a non-zero exit, or an
// empty plate all collapse to "": the picker shows less, never a crash. (The
// status bar deliberately does NOT do this; a blank bar segment is a lie. A picker
// row with a dash is not, because the human is looking right at it.)
func runST(args ...string) string {
	out, err := stread.Run(args...)
	if err != nil {
		return ""
	}
	return out
}

// stateWord and rankOf are the picker's names for stread's verdict vocabulary.
func stateWord(cell string) string { return stread.StateWord(cell) }
func rankOf(cell string) int       { return stread.RankOf(cell) }

// parseCrew turns `st crew`'s table into name -> entry.
func parseCrew(out string) map[string]crewEntry { return stread.ParseCrew(out) }

// buildRows joins the session list with parsed crew state and a per-agent item
// lookup, then sorts attention-first (then by name). Pure: `anchor` is injected
// so the join and the ordering are testable without a live st.
func buildRows(sessions []string, crew map[string]crewEntry, anchor func(name string) string) []CrewRow {
	rows := make([]CrewRow, len(sessions))
	for i, name := range sessions {
		row := CrewRow{Name: name, Item: anchor(name)}
		if e, ok := crew[name]; ok {
			row.State, row.Currency, row.Role, row.rank = e.State, e.Currency, e.Role, rankOf(e.State)
		} else {
			row.rank = 99 // a live session st does not know as crew — sort last
		}
		rows[i] = row
	}
	sort.SliceStable(rows, func(a, b int) bool {
		if rows[a].rank != rows[b].rank {
			return rows[a].rank < rows[b].rank
		}
		return rows[a].Name < rows[b].Name
	})
	return rows
}

// EnrichedRows produces the crew-oriented picker rows for the given sessions, or
// (nil, false) when st cannot enrich them — a missing st binary OR an st that
// reports no judgeable crew. The caller falls back to the plain name list in that
// case: on a machine with no shantytown, or with only personal sessions, a plain
// list is the honest answer, not an empty table.
func EnrichedRows(sessions []string) ([]CrewRow, bool) {
	if len(sessions) == 0 || !stCrewAvailable() {
		return nil, false
	}
	crew := parseCrew(runST("crew"))
	if len(crew) == 0 {
		return nil, false
	}
	// Fetch each agent's held item concurrently: st is a separate process per
	// call, and a serial loop over a full crew is visibly slow for an on-demand
	// picker. anchor items only appear when st's configured tracker has a plate;
	// where it does not, every Item is "" and the picker still shows state.
	items := make([]string, len(sessions))
	var wg sync.WaitGroup
	for i, name := range sessions {
		i, name := i, name
		wg.Add(1)
		go func() {
			defer wg.Done()
			items[i] = runST("anchor", name, "--short")
		}()
	}
	wg.Wait()
	idx := make(map[string]int, len(sessions))
	for i, n := range sessions {
		idx[n] = i
	}
	return buildRows(sessions, crew, func(name string) string { return items[idx[name]] }), true
}
