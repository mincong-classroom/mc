package cmd

import (
	"fmt"
	"log"

	"github.com/mincong-classroom/grading/rules"
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

	for _, team := range teams {
		fmt.Printf("\n=== Grading Team %s ===\n", team.Name)

		err := grader.GradeL1(team.Name)

		if err != nil {
			log.Printf("Warning: %v", err)
			continue
		}

		fmt.Print("Grading done")
	}
}
