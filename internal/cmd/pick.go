package cmd

import (
	"fmt"

	"github.com/intigraph/shanty/internal/session"
	"github.com/spf13/cobra"
)

var pickList bool

var pickCmd = &cobra.Command{
	Use:   "pick",
	Short: "Fuzzy-switch sessions, most recently used first",
	Long: `Switch sessions from a fuzzy-filtering list. Type to narrow it; Enter picks.

This is what prefix+s runs. The list is ordered by when you were last IN each
session, with the session you just switched away from at the top — so prefix+s
then Enter is a toggle back. The session you are currently in is not offered.

It is meant to be run inside a tmux popup, where $TMUX identifies both the server
and the client to switch; the generated shanty config binds it that way. Run by
hand with --list to see the ordering without switching anything.

Requires fzf. Without it, prefix+s falls back to tmux's own choose-tree.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		m := session.NewManager()
		if pickList {
			cands, err := m.Candidates()
			if err != nil {
				return err
			}
			for _, c := range cands {
				mark := " "
				if c.IsLast {
					mark = "*"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", mark, c.Name)
			}
			return nil
		}
		return m.Pick()
	},
}

func init() {
	pickCmd.Flags().BoolVar(&pickList, "list", false,
		"print the ordered candidates instead of switching")
	rootCmd.AddCommand(pickCmd)
}
