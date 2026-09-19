// Package github manages the GitHub organization of the classroom: the team repositories, the
// GitHub teams and their members. It calls GitHub through the gh CLI, which reuses the teacher's
// login (scopes "repo" and "admin:org").
package github

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

const (
	// Org is the GitHub organization of the classroom.
	Org = "mincong-classroom"
	// TemplateRepo is the repository from which the team repositories are generated.
	TemplateRepo = Org + "/containers"
)

// User is a GitHub account.
type User struct {
	Login string `json:"login"`
	Name  string `json:"name"` // Display name, may be empty
}

// Client reads and changes the GitHub organization.
type Client interface {
	// GetUser returns the GitHub user with the given login, or nil if it does not exist.
	GetUser(login string) (*User, error)
	// RepoExists reports whether the repository exists in the organization.
	RepoExists(repo string) (bool, error)
	// TeamExists reports whether the GitHub team exists in the organization.
	TeamExists(team string) (bool, error)
	// GetTeamRepoRole returns the role of the team on the repository, such as "write" or
	// "admin", or "" if the team has no access to it.
	GetTeamRepoRole(team, repo string) (string, error)
	// GetTeamMembershipState returns "active" or "pending" (invitation not accepted yet), or ""
	// if the user is not a member of the team.
	GetTeamMembershipState(team, user string) (string, error)
	// Run runs a command changing the organization.
	Run(command Command) error
}

// Command is a gh command changing the organization, shown to the teacher before it runs.
type Command []string

func (c Command) String() string {
	return "gh " + strings.Join(c, " ")
}

// CreateRepoCommand creates a private repository from the template.
func CreateRepoCommand(repo string) Command {
	return Command{"repo", "create", Org + "/" + repo, "--private", "--template", TemplateRepo}
}

// CreateTeamCommand creates a secret GitHub team.
func CreateTeamCommand(team string) Command {
	return Command{"api", "-X", "POST", "orgs/" + Org + "/teams", "-f", "name=" + team, "-f", "privacy=secret"}
}

// GrantPushCommand grants the team push access to the repository.
func GrantPushCommand(team, repo string) Command {
	return Command{"api", "-X", "PUT", fmt.Sprintf("orgs/%s/teams/%s/repos/%s/%s", Org, team, Org, repo), "-f", "permission=push"}
}

// AddMemberCommand adds the user to the team. GitHub invites the user to the organization if
// they are not a member yet.
func AddMemberCommand(team, user string) Command {
	return Command{"api", "-X", "PUT", fmt.Sprintf("orgs/%s/teams/%s/memberships/%s", Org, team, user), "-f", "role=member"}
}

// CLI is the Client calling GitHub through the gh CLI.
type CLI struct{}

var errNotFound = errors.New("not found")

func (CLI) GetUser(login string) (*User, error) {
	var user User
	err := api(&user, "users/"+login)
	if errors.Is(err, errNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (CLI) RepoExists(repo string) (bool, error) {
	return exists(fmt.Sprintf("repos/%s/%s", Org, repo))
}

func (CLI) TeamExists(team string) (bool, error) {
	return exists(fmt.Sprintf("orgs/%s/teams/%s", Org, team))
}

func (CLI) GetTeamRepoRole(team, repo string) (string, error) {
	var result struct {
		RoleName string `json:"role_name"`
	}
	// Without this media type, GitHub answers 204 with no content instead of the permissions.
	err := api(&result, "-H", "Accept: application/vnd.github.v3.repository+json",
		fmt.Sprintf("orgs/%s/teams/%s/repos/%s/%s", Org, team, Org, repo))
	if errors.Is(err, errNotFound) {
		return "", nil
	}
	return result.RoleName, err
}

func (CLI) GetTeamMembershipState(team, user string) (string, error) {
	var result struct {
		State string `json:"state"`
	}
	err := api(&result, fmt.Sprintf("orgs/%s/teams/%s/memberships/%s", Org, team, user))
	if errors.Is(err, errNotFound) {
		return "", nil
	}
	return result.State, err
}

func (CLI) Run(command Command) error {
	_, err := gh(command...)
	return err
}

func exists(path string) (bool, error) {
	err := api(nil, path)
	if errors.Is(err, errNotFound) {
		return false, nil
	}
	return err == nil, err
}

// api calls "gh api" with the given arguments and decodes the JSON response into result, unless
// result is nil. It returns errNotFound for an HTTP 404.
func api(result any, args ...string) error {
	out, err := gh(append([]string{"api"}, args...)...)
	if err != nil {
		return err
	}
	if result == nil {
		return nil
	}
	return json.Unmarshal(out, result)
}

func gh(args ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("gh", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if strings.Contains(message, "(HTTP 404)") {
			return nil, errNotFound
		}
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("%s: %s", Command(args), message)
	}
	return stdout.Bytes(), nil
}
