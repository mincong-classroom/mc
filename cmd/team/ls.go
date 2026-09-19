package team

import (
	"fmt"

	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/github"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List the teams, their status and the students not in a team",
	Long: `List the teams of the registry with their members, their status on GitHub (see
"mc team status") and a warning for each validation problem (see "mc team validate"). The
students of the school list who are not in a team yet are listed at the end.`,
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE:         runLs,
}

func runLs(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()
	teams, err := common.ListTeams()
	if err != nil {
		return fmt.Errorf("failed to list teams: %v", err)
	}
	students, err := loadStudents(out)
	if err != nil {
		return err
	}

	validations := make([]Validation, len(teams))
	statuses := make([]Status, len(teams))
	forEach(len(teams), func(i int) {
		validations[i] = validateTeam(teams[i], teams, students, github.CLI{})
		statuses[i] = teamStatus(teams[i], github.CLI{})
	})

	fmt.Fprintf(out, "%d teams registered in %s:\n", len(teams), common.TeamRegistryPath())
	for i, team := range teams {
		if len(team.Members) == 0 {
			fmt.Fprintf(out, "  - %s: no members yet\n", team.Name)
		} else {
			fmt.Fprintf(out, "  - %s: %s\n", team.Name, team.GetMembersAsString())
		}
		fmt.Fprintf(out, "    %s\n", statuses[i].Summary())
		for _, problem := range validations[i].Problems {
			fmt.Fprintf(out, "    ⚠ %s\n", problem)
		}
	}
	fmt.Fprintln(out)
	printUnassignedStudents(out, students, teams)
	return nil
}
