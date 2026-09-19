package team

import (
	"bufio"
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

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
			name:    "invalid answers are asked again",
			input:   "\nmaybe\ny\ny\ny\ny\ny\n",
			wantRan: allRedCommands,
		},
		{
			name:    "all",
			input:   "y\na\n",
			wantRan: allRedCommands,
		},
		{
			name:    "no skips the remaining steps",
			input:   "y\nn\n",
			wantErr: errSkipped,
			wantRan: allRedCommands[:1],
		},
		{
			name:    "quit",
			input:   "q\n",
			wantErr: errQuit,
		},
		{
			name:    "end of input quits",
			input:   "y\n",
			wantErr: errQuit,
			wantRan: allRedCommands[:1],
		},
		{
			name:   "dry run runs nothing",
			input:  "a\n",
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

func TestCommandString(t *testing.T) {
	got := github.CreateRepoCommand("k8s-red").String()
	want := "gh repo create mincong-classroom/k8s-red --private --template mincong-classroom/containers"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
