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

var redTeam = newTeam("red", member("SMITH, John", "jsmith"), member("DOE, Jane", "jdoe"))

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

func TestProvision(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		dryRun  bool
		wantErr error
		wantRan []github.Command
	}{
		{
			name:    "yes to every step",
			input:   "y\ny\ny\nyes\nY\n",
			wantRan: allRedCommands,
		},
		{
			name:    "unexpected answers are asked again",
			input:   "maybe\nall\ny\ny\ny\ny\ny\n",
			wantRan: allRedCommands,
		},
		{
			name:    "no by default",
			input:   "y\n\n",
			wantErr: errSkipped,
			wantRan: allRedCommands[:1],
		},
		{
			name:    "no skips the remaining steps",
			input:   "y\nn\n",
			wantErr: errSkipped,
			wantRan: allRedCommands[:1],
		},
		{
			name:    "no quit answer",
			input:   "q\n",
			wantErr: errQuit, // "q" is asked again, then the input is over
		},
		{
			name:    "end of input quits",
			input:   "y\n",
			wantErr: errQuit,
			wantRan: allRedCommands[:1],
		},
		{
			name:   "dry run runs nothing",
			input:  "y\ny\ny\ny\ny\n",
			dryRun: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient()
			p, _ := newTestProvisioner(client, tt.input, tt.dryRun)

			err := p.provision(redTeam, nil)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(client.ran, tt.wantRan) {
				t.Errorf("ran = %v, want %v", client.ran, tt.wantRan)
			}
		})
	}
}

func TestProvisionSkipsTheStepsAlreadyDone(t *testing.T) {
	client := newFakeClient()
	client.repos["k8s-red"] = true
	client.teams["red"] = true
	client.roles["red/k8s-red"] = "write"
	client.memberships["red/jsmith"] = "pending"
	p, out := newTestProvisioner(client, "y\n", false)

	if err := p.provision(redTeam, nil); err != nil {
		t.Fatalf("provision: %v", err)
	}

	want := []github.Command{github.AddMemberCommand("red", "jdoe")}
	if !reflect.DeepEqual(client.ran, want) {
		t.Errorf("ran = %v, want %v", client.ran, want)
	}
	if !strings.Contains(out.String(), "✓ already invited (pending)") {
		t.Errorf("output does not show the pending invitation:\n%s", out)
	}
}

func TestProvisionReplacesTheAdminAccess(t *testing.T) {
	client := newFakeClient()
	client.repos["k8s-red"] = true
	client.teams["red"] = true
	client.roles["red/k8s-red"] = "admin"
	p, _ := newTestProvisioner(client, "y\n", false)

	if err := p.provision(newTeam("red"), nil); err != nil {
		t.Fatalf("provision: %v", err)
	}

	want := []github.Command{github.GrantPushCommand("red", "k8s-red")}
	if !reflect.DeepEqual(client.ran, want) {
		t.Errorf("ran = %v, want %v", client.ran, want)
	}
}

func TestRun(t *testing.T) {
	registry := &common.TeamRegistry{
		Students: []common.Student{{Name: "SMITH, John"}}, // DOE, Jane is not among them
		Teams:    []common.Team{redTeam, newTeam("orange"), newTeam("Bad")},
	}
	load := func() (*common.TeamRegistry, error) {
		return registry, nil
	}
	orangeCommands := []github.Command{
		github.CreateRepoCommand("k8s-orange"),
		github.CreateTeamCommand("orange"),
		github.GrantPushCommand("orange", "k8s-orange"),
	}
	tests := []struct {
		name       string
		input      string
		wantErr    string
		wantRan    []github.Command
		wantOutput string
	}{
		{
			name:       "one team without members, provisioned",
			input:      "orange\nn\ny\ny\ny\n",
			wantRan:    orangeCommands,
			wantOutput: "\nTeam \"orange\" provisioned.\n- repo: https://github.com/mincong-classroom/k8s-orange\n- team: https://github.com/orgs/mincong-classroom/teams/orange\n",
		},
		{
			name:  "end of input quits",
			input: "",
		},
		{
			name:       "an empty answer asks again",
			input:      "\n\norange\nn\ny\ny\ny\n",
			wantRan:    orangeCommands,
			wantOutput: "Team to provision: Team to provision: Team to provision: ",
		},
		{
			name:       "a new team not registered, then a registered team",
			input:      "blue\nn\norange\nn\nn\n",
			wantOutput: "blue is not in the registry. Register it?",
		},
		{
			name:       "an invalid new team name",
			input:      "Blue-2\n\n",
			wantOutput: `✗ invalid name "Blue-2"`,
		},
		{
			name:       "a warning is confirmed before the steps",
			input:      "red\ny\ny\ny\ny\ny\ny\n",
			wantRan:    allRedCommands,
			wantOutput: "⚠ DOE, Jane is not among the students of the registry\nProvision it anyway?",
		},
		{
			name:  "a warning declined skips the team",
			input: "red\nn\n\n",
		},
		{
			name:       "a warning is not confirmed by default",
			input:      "red\n\n",
			wantOutput: "Provision it anyway? (y/N): \nTeam \"red\" not provisioned.\n",
		},
		{
			name:       "invalid team is not provisioned",
			input:      "Bad\n\n",
			wantErr:    "the team Bad is not provisioned: fix the registry",
			wantOutput: `invalid name "Bad"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient()
			client.addUser("jsmith", "John Smith")
			client.addUser("jdoe", "Jane Doe")
			p, out := newTestProvisioner(client, tt.input, false)

			err := p.run(load)

			if gotErr := fmt.Sprint(err); (tt.wantErr == "" && err != nil) || !strings.Contains(gotErr, tt.wantErr) {
				t.Errorf("error = %v, want %q", err, tt.wantErr)
			}
			if !reflect.DeepEqual(client.ran, tt.wantRan) {
				t.Errorf("ran = %v, want %v", client.ran, tt.wantRan)
			}
			if !strings.Contains(out.String(), tt.wantOutput) {
				t.Errorf("output does not contain %q:\n%s", tt.wantOutput, out)
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

func TestRegisterTeam(t *testing.T) {
	newRegistry := func() *common.TeamRegistry {
		return &common.TeamRegistry{
			Students: []common.Student{{Name: "SMITH, John"}, {Name: "DOE, Jane"}, {Name: "MARTIN, Alex"}},
			Teams:    []common.Team{newTeam("red", member("SMITH, John", "jsmith"))},
		}
	}
	tests := []struct {
		name       string
		input      string
		registry   *common.TeamRegistry
		wantErr    error
		wantTeam   common.Team
		wantOutput []string
	}{
		{
			name:     "two students",
			input:    "y\n2 1\n@amartin\njdoe\n",
			wantTeam: newTeam("purple", member("MARTIN, Alex", "amartin"), member("DOE, Jane", "jdoe")),
			wantOutput: []string{
				"Students not in a team yet:\n   1. DOE, Jane\n   2. MARTIN, Alex\n",
				`@jdoe: no display name on GitHub`,
				`@amartin: "Alex Martin" on GitHub`,
				"(dry run) the team purple not saved to the registry",
			},
		},
		{
			name:     "no members yet",
			input:    "y\n\n",
			wantTeam: newTeam("purple"),
		},
		{
			name:       "a wrong pick, then a GitHub user not found",
			input:      "y\n3\n1,1\n1\nnobody\njdoe\n",
			wantTeam:   newTeam("purple", member("DOE, Jane", "jdoe")),
			wantOutput: []string{`✗ "3" is not a number between 1 and 2`, "✗ 1 is picked twice", "✗ the GitHub user @nobody does not exist"},
		},
		{
			name:       "no students to pick: the members are typed",
			input:      "y\nNEWCOMER, Sam\n\njdoe\n",
			registry:   &common.TeamRegistry{},
			wantTeam:   newTeam("purple", member("NEWCOMER, Sam", "jdoe")),
			wantOutput: []string{"No student to pick in the registry: type the members instead.", `Member 1, "LAST, First" (empty when done): `},
		},
		{
			name:    "declined",
			input:   "n\n",
			wantErr: errSkipped,
		},
		{
			name:    "declined by default",
			input:   "\n",
			wantErr: errSkipped,
		},
		{
			name:       "an unexpected answer is asked again",
			input:      "red\ny\n\n",
			wantTeam:   newTeam("purple"),
			wantOutput: []string{"Register it? (y/N): purple is not in the registry. Register it? (y/N): "},
		},
		{
			name:    "end of input",
			input:   "y\n1\n",
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

			team, err := p.registerTeam("purple", registry)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if !reflect.DeepEqual(team, tt.wantTeam) {
				t.Errorf("team = %+v, want %+v", team, tt.wantTeam)
			}
			if last := registry.Teams[len(registry.Teams)-1]; last.Name != "purple" {
				t.Errorf("the team is not added to the registry in memory: %+v", registry.Teams)
			}
			for _, want := range tt.wantOutput {
				if !strings.Contains(out.String(), want) {
					t.Errorf("output does not contain %q:\n%s", want, out)
				}
			}
		})
	}
}

func TestRegisterTeamRefusesAnInvalidName(t *testing.T) {
	for _, name := range []string{"Purple", "ls", "red"} {
		registry := &common.TeamRegistry{Teams: []common.Team{newTeam("red"), newTeam("red")}}
		p, out := newTestProvisioner(newFakeClient(), "y\n", true)

		if _, err := p.registerTeam(name, registry); !errors.Is(err, errSkipped) || !strings.Contains(out.String(), "✗ ") {
			t.Errorf("registerTeam(%q): error = %v, output:\n%s", name, err, out)
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

func TestCompleteTeam(t *testing.T) {
	newRegistry := func() *common.TeamRegistry {
		return &common.TeamRegistry{
			Students: []common.Student{{Name: "SMITH, John"}, {Name: "DOE, Jane"}},
			Teams:    []common.Team{newTeam("green"), newTeam("red", member("DOE, Jane", "jdoe"))},
		}
	}
	tests := []struct {
		name       string
		index      int
		input      string
		wantTeam   common.Team
		wantOutput string
	}{
		{
			name:       "members added to a team without members",
			input:      "y\n1\njsmith\n",
			wantTeam:   newTeam("green", member("SMITH, John", "jsmith")),
			wantOutput: "(dry run) the members of green not saved to the registry",
		},
		{
			name:     "no members added",
			input:    "n\n",
			wantTeam: newTeam("green"),
		},
		{
			name:     "a team with members is not asked",
			index:    1,
			wantTeam: newTeam("red", member("DOE, Jane", "jdoe")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient()
			client.addUser("jsmith", "John Smith")
			p, out := newTestProvisioner(client, tt.input, true)
			registry := newRegistry()

			team, err := p.completeTeam(tt.index, registry)

			if err != nil {
				t.Fatalf("completeTeam: %v", err)
			}
			if !reflect.DeepEqual(team, tt.wantTeam) || !reflect.DeepEqual(registry.Teams[tt.index], tt.wantTeam) {
				t.Errorf("team = %+v, in the registry %+v, want %+v", team, registry.Teams[tt.index], tt.wantTeam)
			}
			if !strings.Contains(out.String(), tt.wantOutput) {
				t.Errorf("output does not contain %q:\n%s", tt.wantOutput, out)
			}
		})
	}
}

func TestProvisionColorsTheCommands(t *testing.T) {
	p, out := newTestProvisioner(newFakeClient(), "n\n", false)
	p.color = true

	_ = p.provision(newTeam("red"), nil)

	if want := "      \033[33m$ gh repo create mincong-classroom/k8s-red"; !strings.Contains(out.String(), want) {
		t.Errorf("output does not contain the command in dark yellow:\n%q", out)
	}
}
