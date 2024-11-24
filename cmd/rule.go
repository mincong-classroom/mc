// cmd/rule.go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var ruleCmd = &cobra.Command{
	Use:   "rule",
	Short: "List grading rules",
	Run:   runRule,
}

func runRule(cmd *cobra.Command, args []string) {
	fmt.Println("Grading Rules:")
	fmt.Println("1. Documentation (30%)")
	fmt.Println("   - README.md completeness")
	fmt.Println("   - Architecture explanation")
	fmt.Println("   - Setup instructions")
	fmt.Println("2. Implementation (40%)")
	fmt.Println("   - Container implementation")
	fmt.Println("   - Orchestration setup")
	fmt.Println("   - Code quality")
	fmt.Println("3. Innovation (30%)")
	fmt.Println("   - Creative solutions")
	fmt.Println("   - Extra features")
}
