package git

import (
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
	Run:   runPull,
	Example: `
  Pull latest changes for all teams:
  mc git pull
`,
}

func runPull(cmd *cobra.Command, args []string) {
	teams, _ := common.ListTeams()
	for _, team := range teams {
		targetDir := fmt.Sprintf("/Users/mincong/github/mincong-classroom/%s", team.GetLocalRepoDirName())

		cmd := exec.Command("git", "--no-pager", "-C", targetDir, "pull")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		fmt.Printf("\033[1m[team: %s] Pulling latest changes for %q\033[0m\n", team.Name, team.GetRepoURL())
		err := cmd.Run()

		if err != nil {
			log.Printf("Failed to pull latest changes for team %s: %v", team.Name, err)
		}
	}
}
