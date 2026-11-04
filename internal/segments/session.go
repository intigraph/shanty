package segments

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/intigraph/shanty/internal/crewid"
	"github.com/intigraph/shanty/internal/stread"
)

// Session renders WHO this pane belongs to as the bar's leftmost pill: the crew
// member's mark and name.
//
// Called by tmux via #(shanty seg session #{session_name}). The session name is
// passed in for the same reason the per-agent segments get it — and because
// querying tmux for it cannot work on a foreign socket: the query used to be
// hardcoded to shanty's OWN socket, so on a fleet socket it failed and every pane
// fell back to rendering the literal word "shanty". A bar that labels twelve
// different agents identically is worse than one with no label.
type Session struct{}

func (s Session) Name() string {
	return "session"
}

func (s Session) Render() string {
	name := sessionName
	if name == "" {
		name = tmuxSessionName()
	}
	if name == "" {
		name = "shanty"
	}
	display, _ := stread.StripSessionPrefix(name)
	if mark := crewid.EmojiFor(display); mark != "" {
		display = mark + " " + display
	}
	return fmt.Sprintf("#[fg=#282a36,bg=#bd93f9,bold] %s #[default]", display)
}

// tmuxSessionName queries tmux for the current session name — the FALLBACK for
// when the seg command was invoked without #{session_name} (an older applied
// bar, or a hand-run `shanty seg session`).
//
// The socket must be the one shanty was pointed at (SHANTY_TMUX_SOCKET), NOT
// hardcoded to shanty's own: `st attach` aims shanty at the fleet's socket, and
// a query against the wrong server does not answer wrongly — it fails.
//
// That is not hypothetical, it is the bug 03b8b49 fixed. The fleet runs on its
// own socket (shantytown declares it — [tmux] socket in shantytown.toml, e.g.
// "gt-ae5f35"), so `tmux -L shanty` failed for every pane, Render fell through
// to the literal "shanty", and every agent's bar showed the brand instead of
// its name. Nothing errored; the fallback string is a plausible-looking label,
// which is why it read as a styling choice rather than a broken lookup.
//
// (This branch and main found the same fault independently and wrote the same
// fix — the two comments are merged here rather than one overwriting the other,
// because each recorded a different half: the trigger and the measured symptom.)
func tmuxSessionName() string {
	socket := os.Getenv("SHANTY_TMUX_SOCKET")
	if socket == "" {
		socket = "shanty"
	}
	out, err := exec.Command("tmux", "-L", socket, "display-message", "-p", "#{session_name}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
