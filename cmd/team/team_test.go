package team

import (
	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/github"
)

// fakeClient is an in-memory GitHub organization. Run only records the commands.
type fakeClient struct {
	users       map[string]*github.User
	repos       map[string]bool
	teams       map[string]bool
	roles       map[string]string // by "team/repo"
	memberships map[string]string // by "team/user"
	ran         []github.Command
}

func newFakeClient() *fakeClient {
	return &fakeClient{
		users:       map[string]*github.User{},
		repos:       map[string]bool{},
		teams:       map[string]bool{},
		roles:       map[string]string{},
		memberships: map[string]string{},
	}
}

func (f *fakeClient) GetUser(login string) (*github.User, error) {
	return f.users[login], nil
}

func (f *fakeClient) RepoExists(repo string) (bool, error) {
	return f.repos[repo], nil
}

func (f *fakeClient) TeamExists(team string) (bool, error) {
	return f.teams[team], nil
}

func (f *fakeClient) GetTeamRepoRole(team, repo string) (string, error) {
	return f.roles[team+"/"+repo], nil
}

func (f *fakeClient) GetTeamMembershipState(team, user string) (string, error) {
	return f.memberships[team+"/"+user], nil
}

func (f *fakeClient) Run(command github.Command) error {
	f.ran = append(f.ran, command)
	return nil
}

func (f *fakeClient) addUser(login, name string) {
	f.users[login] = &github.User{Login: login, Name: name}
}

func newTeam(name string, members ...common.TeamMember) common.Team {
	return common.Team{Name: name, Members: members}
}

func member(name, github string) common.TeamMember {
	return common.TeamMember{Name: name, Github: github}
}
