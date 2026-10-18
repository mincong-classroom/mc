package team

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mincong-classroom/mc/common"
	"github.com/mincong-classroom/mc/github"
)

var redTeam = newTeam("red", member("SMITH John", "jsmith"), member("DOE Jane", "jdoe"))

var allRedCommands = []github.Command{
	github.CreateRepoCommand("k8s-red"),
	github.CreateTeamCommand("red"),
	github.GrantPushCommand("red", "k8s-red"),
	github.AddMemberCommand("red", "jsmith"),
	github.AddMemberCommand("red", "jdoe"),
}

func newTestProvisioner(client github.Client, input string, dryRun bool) (*provisioner, *bytes.Buffer) {
	out := &bytes.Buffer{}
	return &provisioner{
		client: client,
		in:     bufio.NewReader(strings.NewReader(input)),
		out:    out,
		dryRun: dryRun,
	}, out
}

func TestRun(t *testing.T) {
	newRegistry := func() *common.TeamRegistry {
		return &common.TeamRegistry{
			Students: []common.Student{{Name: "SMITH John"}, {Name: "MARTIN Alex"}}, // DOE Jane is not among them
			Teams: []common.Team{
				redTeam,
				newTeam("orange"),
				newTeam("gray", member("NOBODY Someone", "ghost")), // Not on GitHub
			},
		}
	}
	provisioned := func(client *fakeClient) {
		client.repos["k8s-red"] = true
		client.teams["red"] = true
		client.roles["red/k8s-red"] = "write"
		client.memberships["red/jsmith"] = "active"
		client.memberships["red/jdoe"] = "pending"
	}
	tests := []struct {
		name       string
		team       string
		input      string
		dryRun     bool
		setup      func(client *fakeClient)
		wantErr    string
		wantRan    []github.Command
		wantOutput []string
		notOutput  string
	}{
		{
			name:    "one confirmation, then every step",
			team:    "red",
			input:   "y\n",
			wantRan: allRedCommands,
			wantOutput: []string{
				`
== Team red: 2 members
  - SMITH John (@jsmith): "John Smith" on GitHub
  - DOE Jane (@jdoe): "Jane Doe" on GitHub
⚠ DOE Jane is not among the students of the registry
Provision the team red? (y/N): ✓ Create the private repository mincong-classroom/k8s-red from the template mincong-classroom/containers
`,
				`✓ Add @jdoe to the GitHub team red, which invites them to the organization

Team "red" provisioned.
- repo: https://github.com/mincong-classroom/k8s-red
- team: https://github.com/orgs/mincong-classroom/teams/red
`,
			},
		},
		{
			name:       "no by default",
			team:       "red",
			input:      "\n",
			wantOutput: []string{"Provision the team red? (y/N): \nTeam \"red\" not provisioned.\n"},
		},
		{
			name:  "no",
			team:  "red",
			input: "n\n",
		},
		{
			name:       "an unexpected answer is asked again",
			team:       "red",
			input:      "all\ny\n",
			wantRan:    allRedCommands,
			wantOutput: []string{"Provision the team red? (y/N): Provision the team red? (y/N): ✓ "},
		},
		{
			name:       "end of input",
			team:       "red",
			wantOutput: []string{`Team "red" not provisioned.`},
		},
		{
			name:  "the steps already done are skipped",
			team:  "red",
			input: "y\n",
			setup: func(client *fakeClient) {
				provisioned(client)
				client.memberships["red/jdoe"] = ""
			},
			wantRan:    []github.Command{github.AddMemberCommand("red", "jdoe")},
			wantOutput: []string{`  - SMITH John (@jsmith): "John Smith" on GitHub, already a member` + "\n"},
		},
		{
			name:  "the admin access is replaced by push",
			team:  "red",
			input: "y\n",
			setup: func(client *fakeClient) {
				provisioned(client)
				client.roles["red/k8s-red"] = "admin"
			},
			wantRan: []github.Command{github.GrantPushCommand("red", "k8s-red")},
		},
		{
			name:  "a team already provisioned is not asked",
			team:  "red",
			setup: provisioned,
			wantOutput: []string{
				`  - DOE Jane (@jdoe): "Jane Doe" on GitHub, already invited` + "\n",
				"\nTeam \"red\" is already provisioned.\n- repo: https://github.com/mincong-classroom/k8s-red\n",
			},
			notOutput: "Provision the team",
		},
		{
			name:    "a registered team without members, none added",
			team:    "orange",
			input:   "\ny\n",
			wantRan: []github.Command{github.CreateRepoCommand("k8s-orange"), github.CreateTeamCommand("orange"), github.GrantPushCommand("orange", "k8s-orange")},
			wantOutput: []string{
				"The team orange has no members yet.\nStudents not in a team yet:\n   1. MARTIN Alex\n",
				"\n== Team orange: no members yet\nProvision the team orange? (y/N): ",
			},
		},
		{
			name:   "a new team, registered once confirmed",
			team:   "blue",
			input:  "1\namartin\ny\n",
			dryRun: true,
			wantOutput: []string{
				"The team blue is not in the registry yet.\n",
				`
== Team blue: 1 member
  - MARTIN Alex (@amartin): "Alex Martin" on GitHub
Provision the team blue? (y/N): (dry run) the team blue not saved to the registry
(dry run) Create the private repository mincong-classroom/k8s-blue from the template mincong-classroom/containers
          $ gh repo create mincong-classroom/k8s-blue --private --template mincong-classroom/containers
`,
				"\nDry run: team \"blue\" not provisioned, nothing changed on GitHub.\n",
			},
		},
		{
			name:       "an invalid new team name",
			team:       "Blue-2",
			wantErr:    "the team Blue-2 is not provisioned: fix its name",
			wantOutput: []string{`✗ invalid name "Blue-2"`},
			notOutput:  "Students not in a team yet",
		},
		{
			name:       "a team with errors is not asked",
			team:       "gray",
			wantErr:    "the team gray is not provisioned: fix the registry",
			wantOutput: []string{"✗ the GitHub user @ghost does not exist\n⚠ NOBODY Someone is not among the students of the registry\n"},
			notOutput:  "Provision the team",
		},
		{
			name:       "a failed step stops the provisioning",
			team:       "red",
			input:      "y\n",
			setup:      func(client *fakeClient) { client.runErr = errors.New("HTTP 403") },
			wantErr:    "the team red is not fully provisioned, provision it again: HTTP 403",
			wantOutput: []string{"✗ Create the private repository mincong-classroom/k8s-red from the template mincong-classroom/containers: HTTP 403\n"},
			notOutput:  "Create the secret GitHub team",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient()
			client.addUser("jsmith", "John Smith")
			client.addUser("jdoe", "Jane Doe")
			client.addUser("amartin", "Alex Martin")
			if tt.setup != nil {
				tt.setup(client)
			}
			p, out := newTestProvisioner(client, tt.input, tt.dryRun)
			load := func() (*common.TeamRegistry, error) { return newRegistry(), nil }

			err := p.run(tt.team, load)

			if gotErr := fmt.Sprint(err); (tt.wantErr == "" && err != nil) || !strings.Contains(gotErr, tt.wantErr) {
				t.Errorf("error = %v, want %q", err, tt.wantErr)
			}
			if !reflect.DeepEqual(client.ran, tt.wantRan) {
				t.Errorf("ran = %v, want %v", client.ran, tt.wantRan)
			}
			for _, want := range tt.wantOutput {
				if !strings.Contains(out.String(), want) {
					t.Errorf("output does not contain %q:\n%s", want, out)
				}
			}
			if tt.notOutput != "" && strings.Contains(out.String(), tt.notOutput) {
				t.Errorf("output contains %q:\n%s", tt.notOutput, out)
			}
		})
	}
}

func TestCommandString(t *testing.T) {
	got := github.CreateRepoCommand("k8s-red").String()
	want := "gh repo create mincong-classroom/k8s-red --private --template mincong-classroom/containers"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestPrepare(t *testing.T) {
	newRegistry := func() *common.TeamRegistry {
		return &common.TeamRegistry{
			Students: []common.Student{{Name: "SMITH John"}, {Name: "DOE Jane"}, {Name: "MARTIN Alex"}},
			Teams:    []common.Team{newTeam("red", member("SMITH John", "jsmith")), newTeam("green")},
		}
	}
	tests := []struct {
		name       string
		team       string
		input      string
		registry   *common.TeamRegistry
		wantErr    error
		wantTeam   common.Team
		wantChange string
		wantOutput []string
	}{
		{
			name:       "a new team, two students",
			team:       "purple",
			input:      "2 1\n@amartin\njdoe\n",
			wantTeam:   newTeam("purple", member("MARTIN Alex", "amartin"), member("DOE Jane", "jdoe")),
			wantChange: "the team purple",
			wantOutput: []string{
				`Students not in a team yet:
   1. DOE Jane
   2. MARTIN Alex
`,
				`@jdoe: no display name on GitHub`,
				`@amartin: "Alex Martin" on GitHub`,
			},
		},
		{
			name:       "a new team, no members yet",
			team:       "purple",
			input:      "\n",
			wantTeam:   newTeam("purple"),
			wantChange: "the team purple",
		},
		{
			name:       "a wrong pick, then a GitHub user not found",
			team:       "purple",
			input:      "3\n1,1\n1\nnobody\njdoe\n",
			wantTeam:   newTeam("purple", member("DOE Jane", "jdoe")),
			wantChange: "the team purple",
			wantOutput: []string{`✗ "3" is not a number between 1 and 2`, "✗ 1 is picked twice", "✗ the GitHub user @nobody does not exist"},
		},
		{
			name:       "no students to pick: the members are typed",
			team:       "purple",
			input:      "newcomer sam\nNEWCOMER Sam\n\njdoe\n",
			registry:   &common.TeamRegistry{},
			wantTeam:   newTeam("purple", member("NEWCOMER Sam", "jdoe")),
			wantChange: "the team purple",
			wantOutput: []string{"No student to pick in the registry: type the members instead.", `Member 1, "LAST First" (empty when done): `, `✗ "newcomer sam" is not "LAST First"`},
		},
		{
			name:       "a registered team without members gets them",
			team:       "green",
			input:      "1\njdoe\n",
			wantTeam:   newTeam("green", member("DOE Jane", "jdoe")),
			wantChange: "the members of green",
			wantOutput: []string{"The team green has no members yet.\n"},
		},
		{
			name:     "a registered team without members, none added",
			team:     "green",
			input:    "\n",
			wantTeam: newTeam("green"),
		},
		{
			name:     "a registered team with members is not asked",
			team:     "red",
			wantTeam: newTeam("red", member("SMITH John", "jsmith")),
		},
		{
			name:    "end of input",
			team:    "purple",
			input:   "1\n",
			wantErr: errQuit,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient()
			client.addUser("jdoe", "")
			client.addUser("amartin", "Alex Martin")
			p, out := newTestProvisioner(client, tt.input, true)
			registry := tt.registry
			if registry == nil {
				registry = newRegistry()
			}

			team, change, err := p.prepare(tt.team, registry)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if !reflect.DeepEqual(team, tt.wantTeam) {
				t.Errorf("team = %+v, want %+v", team, tt.wantTeam)
			}
			if got := describeChange(change); got != tt.wantChange {
				t.Errorf("change = %q, want %q", got, tt.wantChange)
			}
			if i := indexOf(registry.Teams, tt.team); i < 0 || !reflect.DeepEqual(registry.Teams[i], tt.wantTeam) {
				t.Errorf("the registry in memory does not have the team %+v: %+v", tt.wantTeam, registry.Teams)
			}
			for _, want := range tt.wantOutput {
				if !strings.Contains(out.String(), want) {
					t.Errorf("output does not contain %q:\n%s", want, out)
				}
			}
		})
	}
}

func TestPrepareRefusesAnInvalidName(t *testing.T) {
	for _, name := range []string{"Purple", "ls", "red"} {
		registry := &common.TeamRegistry{Teams: []common.Team{newTeam("red"), newTeam("red")}}
		p, out := newTestProvisioner(newFakeClient(), "\n", true)

		if _, _, err := p.prepare(name, registry); err == nil || !strings.Contains(out.String(), "✗ ") {
			t.Errorf("prepare(%q): error = %v, output:\n%s", name, err, out)
		}
	}
}

func TestParsePicks(t *testing.T) {
	tests := []struct {
		line    string
		want    []int
		wantErr bool
	}{
		{"", nil, false},
		{"1 3", []int{1, 3}, false},
		{" 3, 1 ", []int{3, 1}, false},
		{"0", nil, true},
		{"4", nil, true},
		{"x", nil, true},
		{"2 2", nil, true},
	}
	for _, tt := range tests {
		got, err := parsePicks(tt.line, 3)
		if (err != nil) != tt.wantErr || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("parsePicks(%q) = %v, %v; want %v, error: %v", tt.line, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestDryRunColorsTheCommands(t *testing.T) {
	p, out := newTestProvisioner(newFakeClient(), "", true)
	p.color = true

	if err := p.provision(stepsToRun(newTeam("red"), Status{})); err != nil {
		t.Fatalf("provision: %v", err)
	}

	if want := "          \033[33m$ gh repo create mincong-classroom/k8s-red"; !strings.Contains(out.String(), want) {
		t.Errorf("output does not contain the command in dark yellow:\n%q", out)
	}
}

func describeChange(change *registryChange) string {
	if change == nil {
		return ""
	}
	return change.what
}

func indexOf(teams []common.Team, name string) int {
	for i, team := range teams {
		if team.Name == name {
			return i
		}
	}
	return -1
}
