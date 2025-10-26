package cmd

import (
	"fmt"
	"log"
	"os/exec"

	"github.com/spf13/cobra"
)

var gitCmd = &cobra.Command{
	Use:   "git",
	Short: "Perform git operations in bulk",
	Run:   runGit,
}

func runGit(cmd *cobra.Command, args []string) {
	teams, err := listTeams()
	if err != nil {
		log.Fatalf("Failed to list teams: %v", err)
	}

	for _, team := range teams {
		fmt.Printf("Cloning repository: %s\n", team.GetRepoURL())
		targetDir := fmt.Sprintf("/Users/mincong/github/mincong-classroom/%s", team.Name)

		cloneErr := exec.Command("git", "clone", team.GetRepoURL(), targetDir).Run()
		if cloneErr != nil {
			log.Printf("Failed to clone repository for team %s: %v", team.Name, cloneErr)
		}
	}
}
