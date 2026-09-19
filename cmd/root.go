package cmd

import (
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/mincong-classroom/mc/cmd/git"
	"github.com/mincong-classroom/mc/cmd/team"
	"github.com/mincong-classroom/mc/common"
)

var RootCmd = &cobra.Command{
	Use:   "mc",
	Short: "Mincong Classroom - A CLI tool for grading assignments",
	Long: `Mincong Classroom (mc) is a command line interface for grading student
assignments in the Kubernetes course.`,
}

func Execute() error {
	// The team subcommands, such as "mc team red", are built from the registry before Cobra
	// parses the flags: read the flag --team-file ahead.
	common.TeamRegistryFile = teamFileFromArgs(os.Args[1:])
	team.AddTeamCommands()
	return RootCmd.Execute()
}

// teamFileFromArgs returns the value of the flag --team-file in the arguments, "" if absent.
func teamFileFromArgs(args []string) string {
	flags := pflag.NewFlagSet("mc", pflag.ContinueOnError)
	flags.ParseErrorsWhitelist.UnknownFlags = true
	flags.SetOutput(io.Discard)
	file := flags.String(teamFileFlag, "", "")
	_ = flags.Parse(args)
	return *file
}

const teamFileFlag = "team-file"

func init() {
	RootCmd.PersistentFlags().StringVar(&common.TeamRegistryFile, teamFileFlag, "",
		"Team registry to use instead of ~/.mc/teams-{year}.yaml, e.g. a test registry to try the commands")

	RootCmd.AddCommand(git.GitCmd)
	RootCmd.AddCommand(gradeCmd)
	RootCmd.AddCommand(infoCmd)
	RootCmd.AddCommand(k8sCmd)
	RootCmd.AddCommand(ruleCmd)
	RootCmd.AddCommand(team.TeamCmd)
}
