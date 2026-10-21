# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Move the Go module, build linker paths, and installation links to
  `github.com/intigraph/shanty`. Previously published module tags stay unchanged.

### Fixed

- Failed shantytown status reads back off for 15 seconds while continuing to
  report an error. Repeated bar renders no longer launch a new failed command
  on every refresh; expiry retries and a successful read clears the failure.

## [0.4.2] - 2026-09-30

### Changed

- The `usage` segment reads st's versioned JSON (`st crew --governor --json`,
  version 1) instead of its prose, so the next change to st's prose cannot
  silently blind the bar again. An unknown schema version renders a loud
  `usage schema vN unsupported`; an st without the bar's fields falls back to
  the prose parser (aegis-apfuey).

## [0.4.1] - 2026-09-30

### Fixed

- The `usage` segment reads st's per-lane governor output again. `st crew
  --governor` moved from one line (`ok 45/50 24/45`) to one line per lane
  (`base ok 34/70/1644 ...`, then codex, balance and a roster), so the first
  token became the lane name and the bar showed `⚠ usage ?` on every refresh
  while st exited 0. The bar now shows the `base` (Claude) lane, accepts
  `now/next/reset` triples, and reads the tier from after `freshest[...]`; the
  codex lane's dispatch floor does not leak onto it (aegis-apfuey).

- The `cpu` and `mem` segments report real numbers on macOS. Both read `/proc`,
  which a BSD userland does not have, so on a Mac the bar permanently showed
  `cpu n/a mem n/a` — asking a kernel that does not answer and rendering the
  silence as though the machine had declined to say. Reading now lives per
  platform in `sysread.go`; Darwin takes cpu from the second `top -l 2` sample
  (the first is an average since boot, flat on a machine up for days) and memory
  from `hw.memsize` plus `vm_stat`, counting active + wired + compressor-occupied
  — what Activity Monitor calls "Memory Used". `n/a` survives for a platform
  neither reader understands, but it now means "I asked and got nothing" rather
  than "I only know how to ask Linux".

- The `crewid` segment no longer repeats the agent name and mark. The session
  pill on the left of the bar already renders `<mark> <agent>`, so a one-agent
  pane read `🦊 hammond … 🦊 hammond ⚠ st?` — spending the scarcest thing a
  status bar has on what the eye had already read at the other end. This is a
  deduplication, not a removal: the role, st's verdict, the evidence behind it
  and the stale-settings flag all stay, because none of those is said anywhere
  else, and the mark is still assigned here when an agent has none.

- `st crew` output parses again. Three drifts in st's table each dropped every
  row on their own: a new HOST column moved the agent out of the first field
  (so both Mac rows landed under one key and the second overwrote the first),
  stacked roles like `worker,normal` no longer matched the bare word `worker`,
  and a `?` in the TREE column was read as the WORK verdict from the column next
  door — turning "tree unverified" into the louder and different claim that st
  could not tell. The parse now anchors on the role cell and takes the agent as
  the field before it, reading the old and new shapes without being told which
  is which, and a bare `?` counts only when no real verdict appears on the row.

- The `shantytown` segment asks `st agent stats` instead of the retired `st
  stats` spelling, which survives today only behind a deprecation alias that
  warns on stderr and says it goes away in two releases (aegis st-1).

## [0.4.0] - 2026-09-11

### Added

- The `shantytown` segment now splits token usage into prompt and completion
  traffic and reports a cache hit rate — `in12k out3k cache87%` where it
  previously showed a single `15ktok`. The split is the number you act on: a
  high `in` with a low cache rate is the expensive shape, and one aggregate
  figure cannot tell that apart from a long completion. The rate is cache reads
  over total prompt traffic, so it answers "how much of what I sent did I pay
  for again".

  The segment stays backward-compatible with an older `st` writer: when the
  transcript-backed dimensions are absent from the wire it still renders the
  requested `in`/`out` split from the counts it does have, rather than falling
  back to the old aggregate. Only the cache rate needs the newer writer, and it
  is omitted rather than shown as zero when unmeasured — a rate of `0%` and a
  rate nobody measured are different facts.

### Changed

- The system-metric segments now label what they are showing: `cpu 23%`,
  `mem 61%`, `load 1.4`, `disk 88%`. Four bare numbers in a row is a puzzle the
  reader has to solve from position alone, and position changes with
  configuration.

- The `harness` segment is silent when the agent is running the deployment's
  configured default, and names the program only when it is *not* the default.
  A status bar that reports the expected state on every pane spends attention
  without giving information; the interesting case is the exception. An
  unreadable harness or configuration stays loud — this quiets the boring
  answer, never the unknown one.

## [0.3.0] - 2026-08-02

### Added

- `ctrl-a s` is now a fuzzy session switcher (`shanty pick`) in a popup, replacing
  tmux's stock `choose-tree`. It filters from the first keystroke, and the session
  you last switched *away from* is the top row under the cursor, so `ctrl-a s
  Enter` is a toggle back. Ordering is by `session_last_attached` — when you were
  last *in* a session — deliberately not `choose-tree -O time`, which sorts by
  *activity* and on a busy server ranks whichever session most recently printed
  output rather than the one you came from. The current session is not offered.
  Uses fzf; without it the key falls back to `choose-tree -Zs`, checked when the
  key is pressed rather than frozen at config-generation time.
- `crewid` segment — who a pane belongs to: the agent's mark, name, role, and
  shantytown's own busy/idle verdict, plus a flag when st reports the agent is
  running settings older than the file on disk
- `task` segment — the held item's id *and* its title, so a pane says what the
  agent is working on rather than only which ticket number
- `stats` segment — activity, files touched and token traffic from `st stats`
- The verdict now carries its EVIDENCE: `busy (2 shells live)`,
  `saturated (948k ctx)`, `waiting (blocked on a question)`. st packs those facts
  into its work cell and shanty was rendering the raw notation; decoded, a
  coordinator can weigh the bar against a competing idle signal instead of having to
  believe or ignore a bare word.
- `idle+1sh` — st's own notation for "idle AND carrying live work" — is no longer
  painted the calm green of a free agent. An agent whose turn ended with a build or a
  `gh run watch` still running is not available, and `idle` is exactly the word that
  gets it dispatched over. The administrator's dispatch summary counts those
  separately from free for the same reason.
- Per-crew emoji marks, assigned on first sight and never reassigned, stored in
  `~/.config/shanty/agents.toml`; `shanty marks` lists them and `shanty apply`
  assigns the whole roster in one deterministic pass
- `SHANTY_ST_BIN`, `SHANTY_ST_CWD`, `SHANTY_SEG_NOCACHE` for pointing the
  shantytown segments at the right binary, the right tracker, and around the cache

### Fixed

- **The bar permanently reported the fleet coordinator as a defect.** An
  `administrator` never holds an implementation item — the role exists to stay free
  to coordinate — so the busy-with-no-item warning was known-false every time it was
  drawn for them, and a status surface carrying one permanent false red teaches its
  reader to discount every red on it. For that role the item slot now shows dispatch
  state instead: `⚑ 2 free · 1 need eyes`, or `⚑ crew fed`. Keyed on the ROLE, never
  on a name, so whoever holds it next inherits the same treatment. For a worker the
  warning stays loud — there it is real signal.
- **The whole status bar could render as a blank line.** tmux runs `#(...)` status
  commands from the tmux *server's* environment; a server started outside a login
  shell has no `~/.local/bin` on `PATH`, so every `#(shanty seg …)` failed to exec
  and the entire status-right came out as spaces. The generated config now names
  shanty by absolute path, and the segments locate `st` past `PATH` as well.
- **Per-agent segments were blank for every shantytown fleet pane.** Identity was
  derived by stripping only shanty's own `shanty-` prefix, but a fleet launched by
  `st` names its sessions `st-<agent>`. Both prefixes are now recognized.
- **The left pill said `shanty` on every pane.** It queried shanty's own socket for
  the session name, which cannot work when shanty is pointed at a foreign socket
  (`SHANTY_TMUX_SOCKET`); it now uses the session name tmux passes in, and shows
  the agent's mark and name.
- **st's summary prose was parsed as crew.** `st crew` follows its table with lines
  like `9 busy: bond, felix`, whose second field is verdict-shaped — the session
  picker read that as an agent named `9`. A row must now carry a role as well.
- Segments no longer collapse failure to an empty string. Empty is reserved for
  "no `st` installed"; anything else renders a visible `⚠`, including the
  empty-plate-for-a-busy-agent contradiction that a naive `anchor --short` read
  renders as a convincing blank. That state reads `busy, no item` and deliberately
  names no cause: an unreachable tracker and genuinely untracked work both produce
  it, and asserting a diagnosis we have not established is the same species of
  error as rendering blank.
- The segment cache was process-local and therefore never hit — each `shanty seg`
  invocation is its own process. It is now shared on disk.
- **`st` could not reach its own companions.** `st` shells out to its tracker's CLI,
  installed alongside it; under a tmux server with a narrow `PATH` that failed with
  `bd list failed: No such file or directory: 'bd'` even though `st` itself ran.
  `st` is now launched with its own directory prepended to `PATH`.
- **`shanty ls` reported "No active sessions" on a live fleet.** Sessions were
  filtered to shanty's own `shanty-` prefix, so a dozen `st-<agent>` panes were
  invisible — the session picker's version of the blank status bar, from the same
  single-prefix assumption. The prefix list now lives in one place.
- `shanty apply` copies `SHANTY_ST_BIN`/`SHANTY_ST_CWD` into the target tmux
  server's environment. Setting them in a shell and applying used to leave the bar
  unable to reach the tracker, with a blank segment as the only symptom.

### Changed

- Default status bar: `crewid task events inbox crew stats harness cpu mem host
  clock`, with the right-hand budget raised to 200 cells so the identity, title
  and stats fields are not silently truncated. `anchor` remains available but is
  superseded by `task`.

## [0.2.0] - 2026-07-22

### Added

- `shanty apply` — apply the theme + status bar to the current socket's server
  without attaching; re-runnable after a server restart
- `shanty attach -r/--read-only` — observe a session without keystrokes reaching it
- `SHANTY_TMUX_SOCKET` — point shanty at an existing tmux server/socket instead
  of its own dedicated `-L shanty` socket
- Shantytown segments (anchor, crew, events, inbox, harness) included in the
  default status bar; each self-hides when the `st` CLI or agent identity is absent
- Per-agent segments in a shared bar: segments derive their agent from the
  session name (`shanty-weaver` → `weaver`); `$SHANTY_AGENT` still wins when set
- `shanty ls` is a crew-oriented session selector when shantytown is present:
  agent name, work state, held item, sorted attention-first; falls back to the
  plain session list without `st` or with `--plain`

### Changed

- `attach` prefers a session that exists under the exact name given, applying
  the `shanty-` prefix only when no literal match exists

## [0.1.0] - 2026-02-18

### Added

- Core tmux wrapper with Dracula theme
- ctrl-a prefix key (byobu convention)
- Byobu-compatible F-key bindings (F2-F8)
- Session management: launch, attach, list, named sessions
- Dedicated tmux socket (`-L shanty`) for session isolation
- Pluggable status bar segments via `shanty seg <name>`
- System segments: session, clock, host, cpu, mem, load, disk
- Color-coded resource segments (green/orange/red thresholds)
- 30-second segment cache to reduce overhead
- Custom theme support via TOML files
- Config generation at `~/.config/shanty/tmux.conf`
- `shanty seg list` to discover available segments
- MIT license

[0.4.0]: https://github.com/scbrown/shanty/releases/tag/v0.4.0
[0.3.0]: https://github.com/scbrown/shanty/releases/tag/v0.3.0
[0.2.0]: https://github.com/scbrown/shanty/releases/tag/v0.2.0
[0.1.0]: https://github.com/scbrown/shanty/releases/tag/v0.1.0
