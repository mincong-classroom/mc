package common

import "testing"

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
