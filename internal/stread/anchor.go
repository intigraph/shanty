package stread

import (
	"encoding/json"
	"fmt"
)

// Plate is what an agent currently holds, as st reports it.
type Plate struct {
	ID     string // work item id, "" when st reports no item
	Title  string // the item's title, "" when unknown
	Status string // the item's tracker status (in_progress, open, ...), "" when unknown
}

// Empty reports whether st named no item at all.
func (p Plate) Empty() bool { return p.ID == "" }

// taskContext is the part of `st anchor <agent> --json` (schema
// st.task-context/1) the bar reads: the plate item, or null.
type taskContext struct {
	Schema string          `json:"schema"`
	Item   json.RawMessage `json:"item"`
}

type plateItem struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// TaskContextSchema is the only `st anchor --json` envelope the bar accepts.
const TaskContextSchema = "st.task-context/1"

// Anchor reads what the named agent is holding, from ONE machine-readable read.
//
// `st anchor <agent> --json` returns the plate item's id, title and status
// together (aegis-dh6). The bar used to pair `--short` (the id) with the full
// prose `st anchor` (the title). For an administrator that prose renders the
// whole crew's plate — measured at 17 Quipu requests with 9 owner-rows reads
// against 7 requests / 2 small owner-rows for --json — and the title parse
// took the first line containing the id, which in today's layout is the "Top
// of plate" summary, not the held item. One answer also means the id and the
// title can never describe two different items.
//
// An empty ID with a nil error means st named no item. That is not the same as
// "the plate is empty" — st answers a lookup against a store it can reach but
// that holds none of these items exactly this way, with a null item and a zero
// exit. Callers must not render it as idle without corroboration; see the crew
// state cross-check in the task segment.
func Anchor(agent string) (Plate, error) {
	out, err := RunTTL(SlowTTL, "anchor", agent, "--json")
	if err != nil {
		return Plate{}, err
	}
	return parseTaskContext(agent, out)
}

// parseTaskContext fails closed. An empty answer, a missing or different
// schema, a missing "item" key, or anything that is not valid JSON is st we
// cannot read, which the segments render loud — never an empty plate. Only an
// explicit `"item": null` inside a valid envelope means st named no item.
func parseTaskContext(agent, out string) (Plate, error) {
	bad := func(why string) (Plate, error) {
		return Plate{}, fmt.Errorf("st anchor %s --json: unreadable answer: %s", agent, why)
	}
	if out == "" {
		return bad("empty output")
	}
	var ctx taskContext
	if err := json.Unmarshal([]byte(out), &ctx); err != nil {
		return bad(err.Error())
	}
	if ctx.Schema != TaskContextSchema {
		return bad(fmt.Sprintf("schema %q, want %q", ctx.Schema, TaskContextSchema))
	}
	if len(ctx.Item) == 0 {
		return bad("no item field")
	}
	if string(ctx.Item) == "null" {
		return Plate{}, nil
	}
	var item plateItem
	if err := json.Unmarshal(ctx.Item, &item); err != nil {
		return bad("item: " + err.Error())
	}
	if item.ID == "" {
		return bad("item without an id")
	}
	return Plate{ID: item.ID, Title: item.Title, Status: item.Status}, nil
}

// AnchorID reads only the id the agent holds. It shares Anchor's cached answer,
// so the anchor and task segments together cost one st read per SlowTTL.
func AnchorID(agent string) (string, error) {
	p, err := Anchor(agent)
	return p.ID, err
}
