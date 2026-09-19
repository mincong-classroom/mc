package common

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFilterTeams(t *testing.T) {
	teams := []Team{{Name: "red"}, {Name: "blue"}}

	got, err := FilterTeams(teams, []string{"blue"})
	if err != nil || len(got) != 1 || got[0].Name != "blue" {
		t.Errorf("FilterTeams = %v, %v", got, err)
	}

	if _, err := FilterTeams(teams, []string{"green"}); err == nil {
		t.Error("FilterTeams with an unknown team: want an error")
	}
}

func TestTeamRegistryPath(t *testing.T) {
	t.Setenv("HOME", "/home/teacher")
	t.Setenv("MC_YEAR", "2026")
	tests := []struct {
		file string
		want string
	}{
		{"", "/home/teacher/.mc/teams-2026.yaml"},
		{"/tmp/test-teams.yaml", "/tmp/test-teams.yaml"},
		{"~/.mc/test-teams-2026.yaml", "/home/teacher/.mc/test-teams-2026.yaml"},
	}
	for _, tt := range tests {
		TeamRegistryFile = tt.file
		if got := TeamRegistryPath(); got != tt.want {
			t.Errorf("TeamRegistryPath() with %q = %q, want %q", tt.file, got, tt.want)
		}
	}
	TeamRegistryFile = ""
}

func TestLoadRegistry(t *testing.T) {
	file := filepath.Join(t.TempDir(), "teams.yaml")
	data := `
students:
  - name: "SMITH, John"
  - name: "DOE, Jane"
    email: ignored@example.org
teams:
  - name: red
    members:
      - name: "SMITH, John"
        github: jsmith
`
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	TeamRegistryFile = file
	defer func() { TeamRegistryFile = "" }()

	registry, err := LoadRegistry()

	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if want := []Student{{Name: "SMITH, John"}, {Name: "DOE, Jane"}}; !reflect.DeepEqual(registry.Students, want) {
		t.Errorf("students = %v, want %v", registry.Students, want)
	}
	if len(registry.Teams) != 1 || registry.Teams[0].Members[0].Github != "jsmith" {
		t.Errorf("teams = %+v", registry.Teams)
	}
}
