package cmd

import (
	"fmt"
	"os"

	"github.com/intigraph/shanty/internal/session"
	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

var sessionName string

var rootCmd = &cobra.Command{
	Use:   "shanty",
	Short: "Shanty - terminal multiplexer with Dracula theme",
	Long: `Shanty is a tmux wrapper with a Dracula theme, byobu-style keybindings,
and a pluggable status bar, plus optional shantytown status segments.
Single binary, zero configuration.`,
	Version: Version,
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr := session.NewManager()
		return mgr.LaunchOrAttach(sessionName)
	},
}

func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&sessionName, "session", "s", "main", "session name")
}
