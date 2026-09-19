package team

import (
	"reflect"
	"testing"

	"github.com/mincong-classroom/mc/common"
)

var testStudents = []common.Student{
	{Name: "SMITH, John"},
	{Name: "DOE, Jane"},
	{Name: "MARTIN, Alex"},
	{Name: "DURAND, Camille"},
}

func TestValidateTeam(t *testing.T) {
	client := newFakeClient()
	client.addUser("jsmith", "John Smith")
	client.addUser("jdoe", "")
	client.addUser("amartin", "Alex Martin")
	client.addUser("cdurand", "Camille Durand")
	client.addUser("teacher", "The Teacher")

	tests := []struct {
		name     string
		team     common.Team
		others   []common.Team
		students []common.Student
		want     []string
	}{
		{
			name:     "valid team",
			team:     newTeam("red", member("SMITH, John", "jsmith"), member("DOE, Jane", "jdoe")),
			students: testStudents,
		},
		{
			name:     "team without members",
			team:     newTeam("orange"),
			students: testStudents,
		},
		{
			name:     "name matched ignoring the case and the spaces",
			team:     newTeam("red", member("smith,  john", "jsmith")),
			students: testStudents,
		},
		{
			name:     "invalid name",
			team:     newTeam("north-1"),
			students: testStudents,
			want:     []string{`invalid name "north-1": use lowercase letters only, such as a color`},
		},
		{
			name:     "reserved name",
			team:     newTeam("ls"),
			students: testStudents,
			want:     []string{`invalid name "ls": reserved by the command "mc team ls"`},
		},
		{
			name:     "name used by two teams",
			team:     newTeam("red"),
			others:   []common.Team{newTeam("red")},
			students: testStudents,
			want:     []string{`the name "red" is used by 2 teams`},
		},
		{
			name: "three members",
			team: newTeam("red",
				member("SMITH, John", "jsmith"), member("DOE, Jane", "jdoe"), member("MARTIN, Alex", "amartin")),
			students: testStudents,
			want:     []string{"3 members, at most 2 are allowed"},
		},
		{
			name:     "member not on the school list",
			team:     newTeam("red", member("UNKNOWN, Someone", "jsmith")),
			students: testStudents,
			want:     []string{"UNKNOWN, Someone is not on the school list"},
		},
		{
			name:     "no school list",
			team:     newTeam("red", member("UNKNOWN, Someone", "jsmith")),
			students: nil,
		},
		{
			name:     "teacher not on the school list",
			team:     newTeam("teacher", member("TEACHER, The", "teacher")),
			students: testStudents,
		},
		{
			name:     "member in two teams",
			team:     newTeam("red", member("SMITH, John", "jsmith")),
			others:   []common.Team{newTeam("blue", member("SMITH, John", "jsmith"))},
			students: testStudents,
			want:     []string{"SMITH, John (@jsmith) is in several teams: red, blue"},
		},
		{
			name:     "GitHub username in two teams",
			team:     newTeam("red", member("SMITH, John", "jsmith")),
			others:   []common.Team{newTeam("blue", member("DOE, Jane", "JSmith"))},
			students: testStudents,
			want:     []string{"SMITH, John (@jsmith) is in several teams: red, blue"},
		},
		{
			name:     "GitHub user not found",
			team:     newTeam("red", member("SMITH, John", "jsmith-typo")),
			students: testStudents,
			want:     []string{"the GitHub user @jsmith-typo does not exist"},
		},
		{
			name:     "no GitHub username",
			team:     newTeam("red", member("SMITH, John", "")),
			students: testStudents,
			want:     []string{"SMITH, John has no GitHub username"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			teams := append([]common.Team{tt.team}, tt.others...)
			got := validateTeam(tt.team, teams, tt.students, client).Problems
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("problems = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateTeamUsers(t *testing.T) {
	client := newFakeClient()
	client.addUser("jsmith", "John Smith")
	team := newTeam("red", member("SMITH, John", "jsmith"))

	v := validateTeam(team, []common.Team{team}, testStudents, client)

	if got := describeGithubUser(v.Users["jsmith"]); got != `"John Smith" on GitHub` {
		t.Errorf("describeGithubUser = %q", got)
	}
}

func TestUnassignedStudents(t *testing.T) {
	teams := []common.Team{
		newTeam("red", member("SMITH, John", "jsmith"), member("doe, jane", "jdoe")),
		newTeam("orange"),
	}

	got := unassignedStudents(testStudents, teams)

	want := []common.Student{testStudents[2], testStudents[3]}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unassignedStudents = %v, want %v", got, want)
	}
}
