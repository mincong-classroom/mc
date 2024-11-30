package cmd

import (
	"fmt"
	"log"

	"github.com/mincong-classroom/mc/rules"
	"github.com/spf13/cobra"
)

var ruleCmd = &cobra.Command{
	Use:   "rule",
	Short: "List grading rules",
	Run:   runRule,
}

func runRule(cmd *cobra.Command, args []string) {
	grader, err := rules.NewGrader()
	if err != nil {
		log.Fatalf("Failed to create grader: %v", err)
	}

	for _, rr := range grader.ListRuleRepresentations() {
		fmt.Println(rr)
	}
}
