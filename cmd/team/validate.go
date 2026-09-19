package team

import (
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/github"
	"github.com/spf13/cobra"
)

func newValidateCmd(name string) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate the team in the registry",
		Long: `Validate the team in the registry: the name format and its uniqueness, at most 2 members,
each member in one team only, and each GitHub username existing. The display name of each GitHub
account is printed, for the students to confirm it.`,
		Example: "  mc team " + name + " validate\n  mc team " + name + " validate --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidate(cmd.OutOrStdout(), name, asJSON)
		},
		SilenceUsage: true,
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print the result as JSON")
	return cmd
}

// maxMembers is the maximum number of members of a team. A team without members is valid: it is
// created before the course, and its members are added on the first day.
const maxMembers = 2

var teamNamePattern = regexp.MustCompile(`^[a-z]+$`)

// Validation is the result of the validation of one team.
type Validation struct {
	Team     common.Team
	Users    map[string]*github.User // GitHub account of each member found, by GitHub username
	Problems []string
}

func (v *Validation) addf(format string, args ...any) {
	v.Problems = append(v.Problems, fmt.Sprintf(format, args...))
}

func runValidate(out io.Writer, name string, asJSON bool) error {
	teams, team, err := loadTeam(name)
	if err != nil {
		return err
	}

	v := validateTeam(team, teams, github.CLI{})
	if asJSON {
		if err := writeJSON(out, newTeamJSON(team, &v, nil)); err != nil {
			return err
		}
	} else {
		printValidation(out, v)
	}
	if len(v.Problems) > 0 {
		return fmt.Errorf("the team %s is not valid", name)
	}
	return nil
}

// validateTeam validates the team among all the teams of the registry. It only relies on the
// registry and on GitHub: a valid team can be provisioned.
func validateTeam(team common.Team, teams []common.Team, client github.Client) Validation {
	v := Validation{Team: team, Users: map[string]*github.User{}}

	if !teamNamePattern.MatchString(team.Name) {
		v.addf("invalid name %q: use lowercase letters only, such as a color", team.Name)
	}
	if slices.Contains(reservedNames, team.Name) {
		v.addf("invalid name %q: reserved by the command \"mc team %s\"", team.Name, team.Name)
	}
	if n := countTeams(teams, team.Name); n > 1 {
		v.addf("the name %q is used by %d teams", team.Name, n)
	}
	if len(team.Members) > maxMembers {
		v.addf("%d members, at most %d are allowed", len(team.Members), maxMembers)
	}

	for _, member := range team.Members {
		if member.Name == "" {
			v.addf("@%s has no name", member.Github)
		}
		if teamNames := findTeamsOf(teams, member); len(teamNames) > 1 {
			v.addf("%s is in several teams: %s", describeMember(member), strings.Join(teamNames, ", "))
		}

		if member.Github == "" {
			v.addf("%s has no GitHub username", member.Name)
			continue
		}
		user, err := client.GetUser(member.Github)
		switch {
		case err != nil:
			v.addf("cannot check the GitHub user @%s: %v", member.Github, err)
		case user == nil:
			v.addf("the GitHub user @%s does not exist", member.Github)
		default:
			v.Users[member.Github] = user
		}
	}
	return v
}

func countTeams(teams []common.Team, name string) int {
	count := 0
	for _, team := range teams {
		if team.Name == name {
			count++
		}
	}
	return count
}

// findTeamsOf returns the names of the teams having this member, identified by their name or
// their GitHub username.
func findTeamsOf(teams []common.Team, member common.TeamMember) []string {
	var names []string
	for _, team := range teams {
		for _, m := range team.Members {
			sameName := member.Name != "" && common.SameName(m.Name, member.Name)
			sameGithub := member.Github != "" && strings.EqualFold(m.Github, member.Github)
			if sameName || sameGithub {
				names = append(names, team.Name)
			}
		}
	}
	return names
}

func describeMember(member common.TeamMember) string {
	if member.Github == "" {
		return member.Name
	}
	return fmt.Sprintf("%s (@%s)", member.Name, member.Github)
}

// describeGithubUser describes the GitHub account of a member, for the students to confirm it.
func describeGithubUser(user *github.User) string {
	switch {
	case user == nil:
		return "not found on GitHub"
	case user.Name == "":
		return "no display name on GitHub"
	default:
		return fmt.Sprintf("%q on GitHub", user.Name)
	}
}

func printValidation(out io.Writer, v Validation) {
	fmt.Fprintf(out, "%s: %s\n", v.Team.Name, describeMemberCount(v.Team))
	for _, member := range v.Team.Members {
		fmt.Fprintf(out, "  - %s: %s\n", describeMember(member), describeGithubUser(v.Users[member.Github]))
	}
	for _, problem := range v.Problems {
		fmt.Fprintf(out, "  ✗ %s\n", problem)
	}
	if len(v.Problems) == 0 {
		fmt.Fprintln(out, "  ✓ valid")
	}
}

func describeMemberCount(team common.Team) string {
	switch len(team.Members) {
	case 0:
		return "no members yet"
	case 1:
		return "1 member"
	default:
		return fmt.Sprintf("%d members", len(team.Members))
	}
}
