package team

import (
	"reflect"
	"testing"

	"github.com/mincong-classroom/mc/common"
)

func TestValidateTeam(t *testing.T) {
	client := newFakeClient()
	client.addUser("jsmith", "John Smith")
	client.addUser("jdoe", "")
	client.addUser("amartin", "Alex Martin")
	students := []common.Student{{Name: "SMITH John"}, {Name: "DOE Jane"}, {Name: "MARTIN Alex"}}

	tests := []struct {
		name         string
		team         common.Team
		others       []common.Team
		noStudents   bool
		wantErrors   []string
		wantWarnings []string
	}{
		{
			name: "valid team",
			team: newTeam("red", member("SMITH John", "jsmith"), member("DOE Jane", "jdoe")),
		},
		{
			name: "team without members",
			team: newTeam("orange"),
		},
		{
			name: "student matched ignoring the extra spaces",
			team: newTeam("red", member("SMITH   John", "jsmith")),
		},
		{
			name:         "name not in the format LAST First",
			team:         newTeam("red", member("smith john", "jsmith")),
			wantWarnings: []string{`"smith john" is not "LAST First": it must start with the last name in upper case`},
		},
		{
			name:         "name in the former format LAST, First",
			team:         newTeam("red", member("SMITH, John", "jsmith")),
			wantWarnings: []string{`"SMITH, John" is not "LAST First": remove the comma`},
		},
		{
			name:         "member not among the students",
			team:         newTeam("red", member("NEWCOMER Sam", "jsmith")),
			wantWarnings: []string{"NEWCOMER Sam is not among the students of the registry"},
		},
		{
			name:       "registry without students",
			team:       newTeam("red", member("NEWCOMER Sam", "jsmith")),
			noStudents: true,
		},
		{
			name:       "invalid name",
			team:       newTeam("north-1"),
			wantErrors: []string{`invalid name "north-1": use lowercase letters only, such as a color`},
		},
		{
			name:       "reserved name",
			team:       newTeam("ls"),
			wantErrors: []string{`invalid name "ls": reserved by the command "mc team ls"`},
		},
		{
			name:       "name used by two teams",
			team:       newTeam("red"),
			others:     []common.Team{newTeam("red")},
			wantErrors: []string{`the name "red" is used by 2 teams`},
		},
		{
			name: "three members",
			team: newTeam("red",
				member("SMITH John", "jsmith"), member("DOE Jane", "jdoe"), member("MARTIN Alex", "amartin")),
			wantWarnings: []string{"3 members, at most 2 are expected"},
		},
		{
			name:         "member in two teams",
			team:         newTeam("red", member("SMITH John", "jsmith")),
			others:       []common.Team{newTeam("blue", member("SMITH John", "jsmith"))},
			wantWarnings: []string{"SMITH John (@jsmith) is in several teams: red, blue"},
		},
		{
			name:         "GitHub username in two teams",
			team:         newTeam("red", member("SMITH John", "jsmith")),
			others:       []common.Team{newTeam("blue", member("DOE Jane", "JSmith"))},
			wantWarnings: []string{"SMITH John (@jsmith) is in several teams: red, blue"},
		},
		{
			name:       "GitHub user not found",
			team:       newTeam("red", member("SMITH John", "jsmith-typo")),
			wantErrors: []string{"the GitHub user @jsmith-typo does not exist"},
		},
		{
			name:       "no GitHub username",
			team:       newTeam("red", member("SMITH John", "")),
			wantErrors: []string{"SMITH John has no GitHub username"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := &common.TeamRegistry{Students: students, Teams: append([]common.Team{tt.team}, tt.others...)}
			if tt.noStudents {
				registry.Students = nil
			}

			v := validateTeam(tt.team, registry, client)

			if !reflect.DeepEqual(v.Errors, tt.wantErrors) {
				t.Errorf("errors = %q, want %q", v.Errors, tt.wantErrors)
			}
			if !reflect.DeepEqual(v.Warnings, tt.wantWarnings) {
				t.Errorf("warnings = %q, want %q", v.Warnings, tt.wantWarnings)
			}
		})
	}
}

func TestValidateTeamUsers(t *testing.T) {
	client := newFakeClient()
	client.addUser("jsmith", "John Smith")
	team := newTeam("red", member("SMITH John", "jsmith"))

	v := validateTeam(team, &common.TeamRegistry{Teams: []common.Team{team}}, client)

	if got := describeGithubUser(v.Users["jsmith"]); got != `"John Smith" on GitHub` {
		t.Errorf("describeGithubUser = %q", got)
	}
}
