package git

import (
	"fmt"
	"log"
	"os/exec"

	"github.com/mincong-classroom/mc/common"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:     "show",
	Short:   "Show content of the specific file of all teams",
	Run:     runShow,
	Example: "show main:k8s/pod-nginx.yaml",
}

func runShow(cmd *cobra.Command, args []string) {
	teams, _ := common.ListTeams()
	if len(args) > 1 {
		fmt.Println("Only one argument is allowed")
		return
	}

	if len(args) == 0 {
		fmt.Println("Please specify the file to show, e.g., main:k8s/pod-nginx.yaml")
		return
	}

	fileExpr := args[0]

	for _, team := range teams {
		targetDir := fmt.Sprintf("/Users/mincong/github/mincong-classroom/%s", team.GetLocalRepoDirName())
		cmd := exec.Command("git", "-C", targetDir, "show", fileExpr)

		fmt.Printf("[team: %s] %s\n", team.Name, cmd.String())
		err := cmd.Run()

		if err != nil {
			log.Printf("Failed to show content for team %q: %v", team.Name, err)
		}
	}
}
