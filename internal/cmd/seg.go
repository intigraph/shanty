package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/intigraph/shanty/internal/segments"
	"github.com/spf13/cobra"
)

var segCmd = &cobra.Command{
	Use:   "seg <name>",
	Short: "Render a status bar segment",
	Long: `Render a single status bar segment for tmux.

tmux calls this via #(shanty seg <name> [session]) at status-interval.
Each segment outputs a short formatted string with optional tmux color codes.

System segments: clock, host, cpu, mem, load, disk

shantytown segments, which read the st CLI:
  crewid   who this pane is — mark, name, role, and st's busy/idle verdict
  task     what they hold — item id and title
  stats    what they did — activity, files, input/output tokens, cache hit rate
  crew     the fleet's busy/total count
  events   undelivered stop events addressed to this agent
  inbox    unread messages
  harness  the agent runtime's name
  anchor   the held item's id alone (superseded by task)

Identity comes from $SHANTY_AGENT, else from the session name passed as the
second argument (shanty-<agent> or st-<agent>). tmux runs status commands from
the SERVER's environment, not the pane's, so on a fleet the session name is
normally what identifies the pane — pass #{session_name}.

Failure is VISIBLE. A segment renders empty only when no st binary exists at
all, so a host without shantytown sees the plain bar. When st IS present and
cannot answer — no identity derivable, a failing call, or a plate st will not
name for an agent it calls busy — the segment renders a ⚠ marker instead. A
status bar that blanks when it breaks reads as "nothing to report".

Environment:
  SHANTY_ST_BIN   the st binary to use (default: PATH, then ~/.local/bin etc.)
  SHANTY_ST_CWD   the directory to run st from. st resolves its tracker by
                  walking up from here, so this decides which store it reads.
  SHANTY_SEG_NOCACHE  disable the shared on-disk answer cache.

See https://github.com/scbrown/shantytown`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Optional second arg: the tmux session this render is for, passed from
		// #{session_name}. It is the per-pane identity fallback for the shared
		// fleet bar — a segment learns which agent it draws from the session it
		// is drawn in when $SHANTY_AGENT is not exported per pane.
		if len(args) == 2 {
			segments.SetSession(args[1])
		}

		if name == "list" {
			for _, n := range segments.AllNames() {
				fmt.Println(n)
			}
			return nil
		}

		seg, ok := segments.Registry[name]
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown segment: %s\nAvailable: %s\n",
				name, strings.Join(segments.AllNames(), ", "))
			return fmt.Errorf("unknown segment: %s", name)
		}

		result := seg.Render()
		if result != "" {
			fmt.Print(result)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(segCmd)
}
