package common

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const year = 2025

// ListTeams returns a list of team names by reading the classroom directory
func ListTeams() ([]Team, error) {
	teamFile := fmt.Sprintf("%s/.mc/teams-%d.yaml", os.Getenv("HOME"), year)
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
