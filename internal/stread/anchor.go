package stread

import (
	"strings"
	"time"
)

// Plate is what an agent currently holds, as st reports it.
type Plate struct {
	ID     string // work item id, "" when st reports no item
	Title  string // the item's title, "" when unknown
	Status string // the item's tracker status (in_progress, open, ...), "" when unknown
}

// Empty reports whether st named no item at all.
func (p Plate) Empty() bool { return p.ID == "" }

// Anchor reads what the named agent is holding.
//
// The ID comes from `st anchor <agent> --short`, a documented machine-readable
// flag that prints the id ALONE — so the one field the bar keys on never depends
// on reading prose. The title comes from the full `st anchor <agent>` rendering,
// which is where st prints it; that read is anchored on the id we already know,
// so a layout change in st costs us the title and never the identity.
//
// An empty ID with a nil error means st named no item. That is not the same as
// "the plate is empty" — st answers a lookup against a store it can reach but
// that holds none of these items exactly this way, with empty output and a zero
// exit. Callers must not render it as idle without corroboration; see the crew
// state cross-check in the task segment.
func Anchor(agent string) (Plate, error) {
	id, err := RunTTL(SlowTTL, "anchor", agent, "--short")
	if err != nil {
		return Plate{}, err
	}
	if id == "" {
		return Plate{}, nil
	}
	p := Plate{ID: id}
	// The title belongs to the item, not to the minute: reuse it for as long as
	// the agent holds the same id (aegis-dh6). The full `st anchor` is the
	// expensive half — for an administrator it renders the whole crew's plate,
	// measured at 9 owner-rows Quipu reads (~15k rows) per call against 1 for
	// --short — and the status bar used to pay it every SlowTTL for a title
	// that had not changed.
	titleKey := cacheKey(Bin(), workDir(), []string{"anchor-title", agent, id})
	if raw, ok := cacheGet(titleKey, TitleTTL); ok {
		p.Title, p.Status, _ = strings.Cut(raw, "\x00")
		return p, nil
	}
	full, err := RunTTL(SlowTTL, "anchor", agent)
	if err != nil {
		// The id is real and already in hand; losing the title is a degraded
		// render, not a failure. Returning the error here would throw away the
		// good half of the answer.
		return p, nil
	}
	p.Title, p.Status = titleFor(full, id)
	if p.Title != "" {
		// An unparsed title is not cached: the next SlowTTL miss may read it.
		cacheSet(titleKey, p.Title+"\x00"+p.Status)
	}
	return p, nil
}

// AnchorID reads only the id the agent holds — the cheap `--short` form. A
// segment that renders the id alone must use this, not Anchor, or it pays for
// the full plate rendering just to throw the title away.
func AnchorID(agent string) (string, error) {
	return RunTTL(SlowTTL, "anchor", agent, "--short")
}

// TitleTTL bounds how long an item's title and status are reused while the
// agent keeps holding the same id. A changed id misses at once; a retitled or
// re-statused item shows the change within this window.
const TitleTTL = 30 * time.Minute

// titleFor pulls the title and tracker status out of st's plate line for the
// given id. st renders it as:
//
//	▶ <id>  <title>        (<status>)
//
// The parse finds the line containing the id, drops everything up to and
// including it, and peels a trailing parenthesized status. A line it cannot read
// yields an empty title rather than a guess — the segment then shows the id
// alone, which is still true.
func titleFor(full, id string) (title, status string) {
	for _, line := range strings.Split(full, "\n") {
		i := strings.Index(line, id)
		if i < 0 {
			continue
		}
		rest := strings.TrimSpace(line[i+len(id):])
		if rest == "" {
			return "", ""
		}
		if strings.HasSuffix(rest, ")") {
			if j := strings.LastIndex(rest, "("); j >= 0 {
				status = strings.TrimSuffix(rest[j+1:], ")")
				rest = strings.TrimSpace(rest[:j])
			}
		}
		return rest, status
	}
	return "", ""
}
