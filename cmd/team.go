package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var teamCmd = &cobra.Command{
	Use:   "team",
	Short: "List all teams",
	Run:   runTeam,
}

func runTeam(cmd *cobra.Command, args []string) {
	teams, err := listTeams()
	if err != nil {
		log.Fatalf("Failed to list teams: %v", err)
	}

	fmt.Printf("%d teams registered:\n", len(teams))
	for _, team := range teams {
		fmt.Printf("  - %s: %s\n", team.Name, team.GetMembersAsString())
	}
}

// listTeams returns a list of team names by reading the classroom directory
func listTeams() ([]Team, error) {
	teamFile := fmt.Sprintf("%s/.mc/teams.yaml", os.Getenv("HOME"))
	teamData, err := os.ReadFile(teamFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	var data TeamRegistry
	err = yaml.Unmarshal(teamData, &data)

	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %v", err)
	}
	return data.Teams, nil
}
