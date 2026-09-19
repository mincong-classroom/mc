// Package team manages the teams of the classroom: the team registry (~/.mc/teams-{year}.yaml),
// the school list (~/.mc/students-{year}.tsv), and, on GitHub, the repository and the GitHub
// team of each team.
package team

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sync"

	"github.com/mincong-classroom/mc/common"
	"github.com/spf13/cobra"
)

var TeamCmd = &cobra.Command{
	Use:   "team",
	Short: "Manage the teams",
	Long: `Manage the teams registered in the team registry, and their repository and GitHub team
in the GitHub organization. The GitHub calls go through the gh CLI, logged in with the scopes
"repo" and "admin:org" (gh auth refresh -s admin:org).`,
}

// selectedTeamNames is the value of the flag --team, shared by the subcommands.
var selectedTeamNames []string

func init() {
	TeamCmd.AddCommand(lsCmd)
	TeamCmd.AddCommand(validateCmd)
	TeamCmd.AddCommand(statusCmd)
	TeamCmd.AddCommand(provisionCmd)

	for _, cmd := range []*cobra.Command{validateCmd, statusCmd, provisionCmd} {
		cmd.Flags().StringArrayVarP(&selectedTeamNames, "team", "t", []string{}, "Team(s) to select, all teams if omitted")
		cmd.SilenceUsage = true
	}
}

// loadTeams returns all the registered teams, and the ones selected with the flag --team.
func loadTeams() (all []common.Team, selected []common.Team, err error) {
	all, err = common.ListTeams()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list teams: %v", err)
	}
	if len(selectedTeamNames) == 0 {
		return all, all, nil
	}
	selected, err = common.FilterTeams(all, selectedTeamNames)
	return all, selected, err
}

// loadStudents reads the school list. When it does not exist, it prints a note and returns nil:
// the checks against the school list are skipped.
func loadStudents(out io.Writer) ([]common.Student, error) {
	students, err := common.ListStudents()
	if errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(out, "No school list at %s: the members are not checked against it.\n\n", common.StudentListPath())
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read the school list: %v", err)
	}
	return students, nil
}

// forEach calls fn for each index in [0, n) concurrently, a few at a time, to speed up the
// GitHub calls made for each team.
func forEach(n int, fn func(i int)) {
	var wg sync.WaitGroup
	slots := make(chan struct{}, 8)
	for i := 0; i < n; i++ {
		wg.Add(1)
		slots <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-slots }()
			fn(i)
		}(i)
	}
	wg.Wait()
}
