package cmd

import "testing"

func TestTeamFileFromArgs(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"team", "ls"}, ""},
		{[]string{"team", "ls", "--team-file", "test.yaml"}, "test.yaml"},
		{[]string{"--team-file=test.yaml", "team", "red", "status"}, "test.yaml"},
		{[]string{"team", "provision", "--dry-run", "--team-file", "test.yaml"}, "test.yaml"},
		{[]string{"grade", "-t", "red", "-l", "L1", "--team-file", "test.yaml"}, "test.yaml"},
	}
	for _, tt := range tests {
		if got := teamFileFromArgs(tt.args); got != tt.want {
			t.Errorf("teamFileFromArgs(%q) = %q, want %q", tt.args, got, tt.want)
		}
	}
}
