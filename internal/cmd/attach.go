package cmd

import (
	"github.com/intigraph/shanty/internal/session"
	"github.com/spf13/cobra"
)

var attachReadOnly bool

var attachCmd = &cobra.Command{
	Use:   "attach [name]",
	Short: "Attach to an existing shanty session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr := session.NewManager()
		return mgr.Attach(args[0], attachReadOnly)
	},
}

func init() {
	attachCmd.Flags().BoolVarP(&attachReadOnly, "read-only", "r", false,
		"attach read-only: observe without any keystroke reaching the session")
	rootCmd.AddCommand(attachCmd)
}
