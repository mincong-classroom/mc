package team

import (
	"fmt"
	"io"

	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/github"
	"github.com/spf13/cobra"
)

func newLsCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List the teams, their status and the students not in a team",
		Long: `List the teams of the registry with their members, their status on GitHub (see
"mc team <team> status") and a warning for each validation problem (see
"mc team <team> validate"). When the registry lists the students of the year, the ones who are
not in a team yet are listed at the end.`,
		Example:      "  mc team ls\n  mc team ls --json",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLs(cmd.OutOrStdout(), asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print the teams as JSON")
	return cmd
}

func runLs(out io.Writer, asJSON bool) error {
	registry, err := loadRegistry()
	if err != nil {
		return err
	}
	teams := registry.Teams

	validations := make([]Validation, len(teams))
	statuses := make([]Status, len(teams))
	forEach(len(teams), func(i int) {
		validations[i] = validateTeam(teams[i], registry, github.CLI{})
		statuses[i] = teamStatus(teams[i], github.CLI{})
	})

	if asJSON {
		result := lsJSON{Year: common.Year(), Teams: []teamJSON{}, StudentsNotInTeam: studentsNotInTeam(registry.Students, teams)}
		for i, team := range teams {
			result.Teams = append(result.Teams, newTeamJSON(team, &validations[i], &statuses[i]))
		}
		return writeJSON(out, result)
	}

	fmt.Fprintf(out, "%d teams registered in %s:\n", len(teams), common.TeamRegistryPath())
	for i, team := range teams {
		if len(team.Members) == 0 {
			fmt.Fprintf(out, "  - %s: no members yet\n", team.Name)
		} else {
			fmt.Fprintf(out, "  - %s: %s\n", team.Name, team.GetMembersAsString())
		}
		fmt.Fprintf(out, "    %s\n", statuses[i].Summary())
		printProblems(out, "    ", validations[i])
	}
	if registry.Students != nil {
		fmt.Fprintln(out)
		printUnassignedStudents(out, registry.Students, teams)
	}
	return nil
}

// unassignedStudents returns the students of the registry who are not in any team.
func unassignedStudents(students []common.Student, teams []common.Team) []common.Student {
	var result []common.Student
	for _, student := range students {
		assigned := false
		for _, team := range teams {
			for _, member := range team.Members {
				if common.SameName(member.Name, student.Name) {
					assigned = true
				}
			}
		}
		if !assigned {
			result = append(result, student)
		}
	}
	return result
}

func printUnassignedStudents(out io.Writer, students []common.Student, teams []common.Team) {
	unassigned := unassignedStudents(students, teams)
	if len(unassigned) == 0 {
		fmt.Fprintf(out, "All the %d students are in a team.\n", len(students))
		return
	}
	fmt.Fprintf(out, "%d of %d students not in a team yet:\n", len(unassigned), len(students))
	for _, student := range unassigned {
		fmt.Fprintf(out, "  - %s\n", student.Name)
	}
}
