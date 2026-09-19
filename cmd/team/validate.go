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
		Long: `Validate the team in the registry. The errors prevent its provisioning: an invalid or reserved
name, a name used by another team, a member without a GitHub username or whose GitHub user does not
exist. The warnings do not, once the teacher confirms: more than 2 members, a member in several
teams, a member without a name, or not among the students of the registry. The display name of
each GitHub account is printed, for the students to confirm it.`,
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
	Errors   []string                // Prevent the provisioning
	Warnings []string                // The teacher confirms before the provisioning
}

func (v *Validation) errorf(format string, args ...any) {
	v.Errors = append(v.Errors, fmt.Sprintf(format, args...))
}

func (v *Validation) warnf(format string, args ...any) {
	v.Warnings = append(v.Warnings, fmt.Sprintf(format, args...))
}

func runValidate(out io.Writer, name string, asJSON bool) error {
	registry, team, err := loadTeam(name)
	if err != nil {
		return err
	}

	v := validateTeam(team, registry, github.CLI{})
	if asJSON {
		if err := writeJSON(out, newTeamJSON(team, &v, nil)); err != nil {
			return err
		}
	} else {
		printValidation(out, v)
	}
	if len(v.Errors) > 0 {
		return fmt.Errorf("the team %s is not valid", name)
	}
	return nil
}

// validateTeam validates the team among all the teams of the registry. For a member, only the
// GitHub user must be valid: being among the students of the registry is a warning, and nothing
// when the registry has no students.
func validateTeam(team common.Team, registry *common.TeamRegistry, client github.Client) Validation {
	v := Validation{Team: team, Users: map[string]*github.User{}}
	v.Errors = append(v.Errors, teamNameErrors(team.Name, registry.Teams)...)
	if len(team.Members) > maxMembers {
		v.warnf("%d members, at most %d are expected", len(team.Members), maxMembers)
	}

	for _, member := range team.Members {
		if member.Name == "" {
			v.warnf("@%s has no name", member.Github)
		} else if registry.Students != nil && !isStudent(registry.Students, member.Name) {
			v.warnf("%s is not among the students of the registry", member.Name)
		}
		if teamNames := findTeamsOf(registry.Teams, member); len(teamNames) > 1 {
			v.warnf("%s is in several teams: %s", describeMember(member), strings.Join(teamNames, ", "))
		}

		if member.Github == "" {
			v.errorf("%s has no GitHub username", member.Name)
			continue
		}
		user, err := client.GetUser(member.Github)
		switch {
		case err != nil:
			v.errorf("cannot check the GitHub user @%s: %v", member.Github, err)
		case user == nil:
			v.errorf("the GitHub user @%s does not exist", member.Github)
		default:
			v.Users[member.Github] = user
		}
	}
	return v
}

// teamNameErrors returns the errors of a team name among the teams of the registry.
func teamNameErrors(name string, teams []common.Team) []string {
	var problems []string
	if !teamNamePattern.MatchString(name) {
		problems = append(problems, fmt.Sprintf("invalid name %q: use lowercase letters only, such as a color", name))
	}
	if slices.Contains(reservedNames, name) {
		problems = append(problems, fmt.Sprintf("invalid name %q: reserved by the command \"mc team %s\"", name, name))
	}
	if n := countTeams(teams, name); n > 1 {
		problems = append(problems, fmt.Sprintf("the name %q is used by %d teams", name, n))
	}
	return problems
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

func isStudent(students []common.Student, name string) bool {
	return slices.ContainsFunc(students, func(s common.Student) bool { return common.SameName(s.Name, name) })
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

// printProblems prints the errors, then the warnings of a validation, after the given indentation.
func printProblems(out io.Writer, indent string, v Validation) {
	for _, message := range v.Errors {
		fmt.Fprintf(out, "%s✗ %s\n", indent, message)
	}
	for _, message := range v.Warnings {
		fmt.Fprintf(out, "%s⚠ %s\n", indent, message)
	}
}

func printValidation(out io.Writer, v Validation) {
	fmt.Fprintf(out, "%s: %s\n", v.Team.Name, describeMemberCount(v.Team))
	for _, member := range v.Team.Members {
		fmt.Fprintf(out, "  - %s: %s\n", describeMember(member), describeGithubUser(v.Users[member.Github]))
	}
	printProblems(out, "  ", v)
	switch {
	case len(v.Errors) > 0:
	case len(v.Warnings) > 0:
		fmt.Fprintln(out, "  ✓ valid, with warnings to confirm when provisioning")
	default:
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
