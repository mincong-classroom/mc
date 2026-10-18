package git

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/mincong-classroom/mc/common"
	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull latest changes for all teams",
	Long: `Pull latest changes for all teams, as fast-forwards only: the history of a team is never
merged into the local one. When a team rewrote the history of its branch, e.g. with a
force-push, its repository is not pulled: a warning says so, and the other teams are pulled.`,
	Run: runPull,
	Example: `
  Pull latest changes for all teams:
  mc git pull`,
}

func runPull(cmd *cobra.Command, args []string) {
	teams, _ := common.ListTeams()
	for _, team := range teams {
		targetDir := team.GetRepoPath()

		// advice.diverging=false: the hints of a rewritten history suggest to merge or rebase it.
		cmd := exec.Command("git", "--no-pager", "-C", targetDir, "-c", "advice.diverging=false", "pull", "--ff-only")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		fmt.Printf("\033[1m[team: %s] Pulling latest changes for %q\033[0m\n", team.Name, team.GetRepoURL())
		err := cmd.Run()
		if err == nil {
			continue
		}
		if historyRewritten(targetDir) {
			fmt.Printf("\033[1;33m[team: %s] ⚠ history rewritten: force-push? The local branch is no longer an ancestor of the remote one: not pulled\033[0m\n", team.Name)
			continue
		}
		log.Printf("Failed to pull latest changes for team %s: %v", team.Name, err)
	}
}

// historyRewritten reports whether the current branch of the repository in dir is no longer an
// ancestor of its upstream, as just fetched by git pull: the remote history was rewritten, so it
// cannot be fast-forwarded.
func historyRewritten(dir string) bool {
	err := exec.Command("git", "-C", dir, "merge-base", "--is-ancestor", "HEAD", "@{upstream}").Run()
	// It exits with 1 when HEAD is not an ancestor, and with another code on an error, such as a
	// branch without an upstream.
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 1
}
