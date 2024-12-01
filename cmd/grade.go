package cmd

import (
	"fmt"
	"log"

	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/rules"
	"github.com/spf13/cobra"
)

var selectedTeamNames []string

var gradeCmd = &cobra.Command{
	Use:   "grade",
	Short: "Grade assignments",
	Run:   runGrade,
}

func init() {
	gradeCmd.Flags().StringArrayVarP(&selectedTeamNames, "team", "t", []string{}, "Specify team(s) to grade")
}

func runGrade(cmd *cobra.Command, args []string) {
	teams, err := listTeams()
	if err != nil {
		log.Fatalf("Failed to list teams: %v", err)
	}
	if len(selectedTeamNames) > 0 {
		log.Printf("Grading %d team(s): %s\n", len(selectedTeamNames), selectedTeamNames)
		teams = filterTeams(teams, selectedTeamNames)
	} else {
		log.Println("Grading all teams")
	}

	grader, err := rules.NewGrader()
	if err != nil {
		log.Fatalf("Failed to create grader: %v", err)
	}

	results := make(map[string][]common.RuleEvaluationResult)

	for _, team := range teams {
		results[team.Name] = grader.GradeL1(team)
	}

	report := "Report:\n"
	for _, team := range teams {
		report += fmt.Sprintf("  %s:\n", team.Name)
		for _, r := range results[team.Name] {
			report += fmt.Sprintf("    - %s: %3.0f%% (%s)\n", r.RuleId, r.Completeness*100, r.Reason)
		}
	}
	log.Println(report)
}

func filterTeams(teams []common.Team, selectedTeamNames []string) []common.Team {
	var selectedTeams []common.Team
	for _, team := range teams {
		for _, name := range selectedTeamNames {
			if team.Name == name {
				selectedTeams = append(selectedTeams, team)
			}
		}
	}
	return selectedTeams
}
