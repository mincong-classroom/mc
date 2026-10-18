package git

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/mincong-classroom/mc/common"
	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec -- <git arguments>",
	Short: "Run a git command in the repositories of all teams",
	Long: `Run a git command in the repository of each team, cloned by "mc git clone", and print the
name of the team before its output. The arguments after "--" are passed to git as they are. A team
whose command fails does not stop the others: the command fails at the end, listing these teams.`,
	Args:         cobra.MinimumNArgs(1),
	SilenceUsage: true,
	RunE:         runExec,
	Example: `
  Show the last commit of each team:
  mc git exec -- log -1 --oneline

  Show the uncommitted changes of each team:
  mc git exec -- status --short`,
}

func runExec(cmd *cobra.Command, args []string) error {
	teams, err := common.ListTeams()
	if err != nil {
		return err
	}

	var failures []string
	for _, team := range teams {
		gitCmd := exec.Command("git", append([]string{"--no-pager", "-C", team.GetRepoPath()}, args...)...)
		gitCmd.Stdout = cmd.OutOrStdout()
		gitCmd.Stderr = cmd.ErrOrStderr()

		fmt.Fprintf(cmd.OutOrStdout(), "\033[1m[team: %s] %s\033[0m\n", team.Name, gitCmd.String())
		if err := gitCmd.Run(); err != nil {
			failures = append(failures, fmt.Sprintf("%s (%v)", team.Name, err))
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	if len(failures) > 0 {
		return fmt.Errorf("git failed for %d of %d teams: %s", len(failures), len(teams), strings.Join(failures, ", "))
	}
	return nil
}
