package team

import (
	"fmt"
	"strings"

	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/github"
	"github.com/spf13/cobra"
)

func newStatusCmd(name string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the status of the team on GitHub",
		Long: `Show the status of the team on GitHub: whether the repository and the GitHub team exist,
the access of the GitHub team to the repository, and whether each member is "active" or
"pending" (invitation not accepted yet).`,
		Example: "  mc team " + name + " status",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, team, err := loadTeam(name)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), teamStatus(team, github.CLI{}).Summary())
			return nil
		},
		SilenceUsage: true,
	}
}

// Status is the state of a team on GitHub.
type Status struct {
	Team       common.Team
	RepoExists bool
	TeamExists bool
	RepoRole   string            // Role of the GitHub team on the repository, "" if none
	States     map[string]string // Membership state by GitHub username: "active", "pending" or ""
	Err        error             // Set when the status could not be fetched
}

func teamStatus(team common.Team, client github.Client) Status {
	s := Status{Team: team, States: map[string]string{}}
	repo := team.GetRepoName()
	var err error

	if s.RepoExists, err = client.RepoExists(repo); err != nil {
		s.Err = err
		return s
	}
	if s.TeamExists, err = client.TeamExists(team.Name); err != nil {
		s.Err = err
		return s
	}
	if s.RepoExists && s.TeamExists {
		if s.RepoRole, err = client.GetTeamRepoRole(team.Name, repo); err != nil {
			s.Err = err
			return s
		}
	}
	if s.TeamExists {
		for _, member := range team.Members {
			if member.Github == "" {
				continue
			}
			if s.States[member.Github], err = client.GetTeamMembershipState(team.Name, member.Github); err != nil {
				s.Err = err
				return s
			}
		}
	}
	return s
}

// Summary returns the status in one line, such as:
//
//	✓ repo k8s-red · ✓ team red · ✓ access write · @jsmith active · @jdoe pending
func (s Status) Summary() string {
	if s.Err != nil {
		return "✗ cannot get the status: " + s.Err.Error()
	}
	parts := []string{
		mark(s.RepoExists) + " repo " + s.Team.GetRepoName(),
		mark(s.TeamExists) + " team " + s.Team.Name,
	}
	switch s.RepoRole {
	case "write":
		parts = append(parts, "✓ access write")
	case "":
		parts = append(parts, "✗ no access")
	default:
		parts = append(parts, fmt.Sprintf("⚠ access %s instead of write", s.RepoRole))
	}
	for _, member := range s.Team.Members {
		if member.Github == "" {
			continue
		}
		state := s.States[member.Github]
		if state == "" {
			state = "not invited"
		}
		parts = append(parts, fmt.Sprintf("@%s %s", member.Github, state))
	}
	return strings.Join(parts, " · ")
}

func mark(ok bool) string {
	if ok {
		return "✓"
	}
	return "✗"
}
