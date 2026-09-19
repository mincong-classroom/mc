package team

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/mincong-classroom/mc/common"
)

var testStudents = []common.Student{
	{Name: "SMITH John"},
	{Name: "DOE Jane"},
	{Name: "MARTIN Alex"},
	{Name: "DURAND Camille"},
}

func TestUnassignedStudents(t *testing.T) {
	teams := []common.Team{
		newTeam("red", member("SMITH John", "jsmith"), member("doe, jane", "jdoe")),
		newTeam("orange"),
	}

	got := unassignedStudents(testStudents, teams)

	want := []common.Student{testStudents[2], testStudents[3]}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unassignedStudents = %v, want %v", got, want)
	}
}

func TestPrintUnassignedStudents(t *testing.T) {
	tests := []struct {
		students []common.Student
		want     string
	}{
		{
			students: []common.Student{{Name: "SMITH John"}},
			want:     "The only student is in a team.\n",
		},
		{
			students: []common.Student{{Name: "SMITH John"}, {Name: "DOE Jane"}},
			want:     "All the 2 students are in a team.\n",
		},
		{
			students: []common.Student{{Name: "SMITH John"}, {Name: "MARTIN Alex"}},
			want: `1 of 2 students not in a team yet:
  - MARTIN Alex
`,
		},
	}
	teams := []common.Team{newTeam("red", member("SMITH John", "jsmith"), member("DOE Jane", "jdoe"))}
	for _, tt := range tests {
		var out bytes.Buffer
		printUnassignedStudents(&out, tt.students, teams)
		if out.String() != tt.want {
			t.Errorf("printUnassignedStudents(%v) = %q, want %q", tt.students, out.String(), tt.want)
		}
	}
}
