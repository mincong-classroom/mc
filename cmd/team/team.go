// Package team manages the teams of the classroom: the team registry (~/.mc/teams-{year}.yaml)
// and, on GitHub, the repository and the GitHub team of each team.
package team

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"

	"github.com/mincong-classroom/mc/common"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	actionsGroup = "actions"
	teamsGroup   = "teams"
)

// reservedNames are the actions of "mc team", which cannot be used as a team name. "provision" was
// one, before it moved to "mc team <team> provision".
var reservedNames = []string{"ls", "provision"}

var TeamCmd = newTeamRootCmd()

func newTeamRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "team [<team>] <action>",
		Short: "Manage the teams",
		Long: `Manage the teams registered in the team registry, and their repository and GitHub team
in the GitHub organization. The GitHub calls go through the gh CLI, logged in with the scopes
"repo" and "admin:org" (gh auth refresh -s admin:org).

The action "mc team ls" lists all the teams. Each team of the registry is also a subcommand,
holding the actions on that team, such as "mc team red status". A team that is not in the
registry yet can be provisioned too, which registers it: "mc team purple provision".`,
		Example: `  mc team ls
  mc team red provision
  mc team red validate
  mc team red status`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if args[0] == "provision" {
				return fmt.Errorf(`"mc team provision" is now "mc team <team> provision", e.g. "mc team red provision"`)
			}
			if _, err := common.ListTeams(); err != nil {
				return fmt.Errorf("failed to list teams: %v", err)
			}
			return fmt.Errorf("unknown team or action %q, see \"mc team ls\" for the teams of %s", args[0], common.TeamRegistryPath())
		},
		SilenceUsage: true,
	}
	cmd.AddGroup(
		&cobra.Group{ID: actionsGroup, Title: "Actions:"},
		&cobra.Group{ID: teamsGroup, Title: "Teams, see \"mc team <team> --help\" for their actions:"},
	)

	ls := newLsCmd()
	ls.GroupID = actionsGroup
	cmd.AddCommand(ls)
	return cmd
}

// AddTeamCommands adds one subcommand per team of the registry, such as "mc team red", holding the
// actions on that team. Cobra resolves the subcommands before running them, so it is called before
// running the CLI, with its arguments: when they are "mc team <team> provision" for a team that is
// not in the registry yet, that team gets a subcommand too, to be registered. When the registry
// cannot be read, no team is added and "mc team ls" reports the error.
func AddTeamCommands(args []string) {
	teams, err := common.ListTeams()
	if err != nil {
		return
	}
	addTeamCommands(TeamCmd, teams)
	if name := teamToProvision(args); name != "" {
		addTeamCommands(TeamCmd, []common.Team{{Name: name}})
	}
}

// teamToProvision returns the team of the arguments "team <team> provision", wherever the flags
// are, and "" for other arguments.
func teamToProvision(args []string) string {
	flags := pflag.NewFlagSet("mc", pflag.ContinueOnError)
	flags.ParseErrorsWhitelist.UnknownFlags = true
	flags.SetOutput(io.Discard)
	flags.String("team-file", "", "")
	flags.Bool("dry-run", false, "")
	flags.BoolP("help", "h", false, "")
	// A wrong flag is reported by Cobra, once it parses the arguments.
	_ = flags.Parse(args)
	positional := flags.Args()
	if len(positional) != 3 || positional[0] != "team" || positional[2] != "provision" {
		return ""
	}
	return positional[1]
}

func addTeamCommands(parent *cobra.Command, teams []common.Team) {
	for _, team := range teams {
		// A reserved or duplicated name is reported by "mc team ls". A name that is not a single
		// word cannot be typed as a command.
		registered := slices.ContainsFunc(parent.Commands(), func(c *cobra.Command) bool { return c.Name() == team.Name })
		if registered || slices.Contains(reservedNames, team.Name) || len(strings.Fields(team.Name)) != 1 {
			continue
		}
		parent.AddCommand(newTeamCmd(team))
	}
}

func newTeamCmd(team common.Team) *cobra.Command {
	short := team.GetMembersAsString()
	if len(team.Members) == 0 {
		short = "no members yet"
	}
	cmd := &cobra.Command{
		Use:     team.Name + " <action>",
		Short:   short,
		GroupID: teamsGroup,
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return fmt.Errorf("unknown action %q for the team %s, see \"mc team %s --help\"", args[0], team.Name, team.Name)
		},
		SilenceUsage: true,
	}
	cmd.AddCommand(newProvisionCmd(team.Name), newValidateCmd(team.Name), newStatusCmd(team.Name))
	return cmd
}

// loadRegistry reads the team registry.
func loadRegistry() (*common.TeamRegistry, error) {
	registry, err := common.LoadRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to list teams: %v", err)
	}
	return registry, nil
}

// loadTeam returns the team registry, and its team with the given name.
func loadTeam(name string) (*common.TeamRegistry, common.Team, error) {
	registry, err := loadRegistry()
	if err != nil {
		return nil, common.Team{}, err
	}
	selected, err := common.FilterTeams(registry.Teams, []string{name})
	if err != nil {
		return nil, common.Team{}, err
	}
	return registry, selected[0], nil
}

// forEach calls fn for each index in [0, n) concurrently, a few at a time, to speed up the
// GitHub calls made for each team.
func forEach(n int, fn func(i int)) {
	var wg sync.WaitGroup
	slots := make(chan struct{}, 8)
	for i := 0; i < n; i++ {
		wg.Add(1)
		slots <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-slots }()
			fn(i)
		}(i)
	}
	wg.Wait()
}
