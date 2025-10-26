package cmd

import (
	"fmt"
	"log"

	"github.com/mincong-classroom/mc/common"
	"github.com/spf13/cobra"
)

var teamCmd = &cobra.Command{
	Use:   "team",
	Short: "List all teams",
	Run:   runTeam,
}

func runTeam(cmd *cobra.Command, args []string) {
	teams, err := common.ListTeams()
	if err != nil {
		log.Fatalf("Failed to list teams: %v", err)
	}

	fmt.Printf("%d teams registered:\n", len(teams))
	for _, team := range teams {
		fmt.Printf("  - %s: %s\n", team.Name, team.GetMembersAsString())
	}
}
