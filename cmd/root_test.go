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
		{[]string{"team", "red", "provision", "--dry-run", "--team-file", "test.yaml"}, "test.yaml"},
		{[]string{"eval", "gather-facts", "-t", "red", "-l", "L1", "--team-file", "test.yaml"}, "test.yaml"},
	}
	for _, tt := range tests {
		if got := teamFileFromArgs(tt.args); got != tt.want {
			t.Errorf("teamFileFromArgs(%q) = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestEveryCommandHasAGroup(t *testing.T) {
	for _, c := range RootCmd.Commands() {
		if c.GroupID == "" {
			t.Errorf("mc %s has no group: mc --help lists it apart, under Additional Commands", c.Name())
		}
	}
}

func TestGatherFactsDirShadowsTheOneOfEval(t *testing.T) {
	flag := evalGatherFactsCmd.Flags().Lookup("dir")
	if flag == nil || flag.Usage != "The directory of the worktrees, one per team (default: ~/.mc/{year}/teacher)" {
		t.Fatalf("the flag --dir of mc eval gather-facts = %+v, want the directory of the worktrees", flag)
	}
	if evalGatherFactsCmd.InheritedFlags().Lookup("dir") != nil {
		t.Errorf("mc eval gather-facts inherits the flag --dir of mc eval, the directory of the evaluations")
	}
}
