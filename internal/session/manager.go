package session

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/intigraph/shanty/internal/crewid"
	"github.com/intigraph/shanty/internal/stread"
)

const sessionPrefix = "shanty-"

// socketName is the tmux socket for shanty sessions.
//
// It defaults to a dedicated "shanty" server: using a separate socket (-L)
// ensures shanty gets its own tmux server with its own config, independent of
// any other tmux server (e.g. agent sessions). Without this, tmux -f is
// silently ignored when a server is already running on the default socket.
//
// SHANTY_TMUX_SOCKET overrides it. This lets a caller point shanty at an
// EXISTING fleet socket to VIEW its sessions themed — without migrating any
// session onto shanty's own server. shantytown's `st attach` sets it so an
// operator gets shanty's bar over the fleet's real panes by name; the agents
// never move. Resolved once at startup, which is correct because the socket is
// a launch-time choice, not something that changes mid-process.
var socketName = resolveSocketName()

func resolveSocketName() string {
	if s := os.Getenv("SHANTY_TMUX_SOCKET"); s != "" {
		return s
	}
	return "shanty"
}

// fullName returns the tmux session name with the shanty- prefix.
func fullName(name string) string {
	if strings.HasPrefix(name, sessionPrefix) {
		return name
	}
	return sessionPrefix + name
}

// displayName strips a recognized agent prefix for user-facing output. A name with
// no known prefix is shown as it is.
func displayName(name string) string {
	stripped, _ := stread.StripSessionPrefix(name)
	return stripped
}

// Manager handles shanty tmux session lifecycle.
type Manager struct {
	tmuxBin string
}

// NewManager creates a session manager, locating the tmux binary.
func NewManager() *Manager {
	bin, err := exec.LookPath("tmux")
	if err != nil {
		bin = "tmux"
	}
	return &Manager{tmuxBin: bin}
}

// LaunchOrAttach starts a new session or attaches to an existing one.
// If the session exists, attaches (works as new client if already attached).
// If the session doesn't exist, creates it with generated config.
func (m *Manager) LaunchOrAttach(name string) error {
	full := fullName(name)
	if m.sessionExists(full) {
		return m.attach(full, false)
	}
	return m.create(full)
}

// Attach connects to an existing tmux session by name.
//
// It prefers a session that exists under the EXACT name given, and only applies
// the shanty- prefix when no literal match exists. That ordering is what lets
// shanty view a FOREIGN socket's sessions (SHANTY_TMUX_SOCKET): a fleet pane is
// named by whoever created it — `legacy-worker-3`, `crew-lead` — not by
// shanty's convention, so force-prefixing would miss every session shanty did
// not create. `shanty attach dev` still resolves to `shanty-dev`, because no
// literal `dev` session exists.
func (m *Manager) Attach(name string, readOnly bool) error {
	if m.sessionExists(name) {
		return m.attach(name, readOnly)
	}
	full := fullName(name)
	if !m.sessionExists(full) {
		return fmt.Errorf(
			"session %q not found on socket %q (looked for %q and %q)",
			name, socketName, name, full)
	}
	return m.attach(full, readOnly)
}

// List returns all shanty-managed tmux sessions (shanty- prefix stripped).
func (m *Manager) List() ([]string, error) {
	cmd := exec.Command(m.tmuxBin, "-L", socketName, "list-sessions", "-F", "#{session_name}")
	out, err := cmd.Output()
	if err != nil {
		// tmux returns error when no server is running
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var sessions []string
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Both shanty's own prefix and shantytown's `st-` count. Filtering to
		// `shanty-` alone made `shanty ls` report "No active sessions" on a fleet of
		// a dozen live agent panes — the picker's own version of the blank status
		// bar, from the same single-prefix assumption.
		if stread.IsAgentSession(line) {
			sessions = append(sessions, displayName(line))
		}
	}
	return sessions, nil
}

// attach connects to a tmux session by its full (prefixed) name.
// It regenerates and sources the shanty config so that existing sessions
// pick up theme, keybindings, and status bar changes.
func (m *Manager) attach(fullSessionName string, readOnly bool) error {
	// Regenerate config and source it into the shanty server so that
	// prefix, keybindings, theme, and status bar are always applied.
	if confPath, err := GenerateConfig(); err == nil {
		_ = exec.Command(m.tmuxBin, "-L", socketName, "source-file", confPath).Run()
	}

	args := []string{"-L", socketName, "attach-session", "-t", fullSessionName}
	if readOnly {
		// -r: a read-only client. No keystroke reaches the session — the
		// observe-without-touching mode `st attach -r` needs.
		args = append(args, "-r")
	}
	cmd := exec.Command(m.tmuxBin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (m *Manager) sessionExists(fullSessionName string) bool {
	cmd := exec.Command(m.tmuxBin, "-L", socketName, "has-session", "-t", fullSessionName)
	return cmd.Run() == nil
}

// Apply generates shanty's config and sources it into the socket's tmux server,
// theming its sessions WITHOUT attaching.
//
// This makes "using shanty" a REPRODUCIBLE command rather than hand-typed host
// state a tmux server restart loses. Pointed at a fleet
// socket via SHANTY_TMUX_SOCKET, `shanty apply` puts the Dracula bar + segments
// on every session at once and can be re-run after a restart, so no operator ever
// types the `tmux -L <sock> set -g status-...` incantation. `attach` already
// sources the same config on the way in, so an attach and an apply agree.
func (m *Manager) Apply() error {
	confPath, err := GenerateConfig()
	if err != nil {
		return fmt.Errorf("generating tmux config: %w", err)
	}
	// No server on this socket = nothing to theme. Say so rather than let
	// source-file fail with a raw tmux error; a themed server with no sessions is
	// not what the operator meant.
	if exec.Command(m.tmuxBin, "-L", socketName, "list-sessions").Run() != nil {
		return fmt.Errorf(
			"no tmux server on socket %q — nothing to theme "+
				"(set SHANTY_TMUX_SOCKET to the fleet's socket)", socketName)
	}
	if out, err := exec.Command(
		m.tmuxBin, "-L", socketName, "source-file", confPath).CombinedOutput(); err != nil {
		return fmt.Errorf("sourcing shanty config into %q: %v: %s",
			socketName, err, strings.TrimSpace(string(out)))
	}
	m.exportSegmentEnv()
	assignCrewMarks()
	return nil
}

// segmentEnv are the variables the status segments read which must exist in the
// tmux SERVER's environment, not just the operator's shell.
//
// This is the trap they exist to close. tmux runs `#(...)` status commands from the
// server's environment, so an operator who exports SHANTY_ST_CWD in their shell and
// runs `shanty apply` gets a bar that still cannot find the tracker — and the
// symptom is a blank segment, which looks like "nothing to report". `apply` is the
// command that knows both values and the target server, so it is the right place to
// carry them across.
// SHANTY_ROOT is here because st needs BOTH halves and they are different
// directories: the root is where st finds WHO the crew are, and the working
// directory decides which tracker store it reads. Propagating only the second
// produced a bar that knew a session was `st-<agent>` and then could not confirm the
// agent exists — "no such agent", on a fleet where they plainly do.
var segmentEnv = []string{"SHANTY_ST_BIN", "SHANTY_ST_CWD", "SHANTY_ROOT"}

// exportSegmentEnv copies the segment configuration this process was given into the
// target server. Only variables actually set are copied — writing an empty value
// would override a server that was already configured correctly.
//
// Failure is silent: the theming apply came for has already succeeded, and a server
// that refuses an environment write still renders a bar (a loud one, saying it
// cannot reach the tracker — which is the honest outcome).
func (m *Manager) exportSegmentEnv() {
	for _, key := range segmentEnv {
		val := os.Getenv(key)
		if val == "" {
			continue
		}
		_ = exec.Command(m.tmuxBin, "-L", socketName,
			"set-environment", "-g", key, val).Run()
	}
}

// assignCrewMarks gives every crew member st knows about its display mark, here
// rather than lazily at render time.
//
// Doing it in one pass over the WHOLE sorted roster is what makes the assignment
// deterministic from the roster rather than from the order panes happened to
// redraw in. Lazy assignment in the segment remains as a safety net for an agent
// created after this ran.
//
// Failure is silent on purpose: marks are cosmetic, and `apply`'s job — theming
// the server — has already succeeded by this point. Refusing the whole apply
// because an emoji file could not be written would be a worse trade.
func assignCrewMarks() {
	if !stread.Installed() {
		return
	}
	crew, err := stread.Crew()
	if err != nil || len(crew) == 0 {
		return
	}
	agents := make([]string, 0, len(crew))
	for name := range crew {
		agents = append(agents, name)
	}
	sort.Strings(agents)
	_, _ = crewid.Assign(agents)
}

func (m *Manager) create(fullSessionName string) error {
	confPath, err := GenerateConfig()
	if err != nil {
		return fmt.Errorf("generating tmux config: %w", err)
	}

	cmd := exec.Command(m.tmuxBin, "-L", socketName, "-f", confPath, "new-session", "-s", fullSessionName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
