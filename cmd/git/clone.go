package git

import (
	"fmt"
	"log"
	"os/exec"

	"github.com/mincong-classroom/mc/common"
	"github.com/spf13/cobra"
)

var cloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone repositories for all teams",
	Run:   runClone,
}

func runClone(cmd *cobra.Command, args []string) {
	teams, _ := common.ListTeams()
	for _, team := range teams {
		fmt.Printf("Cloning repository: %s\n", team.GetRepoURL())
		targetDir := fmt.Sprintf("/Users/mincong/github/mincong-classroom/%s", team.GetLocalRepoDirName())

		cloneErr := exec.Command("git", "clone", team.GetRepoURL(), targetDir).Run()
		if cloneErr != nil {
			log.Printf("Failed to clone repository for team %s: %v", team.Name, cloneErr)
		}
	}
}
