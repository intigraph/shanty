# Configuration

Shanty is designed to work with zero configuration. When you run `shanty`, it generates a tmux config at `~/.config/shanty/tmux.conf` and uses it automatically.

## Config location

Shanty follows the XDG Base Directory Specification:

- Config directory: `$XDG_CONFIG_HOME/shanty/` (default: `~/.config/shanty/`)
- User choices: `~/.config/shanty/config.toml`
- Generated tmux config: `~/.config/shanty/tmux.conf`
- Custom theme: `~/.config/shanty/themes/dracula.toml`

## How config generation works

Each time shanty launches or attaches to a session, it:

1. Loads the theme (custom file or built-in Dracula)
2. Generates a tmux config with keybindings, theme colors, and status bar
3. Writes it to `~/.config/shanty/tmux.conf`
4. Tells tmux to use this config

This means changes to your theme file take effect on the next attach or launch — no restart needed.

## Session names

Sessions are prefixed with `shanty-` internally to avoid collisions with other tmux sessions:

```bash
shanty              # creates tmux session "shanty-main"
shanty -s work      # creates tmux session "shanty-work"
shanty ls           # shows "main", "work" (prefix stripped)
```

## tmux socket

Shanty uses a dedicated tmux socket (`-L shanty`). This means:

- Shanty sessions are isolated from your regular tmux sessions
- Shanty's config doesn't affect other tmux servers
- You can run shanty alongside your existing tmux setup


## Persistent status segments

Put segment choices in `~/.config/shanty/config.toml` (or
`$XDG_CONFIG_HOME/shanty/config.toml`), not in generated `tmux.conf`:

```toml
[status]
left = ["session", "crewid"]
right = ["task", "events", "inbox", "crew", "usage", "harness", "cpu", "mem", "host", "clock"]
```

This layout omits `stats`. Every launch, attach and `shanty apply` reloads these
choices; regeneration preserves this file. A missing side keeps its default;
an explicit `left = []` or `right = []` disables that side. Available names are
listed by `shanty seg list`. Unknown keys/names, duplicate non-identity segments,
and malformed TOML return an error before replacing the generated config.

When `crewid` is selected on either side, it is rendered once, **first on the
left**, followed by the other left segments. Its role therefore survives tmux's
left-clipping of a long right-hand bar. The left maximum is 60 cells; the right
maximum remains 200. Other segment order is preserved. Explicitly omitting
`crewid` from both lists hides it; selecting no segments leaves a blank status
area rather than restoring defaults. Very narrow terminals can still clip
content; the protected role was tested with a 170-column client.

After editing, use `shanty apply` to regenerate and apply to the selected socket,
or relaunch/attach normally. F5 reloads the already generated file; it does not
regenerate it. To restore defaults, remove the `[status]` overrides or the user
config file and regenerate. This feature does not change the system default
segment set or install/update Shanty on another host.
