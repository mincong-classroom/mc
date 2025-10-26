package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/mincong-classroom/mc/common"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const year = 2025

var teamCmd = &cobra.Command{
	Use:   "team",
	Short: "List all teams",
	Run:   runTeam,
}

func runTeam(cmd *cobra.Command, args []string) {
	teams, err := ListTeams()
	if err != nil {
		log.Fatalf("Failed to list teams: %v", err)
	}

	fmt.Printf("%d teams registered:\n", len(teams))
	for _, team := range teams {
		fmt.Printf("  - %s: %s\n", team.Name, team.GetMembersAsString())
	}
}

// TODO Remove this function, use common.ListTeams instead
// ListTeams returns a list of team names by reading the classroom directory
func ListTeams() ([]common.Team, error) {
	teamFile := fmt.Sprintf("%s/.mc/teams-%d.yaml", os.Getenv("HOME"), year)
	teamData, err := os.ReadFile(teamFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	var data common.TeamRegistry
	err = yaml.Unmarshal(teamData, &data)

	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %v", err)
	}
	return data.Teams, nil
}
