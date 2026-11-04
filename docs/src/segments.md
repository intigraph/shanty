# Status Bar Segments

The shanty status bar is built from pluggable segments. Each segment is a small Go module that renders a short string for the tmux status bar.

## How segments work

tmux calls `shanty seg <name>` at the configured interval (default: 5 seconds). The segment renders its output — optionally with tmux color codes — and tmux displays it in the status bar.

```bash
# Render a single segment (for testing)
shanty seg cpu

# List all available segments
shanty seg list
```

## Default layout

- **Left:** the agent's mark and name
- **Right:** crewid, task, events, inbox, crew, stats, harness, cpu, mem, host, clock

The right-hand budget is 200 cells. tmux truncates `status-right` silently, so a
budget sized for count-only segments would hide the identity, title and stats
fields without saying so.

## System segments

### session

The leftmost pill: who this pane belongs to — the agent's mark and name, derived
from the session name tmux passes in.

It takes the session name as an argument rather than asking tmux for it. Querying
cannot work when shanty is pointed at a foreign socket (`SHANTY_TMUX_SOCKET`): the
query fails, and every pane then falls back to the same placeholder. A bar that
labels twelve different agents identically is worse than one with no label.

### clock

Current time in `HH:MM` format.

### host

System hostname.

### cpu

CPU usage as a labelled percentage (`cpu 23%`), color-coded by threshold. Samples `/proc/stat` over a 200ms window.

### mem

Memory usage as a labelled percentage (`mem 61%`), color-coded. Reads from `/proc/meminfo`.

### load

Labelled 1-minute load average (`load 1.4`) from `/proc/loadavg`.

### disk

Root partition (`/`) usage as a labelled percentage (`disk 88%`), color-coded.

## shantytown segments

Eight further segments surface state from [shantytown](https://github.com/scbrown/shantytown),
a multi-agent workspace manager. They shell out to shantytown's `st` CLI.

| Segment | Description | Example |
|---------|-------------|---------|
| `crewid` | Who this pane is: mark, name, role, state | `🦊 bond·wkr busy` |
| `task` | The item held, with its title | `⚓ ss-1 rework the cache` |
| `stats` | Activity, files, input/output tokens, prompt-cache hit rate | `Σ 412⚡ 17f in180k out42k cache50%` |
| `crew` | Busy / total workers | `⚙ 3/9` |
| `events` | Undelivered stop events for you | `⚠ 2` |
| `inbox` | Unread messages | `✉ 1` |
| `harness` | Non-default agent program (silent at the deployment default) | `agent claude` |
| `anchor` | The held item's id alone (superseded by `task`) | `⚓ ss-1` |

The stats cache percentage is `cache_read / usage_in`. `usage_in` is normalized
prompt traffic: Claude's base, cache-read and cache-creation dimensions are
combined, while Codex's input total already includes its cached subset. If the
installed `st` predates these transcript dimensions, the bar still shows the
legacy input/output split and does not invent a cache percentage.

## Failure is visible

A status segment that goes blank when it breaks reads as "nothing to report", so it
gets believed. The st-backed segments render an empty string when no `st` binary
exists anywhere, meaning you do not run shantytown and must not be warned about a
tool you never installed. The `harness` segment has one additional intentional
quiet state: the agent uses the deployment's configured default. A non-default
harness is named, and an unreadable harness or configuration remains loud.

With `st` present, everything else gets words:

| Situation | Rendering |
|-----------|-----------|
| Agent holds an item | `⚓ ss-1 rework the cache` |
| Agent is idle, plate empty | `⚓ — nothing held` |
| `st` calls the agent busy but names no item | `⚓ ⚠ busy, no item` |
| No identity derivable | `⚠ no agent` |
| An `st` call failed | `⚠ st?` |
| `st stats` has no capture store | `Σ off` |
| `st` reports the agent's settings are stale | `settings:STALE` beside the name |
| The agent is an `administrator` holding nothing | `⚑ 2 free · 1 need eyes` (see below) |

The third row is the one worth understanding. `st anchor <agent> --short` answers a
lookup against a store that does not hold the item with EMPTY output and a zero
exit — so a bar built naively on it renders blank, consistently and silently, and
looks exactly like an idle agent. The `task` segment therefore cross-checks against
`st`'s own busy/idle verdict and reports the contradiction.

It says "busy, no item" rather than naming a cause, because two different situations
produce it and shanty cannot tell them apart: the tracker may be unreachable from
where `st` ran, or the tracker may be fine and the agent really is working on
something nobody put on a plate. Both want a human. Asserting a cause we have not
established would be the same species of error as rendering blank — an unearned
claim that reads as fact — so the segment states only what it knows and leaves the
diagnosis to the person who can check.

The last row is not an error but is easy to miss: an agent running settings older
than the file on disk looks perfectly healthy while its hooks are whatever the file
said at launch.

`crew` hides only on `0/0`; an idle-but-present crew such as `0/9` still shows,
because "no crew" and "everyone idle" are different facts. `events` and `inbox` hide
on zero — a quiet inbox has nothing to say.

### Why the bar thinks what it thinks

`st` packs three facts into one work cell — the verdict, the number of background
shells still running, and (when saturated) the context size:

    busy   busy+1sh   idle+1sh   saturated·948k

`crewid` decodes that into words: `busy (1 shell live)`, `saturated (948k ctx)`,
`waiting (blocked on a question)`. Nothing is inferred; the evidence was already in
`st`'s cell. It matters because a coordinator choosing whether to dispatch is weighing
this bar against some other idle signal, and a verdict with its evidence attached can
be trusted or argued with, where a bare word can only be believed or ignored.

**`idle+1sh` is the case to know.** `st`'s own note on that suffix reads "idle AND
carrying live work": an agent whose turn ended with a build, a test run, or a
`gh run watch` still live is not finished. `idle` is exactly the word that gets it
dispatched over, so the bar does not paint it the calm green a genuinely free agent
gets, and the administrator's dispatch summary counts it apart from free.

### The administrator's item slot

An `administrator` never holds an implementation item — the role exists to stay free
to coordinate. Warning about its empty plate would be a red that is known-false every
time it is drawn, and one permanent false red on a status surface teaches its reader
to discount every red on it. So for that role the slot shows the state the role
actually acts on, counted from the same `st crew` read:

    ⚑ 2 free · 1 need eyes · 1 idle+live
    ⚑ crew fed                             (everyone has work, nothing stalled)
    ⚑ ⚠ crew unreadable                    (we could not count — not a confident zero)

This is keyed on the ROLE, never on an agent name, so a future administrator inherits
it and the current one loses it if the role moves. For a `worker`, busy-with-no-item
stays loud: there it is real signal.

### Agent identity

Every segment except `crew` is per-agent. Identity comes from `$SHANTY_AGENT` — the
same variable `st` itself uses — and otherwise from the session name, stripping
either `shanty-` or shantytown's own `st-` prefix.

The session-name path is the normal one on a fleet: tmux runs status commands from
the tmux **server's** environment, not the pane's, so `$SHANTY_AGENT` exported in a
pane is not visible to the bar. That is why the generated config passes
`#{session_name}` to every per-agent segment — one shared `status-right` over many
sessions, each rendering its own agent.

With no identity resolvable at all, the segments say `⚠ no agent` rather than guess.
Guessing would put another agent's plate on this bar.

### Non-marking reads

The bar polls `inbox --count` and `anchor --events`, which shantytown documents as
reads that mark nothing. Polling them from a status bar can never consume unread
mail or mark an event delivered. The listing and draining forms are never used.

### Environment

| Variable | Effect |
|----------|--------|
| `SHANTY_ST_BIN` | The `st` binary to use. Default: `PATH`, then `~/.local/bin`, `/usr/local/bin`, `/opt/homebrew/bin`, `/usr/bin`. |
| `SHANTY_ST_CWD` | The directory to run `st` from. `st` resolves its tracker by walking up from here, so this decides which store the bar reads. Set it when the tmux server's working directory is not the deployment's tracker root. |
| `SHANTY_SEG_NOCACHE` | Disable the shared on-disk answer cache. |

`shanty apply` copies `SHANTY_ST_BIN` and `SHANTY_ST_CWD` from your shell into the
target tmux server's environment, because that is the environment the segments
actually run in. `st` is also launched with its own directory prepended to `PATH`,
so the tracker CLI installed alongside it resolves even when the server's `PATH`
does not include it.

## Crew marks

Each agent gets one emoji so a wall of panes is identifiable without reading:

```bash
shanty marks
```

Marks are assigned on first sight and **never reassigned**. That is why they are
stored, in `~/.config/shanty/agents.toml`, rather than derived from a hash of the
name: a hash is stable per name, but its collision handling is not stable across
roster changes, so adding one crew member would silently re-badge others and destroy
the recognition the mark exists to provide.

The registry is plain TOML — edit a line to choose your own mark, and assignment will
not overwrite it. Keep them distinct, or two panes will look like the same agent.
`shanty apply` assigns the whole roster in one deterministic pass over the sorted
agent list; a segment meeting an unbadged agent assigns lazily as a safety net.

The palette avoids the glyphs the bar already spends on meaning (⚓ ⚙ ⚠ ✉ ⏱ Σ): a
mark that reads as a status would be worse than no mark. It is finite, and a roster
that outgrows it gets no mark rather than a duplicate one.

## Caching

Every `#(shanty seg …)` is its own short-lived process. External command
answers therefore share an on-disk cache, normally for 15 seconds. Failures
remain failures during that bounded backoff; reading an entry never renews it.

The inbox **display** reuses counts and errors for at most 60 seconds. Its key
includes the owner, working directory, executable identity and routing
environment. A nonblocking process lock permits one refresh per key; other
renders show an unknown state while it runs. Invalid counts, unsupported locks
or an unavailable cache also remain unknown. The next render after expiry
attempts a fresh read. This never lists, acknowledges or drains messages.
`SHANTY_SEG_NOCACHE` bypasses caching for diagnostics.

System segments that read `/proc` are not cached; those reads are fast.
