package cmd

import (
	"fmt"
	"log"
	"strings"

	"github.com/mincong-classroom/mc/rules"
	"github.com/spf13/cobra"
)

var ruleLab string

var ruleCmd = &cobra.Command{
	Use:   "rule",
	Short: "List grading rules",
	Run:   runRule,
}

func init() {
	ruleCmd.Flags().StringVarP(&ruleLab, "lab", "l", "", "List the rules of one lab session only, e.g. L1 or 1")
}

func runRule(cmd *cobra.Command, args []string) {
	grader, err := rules.NewGrader()
	if err != nil {
		log.Fatalf("Failed to create grader: %v", err)
	}

	labID := strings.ToUpper(ruleLab)
	if labID != "" && !strings.HasPrefix(labID, "L") {
		labID = "L" + labID
	}
	found := false
	for _, spec := range grader.ListRuleSpecs() {
		if labID != "" && spec.LabId != labID {
			continue
		}
		found = true
		fmt.Println(spec.Representation())
		fmt.Println()
	}
	if !found {
		log.Fatalf("No rule for the lab %q: use L1 to L5", ruleLab)
	}
}
