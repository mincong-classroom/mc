// cmd/team.go
package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
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
	
	fmt.Println("Registered teams:")
	for _, team := range teams {
		fmt.Printf("- %s\n", team)
	}
}

// listTeams returns a list of team names by reading the classroom directory
func listTeams() ([]string, error) {
	baseDir := "/Users/mincong/github/classroom"
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %v", err)
	}

	var teams []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if len(name) > 11 && name[:11] == "containers-" {
			teams = append(teams, name[11:])
		}
	}
	return teams, nil
}
