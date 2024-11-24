// cmd/root.go
package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mc",
	Short: "Mincong Classroom - A CLI tool for grading assignments",
	Long: `Mincong Classroom (mc) is a command line interface for grading student
assignments in the Software Containerization and Orchestration course.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(gradeCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(teamCmd)
	rootCmd.AddCommand(ruleCmd)
}
