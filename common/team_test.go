package common

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestAddTeam(t *testing.T) {
	tests := []struct {
		name     string
		registry string
		want     string
	}{
		{
			name: "after the other teams, keeping the comments",
			registry: `# The registry.
students:
  - name: "SMITH, John"
teams:
  # Created before the course.
  - name: green
    members: [] # not taken yet
`,
			want: `# The registry.
students:
  - name: "SMITH, John"
teams:
  # Created before the course.
  - name: green
    members: [] # not taken yet
  - name: red
    members:
      - name: "SMITH, John"
        github: jsmith
`,
		},
		{
			name: "into an empty list",
			registry: `students: []
teams: []
`,
			want: `students: []
teams:
  - name: red
    members:
      - name: "SMITH, John"
        github: jsmith
`,
		},
		{
			name: "without teams yet",
			registry: `students: []
`,
			want: `students: []
teams:
  - name: red
    members:
      - name: "SMITH, John"
        github: jsmith
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "teams.yaml")
			if err := os.WriteFile(file, []byte(tt.registry), 0o600); err != nil {
				t.Fatal(err)
			}
			TeamRegistryFile = file
			defer func() { TeamRegistryFile = "" }()

			err := AddTeam(Team{Name: "red", Members: []TeamMember{{Name: "SMITH, John", Github: "jsmith"}}})

			if err != nil {
				t.Fatalf("AddTeam: %v", err)
			}
			got, _ := os.ReadFile(file)
			if string(got) != tt.want {
				t.Errorf("registry =\n%s\nwant\n%s", got, tt.want)
			}
			if info, _ := os.Stat(file); info.Mode().Perm() != 0o600 {
				t.Errorf("permissions = %v, want the original ones", info.Mode().Perm())
			}
		})
	}
}

func TestSetTeamMembers(t *testing.T) {
	file := filepath.Join(t.TempDir(), "teams.yaml")
	registry := `students:
  - name: "SMITH, John"
teams:
  # Created before the course.
  - name: green
    members: [] # not taken yet
  - name: red
    members: []
`
	if err := os.WriteFile(file, []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}
	TeamRegistryFile = file
	defer func() { TeamRegistryFile = "" }()

	err := SetTeamMembers("green", []TeamMember{{Name: "SMITH, John", Github: "jsmith"}})

	if err != nil {
		t.Fatalf("SetTeamMembers: %v", err)
	}
	want := `students:
  - name: "SMITH, John"
teams:
  # Created before the course.
  - name: green
    members:
      - name: "SMITH, John"
        github: jsmith
  - name: red
    members: []
`
	if got, _ := os.ReadFile(file); string(got) != want {
		t.Errorf("registry =\n%s\nwant\n%s", got, want)
	}
	if err := SetTeamMembers("blue", nil); err == nil || !strings.Contains(err.Error(), `team "blue" not found`) {
		t.Errorf("SetTeamMembers of an unknown team: error = %v", err)
	}
}

func TestLoadRegistryRejectsAnUnknownKey(t *testing.T) {
	file := filepath.Join(t.TempDir(), "teams.yaml")
	if err := os.WriteFile(file, []byte("student:\n  - name: \"SMITH, John\"\nteams: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	TeamRegistryFile = file
	defer func() { TeamRegistryFile = "" }()

	_, err := LoadRegistry()

	if err == nil || !strings.Contains(err.Error(), `unknown key "student"`) {
		t.Errorf("LoadRegistry with a typo: error = %v", err)
	}
}
