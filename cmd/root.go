package cmd

import (
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "mc",
	Short: "Mincong Classroom - A CLI tool for grading assignments",
	Long: `Mincong Classroom (mc) is a command line interface for grading student
assignments in the Kubernetes course.`,
}

func Execute() error {
	return RootCmd.Execute()
}

func init() {
	RootCmd.AddCommand(gitCmd)
	RootCmd.AddCommand(gradeCmd)
	RootCmd.AddCommand(infoCmd)
	RootCmd.AddCommand(k8sCmd)
	RootCmd.AddCommand(ruleCmd)
	RootCmd.AddCommand(teamCmd)
}
