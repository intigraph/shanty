package cmd

import (
	"github.com/intigraph/shanty/internal/session"
	"github.com/spf13/cobra"
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply shanty's theme + status bar to the current socket's tmux server",
	Long: `Apply shanty's Dracula theme, keybindings, and status bar to every
session on the target tmux server, WITHOUT attaching.

The socket is SHANTY_TMUX_SOCKET (the fleet's socket), same as attach and ls. This
makes "using shanty" a reproducible command rather than hand-typed host state a
tmux restart loses — re-run it after a restart and the themed bar is back:

    SHANTY_TMUX_SOCKET=<fleet-socket> shanty apply`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return session.NewManager().Apply()
	},
}

func init() {
	rootCmd.AddCommand(applyCmd)
}
