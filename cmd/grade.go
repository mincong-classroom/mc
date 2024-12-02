package cmd

import (
	"fmt"
	"log"

	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/rules"
	"github.com/spf13/cobra"
)

var selectedTeamNames []string
var selectedLab string

var gradeCmd = &cobra.Command{
	Use:   "grade",
	Short: "Grade assignments",
	Run:   runGrade,
}

func init() {
	gradeCmd.Flags().StringArrayVarP(&selectedTeamNames, "team", "t", []string{}, "Specify team(s) to grade")
	gradeCmd.Flags().StringVarP(&selectedLab, "lab", "l", "", "Specify lab session to grade")
}

func runGrade(cmd *cobra.Command, args []string) {
	teams, err := listTeams()
	if err != nil {
		fmt.Printf("Failed to list teams: %v", err)
		return
	}
	if len(selectedTeamNames) > 0 {
		fmt.Printf("Grading %d team(s): %s\n", len(selectedTeamNames), selectedTeamNames)
		teams = filterTeams(teams, selectedTeamNames)
	} else {
		fmt.Println("Grading all teams")
	}

	grader, err := rules.NewGrader()
	if err != nil {
		log.Fatalf("Failed to create grader: %v", err)
		return
	}

	allResults := make(map[string][]common.RuleEvaluationResult)

	for _, team := range teams {
		var (
			shouldGradeL1 = selectedLab == "" || selectedLab == "L1" || selectedLab == "1"
			shouldGradeL2 = selectedLab == "" || selectedLab == "L2" || selectedLab == "2"
			shouldGradeL3 = selectedLab == "" || selectedLab == "L3" || selectedLab == "3"
			shouldGradeL4 = selectedLab == "" || selectedLab == "L4" || selectedLab == "4"
			shouldGradeL5 = selectedLab == "" || selectedLab == "L5" || selectedLab == "5"
			results       []common.RuleEvaluationResult
		)
		if shouldGradeL1 {
			r := grader.GradeL1(team)
			results = append(results, r...)
		}
		if shouldGradeL2 {
			r := grader.GradeL2(team)
			results = append(results, r...)
		}
		if shouldGradeL3 {
			r := grader.GradeL3(team)
			results = append(results, r...)
		}
		if shouldGradeL4 {
			r := grader.GradeL4(team)
			results = append(results, r...)
		}
		if shouldGradeL5 {
			r := grader.GradeL5(team)
			results = append(results, r...)
		}
		allResults[team.Name] = results
	}

	report := "Report:\n"
	for _, team := range teams {
		report += fmt.Sprintf("  %s:\n", team.Name)
		for _, r := range allResults[team.Name] {
			report += fmt.Sprintf("    - %s: %3.0f%% (%s)\n", r.RuleId, r.Completeness*100, r.Reason)
			if r.ExecError != nil {
				report += fmt.Sprintf("      Error: %v\n", r.ExecError)
			}
		}
	}
	fmt.Println(report)
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
