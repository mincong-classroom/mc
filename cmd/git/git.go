package git

import (
	"github.com/spf13/cobra"
)

var GitCmd = &cobra.Command{
	Use:   "git",
	Short: "Perform Git operations in bulk",
	Long:  `Perform Git operations in builk for all the teams in the classroom, based on the team definition file.`,
}

func Execute() error {
	return GitCmd.Execute()
}

func init() {
	GitCmd.AddCommand(cloneCmd)
}
