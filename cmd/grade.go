package cmd

import (
	"fmt"
	"log"

	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/rules"
	"github.com/spf13/cobra"
)

var gradeCmd = &cobra.Command{
	Use:   "grade",
	Short: "Grade assignments",
	Run:   runGrade,
}

func runGrade(cmd *cobra.Command, args []string) {
	teams, err := listTeams()
	if err != nil {
		log.Fatalf("Failed to list teams: %v", err)
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
	for team, res := range results {
		report += fmt.Sprintf("  %s:\n", team)
		for _, r := range res {
			report += fmt.Sprintf("    - %s: %.0f%% (%s)\n", r.RuleId, r.Completeness*100, r.Reason)
		}
	}
	log.Println(report)
}
