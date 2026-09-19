package common

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseStudents(t *testing.T) {
	input := "SMITH\tJohn\n\nDOE\tJane Marie\tignored@example.org\r\n"

	got, err := ParseStudents(strings.NewReader(input))

	if err != nil {
		t.Fatalf("ParseStudents: %v", err)
	}
	want := []Student{
		{LastName: "SMITH", FirstName: "John"},
		{LastName: "DOE", FirstName: "Jane Marie"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseStudents = %v, want %v", got, want)
	}
	if got[1].FullName() != "DOE, Jane Marie" {
		t.Errorf("FullName = %q", got[1].FullName())
	}
}

func TestParseStudentsRejectsALineWithOneColumn(t *testing.T) {
	_, err := ParseStudents(strings.NewReader("SMITH\tJohn\nDOE Jane\n"))

	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Errorf("error = %v, want an error on line 2", err)
	}
}

func TestSameName(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"SMITH, John", "SMITH, John", true},
		{"SMITH, John", "smith,john", true},
		{"SMITH ,  John", "SMITH, John", true},
		{"SMITH, John", "SMITH, Jane", false},
	}
	for _, tt := range tests {
		if got := SameName(tt.a, tt.b); got != tt.want {
			t.Errorf("SameName(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

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
