package cmd

import (
	"fmt"
	"sort"

	"github.com/intigraph/shanty/internal/crewid"
	"github.com/intigraph/shanty/internal/stread"
	"github.com/spf13/cobra"
)

var marksCmd = &cobra.Command{
	Use:   "marks",
	Short: "Show each crew member's status-bar mark",
	Long: `Show the emoji each crew member is badged with on the status bar.

The marks are what make a wall of panes readable without reading: one glance at
the bar identifies the agent. They are assigned on first sight and NEVER
reassigned, because a mark that moves when the roster changes destroys the
recognition it exists to provide.

The registry is a plain TOML file — the path is printed below. To choose your own
mark for an agent, edit its line; assignment will not overwrite it. Keep them
distinct, or two panes will look like the same agent.

Crew that st knows about but which have no mark yet are assigned one here.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := crewid.Path()
		if err != nil {
			return err
		}

		// Assign for the whole roster first, so `marks` is also the command that
		// repairs a registry missing a newly-created agent.
		var marks map[string]string
		if stread.Installed() {
			if crew, cerr := stread.Crew(); cerr == nil && len(crew) > 0 {
				agents := make([]string, 0, len(crew))
				for name := range crew {
					agents = append(agents, name)
				}
				sort.Strings(agents)
				marks, _ = crewid.Assign(agents)
			}
		}
		if len(marks) == 0 {
			// No st, or st could not name a crew: show what is already stored rather
			// than nothing. An operator asking about marks wants the file's contents.
			fmt.Printf("  (st reported no crew — showing stored marks only)\n\n")
		}

		stored := marks
		if len(stored) == 0 {
			stored = crewid.Stored()
		}
		names := make([]string, 0, len(stored))
		for n := range stored {
			names = append(names, n)
		}
		sort.Strings(names)

		if len(names) == 0 {
			fmt.Println("  no marks assigned yet")
		}
		for _, n := range names {
			fmt.Printf("  %s  %s\n", stored[n], n)
		}
		fmt.Printf("\n  registry: %s\n", path)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(marksCmd)
}
