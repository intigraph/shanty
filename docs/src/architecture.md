# Architecture

## Project layout

```
shanty/
  cmd/shanty/main.go          Entry point
  internal/
    cmd/                       Cobra CLI commands
      root.go                  Default command (launch/attach)
      ls.go                    List sessions
      attach.go                Attach to session
      apply.go                 Theme a socket's server without attaching
      seg.go                   Render a status bar segment
      marks.go                 Show each crew member's emoji mark
    config/                    Configuration generation
      theme.go                 Theme loading and defaults
      keys.go                  Keybinding definitions
      status.go                Status bar layout
    session/                   tmux session management
      manager.go               Session lifecycle (create, attach, list)
      tmux.go                  tmux config file generation
    stread/                    The ONE reader for shantytown's st CLI
      st.go                    Locate st, run it, cache the answer on disk
      crew.go                  Parse `st crew` into per-agent role/state/currency
      anchor.go                Read the held item's id and title
      stats.go                 Read one agent's activity numbers
    crewid/                    Per-agent emoji marks
      emoji.go                 Palette, registry file, assign-once semantics
    segments/                  Status bar segment implementations
      segment.go               Segment interface
      registry.go              Segment name registry
      shantytown.go            crewid, task, stats, crew, events, inbox, harness
      session.go               The identity pill
      clock.go                 Time segment
      host.go                  Hostname segment
      cpu.go                   CPU usage segment
      mem.go                   Memory usage segment
      load.go                  Load average segment
      disk.go                  Disk usage segment
      color.go                 Color threshold helper
  themes/
    dracula.toml               Default Dracula theme
```

## Design principles

### Single binary

Shanty is a single Go binary with no runtime dependencies beyond tmux itself. The binary serves two roles:

1. **Session manager** — creates, attaches to, and lists tmux sessions
2. **Segment renderer** — tmux calls `shanty seg <name>` to render status bar segments

### Config generation

Rather than shipping a static tmux config, shanty generates one on each launch. This allows the config to incorporate the current theme, keybindings, and segment list dynamically.

The generated config is written to `~/.config/shanty/tmux.conf`. When attaching to an existing session, shanty regenerates and sources the config so changes take effect immediately.

### Socket isolation

Shanty uses a dedicated tmux socket (`-L shanty`). This means:

- Shanty gets its own tmux server with its own config
- It won't interfere with other tmux servers on the default socket
- The `-f` flag for config is respected (tmux ignores `-f` if a server is already running on the same socket)

### Segment interface

All segments implement a simple interface:

```go
type Segment interface {
    Name() string
    Render() string
}
```

Segments are registered in a global `Registry` map. The `seg` subcommand looks up the segment by name and calls `Render()`.

### Caching

Segments that call external commands share an on-disk cache: each `#(shanty seg …)` runs in a new process. Most st reads reuse successes and failures for 15 seconds. The display-only inbox count uses a 60-second cache and a nonblocking cross-process refresh lock; a competing refresh or unavailable cache renders UNKNOWN, never an empty inbox. A hit does not extend expiry. Inbox entries are isolated by owner, working directory, executable identity and routing environment. Interactive inbox commands are unaffected.
