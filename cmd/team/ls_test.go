package team

import (
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
