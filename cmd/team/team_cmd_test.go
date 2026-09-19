package team

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mincong-classroom/mc/common"
	"github.com/spf13/cobra"
)

func TestAddTeamCommands(t *testing.T) {
	root := newTeamRootCmd()

	addTeamCommands(root, []common.Team{
		newTeam("red", member("SMITH John", "jsmith")),
		newTeam("orange"),
		newTeam("red"),       // duplicated
		newTeam("ls"),        // reserved
		newTeam("two words"), // cannot be typed
	})

	var names []string
	for _, c := range root.Commands() {
		names = append(names, c.Name())
	}
	if want := []string{"ls", "orange", "provision", "red"}; !reflect.DeepEqual(names, want) {
		t.Errorf("commands = %v, want %v", names, want)
	}

	red, _, _ := root.Find([]string{"red"})
	if red.Short != "SMITH John (@jsmith)" {
		t.Errorf("red.Short = %q", red.Short)
	}
	for _, action := range []string{"validate", "status"} {
		cmd, _, err := root.Find([]string{"red", action})
		if err != nil || cmd.CommandPath() != "team red "+action {
			t.Errorf("Find(red %s) = %q, %v", action, cmd.CommandPath(), err)
		}
	}
}

func TestUnknownTeamOrAction(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("MC_YEAR", "2026")
	if err := os.MkdirAll(filepath.Join(home, ".mc"), 0o755); err != nil {
		t.Fatal(err)
	}
	registry := `teams:
  - name: red
`
	if err := os.WriteFile(filepath.Join(home, ".mc", "teams-2026.yaml"), []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		args []string
		want string
	}{
		{[]string{"blue", "status"}, `unknown team or action "blue"`},
		{[]string{"red", "delete"}, `unknown action "delete" for the team red`},
	}
	for _, tt := range tests {
		root := newTeamRootCmd()
		addTeamCommands(root, []common.Team{newTeam("red")})

		err := execute(root, tt.args...)

		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("mc team %v: error = %v, want %q", tt.args, err, tt.want)
		}
	}
}

func execute(root *cobra.Command, args ...string) error {
	root.SetArgs(args)
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	return root.Execute()
}
