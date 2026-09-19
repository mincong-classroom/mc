package common

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultYear = 2026

// Year returns the year of the cohort, which selects the files read in ~/.mc. It can be
// overridden with the environment variable MC_YEAR, e.g. MC_YEAR=2025.
func Year() int {
	if year, err := strconv.Atoi(os.Getenv("MC_YEAR")); err == nil {
		return year
	}
	return defaultYear
}

// ConfigDir returns the directory holding the private data of the classroom, i.e. ~/.mc.
func ConfigDir() string {
	return filepath.Join(os.Getenv("HOME"), ".mc")
}

// TeamRegistryFile replaces the team registry of the current year when it is not empty, e.g. to
// try the commands on a test registry. It is set by the global flag --team-file.
var TeamRegistryFile string

// TeamRegistryPath returns the path of the team registry: TeamRegistryFile if set, otherwise the
// registry of the current year.
func TeamRegistryPath() string {
	if TeamRegistryFile == "" {
		return filepath.Join(ConfigDir(), fmt.Sprintf("teams-%d.yaml", Year()))
	}
	// The shell does not expand the "~" of --team-file=~/...
	if rest, ok := strings.CutPrefix(TeamRegistryFile, "~/"); ok {
		return filepath.Join(os.Getenv("HOME"), rest)
	}
	return TeamRegistryFile
}

// LoadRegistry reads the team registry.
func LoadRegistry() (*TeamRegistry, error) {
	teamData, err := os.ReadFile(TeamRegistryPath())
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	var data TeamRegistry
	err = yaml.Unmarshal(teamData, &data)

	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %v", err)
	}
	return &data, nil
}

// ListTeams returns the teams of the team registry.
func ListTeams() ([]Team, error) {
	registry, err := LoadRegistry()
	if err != nil {
		return nil, err
	}
	return registry.Teams, nil
}

// FilterTeams returns the teams with the given names, or an error if a name is not registered.
func FilterTeams(teams []Team, names []string) ([]Team, error) {
	var selected []Team
	for _, name := range names {
		i := slices.IndexFunc(teams, func(team Team) bool { return team.Name == name })
		if i < 0 {
			return nil, fmt.Errorf("team %q not found in %s", name, TeamRegistryPath())
		}
		selected = append(selected, teams[i])
	}
	return selected, nil
}
