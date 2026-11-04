package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StatusBarConfig holds the status bar layout.
type StatusBarConfig struct {
	Left  []string `toml:"left"`
	Right []string `toml:"right"`
}

// DefaultStatusBar returns the default status bar segment layout.
//
// The session pill names the crew member. RenderStatusBar protects the role and
// verdict by placing a selected crewid first on the left, before that pill.
//
// The right side leads with the shantytown segments because they are the ones
// that want you to act — who this is and what state they are in, what they hold,
// what is waiting on them, then what they have actually been doing. CPU, memory,
// host and clock are ambient and sit furthest out.
//
// They are included unconditionally, which is safe and deliberate. Every one
// self-hides when no `st` binary exists at all, so a user who does not run
// shantytown sees exactly the old bar and pays one exec per segment per interval
// to learn nothing changed. The alternative — probing for `st` here and building a
// different default — would make the bar's contents depend on installation order,
// which is worse than a cheap no-op.
func DefaultStatusBar() StatusBarConfig {
	return StatusBarConfig{
		Left:  []string{"session"},
		Right: []string{"crewid", "task", "events", "inbox", "crew", "usage", "stats", "harness", "cpu", "mem", "host", "clock"},
	}
}

// StatusRightLength is the cell budget for the right side. The identity, plate
// title and stats segments together need substantially more room than the old
// count-only bar did; a budget that clips them would hide the very fields this
// layout exists to show, and tmux truncates silently.
const StatusRightLength = 200

// RenderStatusBar generates tmux status bar configuration.
// All segments are rendered by calling `shanty seg <name>` at status-interval.
func RenderStatusBar(theme Theme, cfg StatusBarConfig) string {
	var out string

	out += "# Status bar\n"
	out += "set-option -g status on\n"
	out += "set-option -g status-interval 5\n"
	out += fmt.Sprintf("set-option -g status-style 'bg=%s,fg=%s'\n", theme.BG, theme.FG)

	// tmux clips status-right from the left. Keep the role/verdict at the
	// beginning of status-left whenever crewid is selected, ahead of even a
	// long session name. Other user-selected segments keep their order.
	leftNames, rightNames := protectCrewID(cfg)
	left := renderSegmentCalls(leftNames)
	out += fmt.Sprintf("set-option -g status-left '%s '\n", left)
	out += "set-option -g status-left-length 60\n"

	// Right status — dynamic segments via shanty seg
	right := renderSegmentCalls(rightNames)
	out += fmt.Sprintf("set-option -g status-right '%s '\n", right)
	out += fmt.Sprintf("set-option -g status-right-length %d\n", StatusRightLength)

	// Window status
	out += fmt.Sprintf("set-option -g window-status-current-style 'fg=%s,bg=%s,bold'\n",
		theme.BG, theme.Highlight)
	out += fmt.Sprintf("set-option -g window-status-style 'fg=%s,bg=%s'\n", theme.FG, theme.StatusBG)

	return out
}

// protectCrewID works on copies: rendering must not mutate caller choices.
func protectCrewID(cfg StatusBarConfig) ([]string, []string) {
	left, right := []string{}, []string{}
	hasCrewID := false
	for _, name := range cfg.Left {
		if name == "crewid" {
			hasCrewID = true
		} else {
			left = append(left, name)
		}
	}
	for _, name := range cfg.Right {
		if name == "crewid" {
			hasCrewID = true
		} else {
			right = append(right, name)
		}
	}
	if hasCrewID {
		left = append([]string{"crewid"}, left...)
	}
	return left, right
}

// perAgentSegments read a shantytown identity ($SHANTY_AGENT, else the session).
// They get #{session_name} passed so the SHARED bar renders each pane its own —
// the fleet runs one status-right over many sessions. The others (crew, cpu,
// clock, …) are fleet- or host-wide and need no session.
var perAgentSegments = map[string]bool{
	"crewid": true, "task": true, "stats": true,
	"anchor": true, "events": true, "inbox": true, "harness": true,
	// `session` renders WHICH pane this is, so on a shared bar it is per-agent
	// in exactly the same way. It was omitted here and left querying tmux for
	// its own name over a hardcoded socket — the wrong half of the same
	// mechanism this map exists to express. tmux already knows the session it
	// is drawing; passing it is strictly more robust than asking back.
	//
	// This branch and main added this key INDEPENDENTLY, and git auto-merged
	// both copies into a duplicate that only the compiler caught — status.go
	// reported no conflict. Keep exactly one entry.
	"session": true,
}

// selfPath is the absolute path of the running shanty binary, written into the
// generated config instead of the bare name.
//
// This is the difference between a bar and a blank line. tmux runs `#(...)`
// status commands from the SERVER's environment, and a tmux server started
// outside a login shell routinely has a narrower PATH than the user — no
// ~/.local/bin. `#(shanty seg …)` then fails to exec for EVERY segment and tmux
// renders the lot as empty, which reads as a bar with nothing to say. Writing the
// absolute path of the binary that generated the config takes PATH out of the
// equation: the server runs the same shanty the operator ran.
//
// A bare "shanty" is the fallback when our own path is unknowable, which is no
// worse than the old behaviour.
func selfPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "shanty"
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	// A path containing a space or a quote would break the single-quoted tmux
	// option being generated. Rather than invent an escaping scheme for a case
	// that does not arise in practice, fall back to the name and let PATH decide.
	if strings.ContainsAny(exe, " '\"") {
		return "shanty"
	}
	return exe
}

// renderSegmentCalls builds tmux format strings that invoke shanty seg for each
// segment. A per-agent segment is passed #{session_name}, which tmux expands to
// the session being drawn before running the command (verified: the arg arrives
// as the literal session name), so the segment can derive its own agent.
func renderSegmentCalls(names []string) string {
	bin := selfPath()
	var parts []string
	for _, name := range names {
		if perAgentSegments[name] {
			parts = append(parts, fmt.Sprintf("#(%s seg %s #{session_name})", bin, name))
		} else {
			parts = append(parts, fmt.Sprintf("#(%s seg %s)", bin, name))
		}
	}
	return strings.Join(parts, " ")
}
