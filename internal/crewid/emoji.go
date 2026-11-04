// Package crewid assigns each crew member one memorable emoji and remembers it.
//
// The point is a WALL of panes. An operator with a dozen agents open should be
// able to tell whose pane they are looking at without reading text, and the mark
// has to mean the same thing tomorrow as it did today.
//
// So the assignment is STORED, not computed. A hash of the agent's name would be
// deterministic per name but its COLLISIONS are not stable: two agents that hash
// to the same emoji have to be broken apart somehow, and every scheme for doing
// that shifts marks around when the roster changes. Adding one crew member would
// silently re-badge others, which destroys exactly the recognition the mark
// exists to provide. A stored registry assigns once, on first sight, and never
// reassigns.
//
// The registry lives beside shanty's other config as TOML. It is shanty's own
// display state — no crew state is touched by reading or writing it.
package crewid

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"

	"github.com/BurntSushi/toml"
	"github.com/intigraph/shanty/internal/config"
)

// Palette is the pool marks are drawn from, in assignment order.
//
// Chosen for silhouette, not theme: each one has to be tellable from every other
// at a glance in a status bar, so they are creatures with distinct outlines rather
// than a set of near-identical faces or objects. None of them collide with the
// glyphs the bar already uses for meaning (⚓ plate, ⚑ dispatch, ⚙ crew,
// ⚠ attention, ✉ inbox, ⏱ harness, Σ stats) — a mark that reads as a status would
// be worse than no mark.
var Palette = []string{
	"🦊", "🦉", "🐙", "🦈", "🐝", "🦋", "🐢", "🐧",
	"🦁", "🐺", "🦅", "🐗", "🦔", "🦎", "🐳", "🦩",
	"🐊", "🦌", "🐿", "🦇", "🐡", "🦞", "🐌", "🦜",
	"🦡", "🐨", "🐫", "🦥", "🦨", "🦦", "🦫", "🐴",
	"🦭", "🐷", "🐮", "🐸", "🐰", "🐭", "🦃", "🦚",
}

// registryFile is the on-disk name of the mark registry.
const registryFile = "agents.toml"

// registry is the file's shape. A single [agents] table keeps the file readable
// and hand-editable: an operator who wants a specific mark for a specific agent
// should be able to just set it.
type registry struct {
	Agents map[string]string `toml:"agents"`
}

var (
	loadOnce sync.Once
	loaded   map[string]string
)

// onceReset yields a fresh sync.Once. Tests point the config dir at a scratch
// directory between cases and must be able to re-read; a segment process is
// short-lived enough that the memoization never needs clearing in production.
func onceReset() sync.Once { return sync.Once{} }

// Path returns the registry's location.
func Path() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, registryFile), nil
}

// load reads the registry once per process. A missing or unreadable file yields an
// empty map: an absent registry means "nothing assigned yet", which is a normal
// first-run state and not an error to report on a status bar.
func load() map[string]string {
	loadOnce.Do(func() {
		loaded = map[string]string{}
		p, err := Path()
		if err != nil {
			return
		}
		var r registry
		if _, err := toml.DecodeFile(p, &r); err != nil {
			return
		}
		for k, v := range r.Agents {
			if v != "" {
				loaded[k] = v
			}
		}
	})
	return loaded
}

// EmojiFor returns the agent's stored mark, or "" when it has none.
//
// "" is a real answer, not a failure: the palette is finite, and a roster that
// outgrows it gets no mark rather than a duplicate one. A duplicate mark is worse
// than none — it makes two panes look like the same agent, which is the specific
// confusion this package exists to prevent. Callers render the name alone.
func EmojiFor(agent string) string {
	return load()[agent]
}

// Stored returns a copy of every mark currently on disk, assigning nothing.
func Stored() map[string]string {
	src := load()
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// Assign gives every named agent a mark it does not already have and persists the
// result, returning the full agent -> mark map afterwards.
//
// Assignment order is the sorted agent list, and each new agent takes the first
// palette entry not already spoken for, so the same roster against the same file
// always produces the same result. Existing entries are never touched — that is
// the whole promise.
func Assign(agents []string) (map[string]string, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}

	// Several segment processes render at once and any of them may be the first to
	// see a new agent, so the read-modify-write has to be exclusive. Without the
	// lock, two processes racing on a new roster can each assign the same palette
	// entry to different agents — a duplicate mark, permanently stored.
	unlock, err := lock(p + ".lock")
	if err != nil {
		return nil, err
	}
	defer unlock()

	// Re-read under the lock: the copy this process cached may predate another
	// process's assignment.
	cur := map[string]string{}
	var r registry
	if _, err := toml.DecodeFile(p, &r); err == nil {
		for k, v := range r.Agents {
			if v != "" {
				cur[k] = v
			}
		}
	}

	taken := map[string]bool{}
	for _, v := range cur {
		taken[v] = true
	}
	missing := make([]string, 0, len(agents))
	for _, a := range agents {
		if a != "" && cur[a] == "" {
			missing = append(missing, a)
		}
	}
	if len(missing) == 0 {
		return cur, nil
	}
	sort.Strings(missing)
	for _, a := range missing {
		for _, e := range Palette {
			if !taken[e] {
				cur[a] = e
				taken[e] = true
				break
			}
		}
	}
	if err := write(p, cur); err != nil {
		return cur, err
	}
	// Keep this process's view consistent with what we just stored.
	loadOnce.Do(func() {})
	loaded = cur
	return cur, nil
}

// write renders the registry deterministically — sorted keys, one per line — so
// the file diffs cleanly and a human can read it. Temp-file-plus-rename means a
// reader never sees a half-written registry.
func write(path string, m map[string]string) error {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("# shanty crew marks — one emoji per agent, for telling panes apart.\n")
	b.WriteString("# Assigned on first sight and never reassigned: a mark that moves is\n")
	b.WriteString("# worse than no mark. Edit a line to choose your own; keep them distinct.\n\n")
	b.WriteString("[agents]\n")
	for _, n := range names {
		fmt.Fprintf(&b, "%q = %q\n", n, m[n])
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), registryFile+".*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.WriteString(b.String()); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// lock takes an exclusive advisory lock on path, returning the release func.
func lock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}
