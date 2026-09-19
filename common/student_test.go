package common

import (
	"reflect"
	"testing"
)

func TestParseStudents(t *testing.T) {
	input := `
students:
  - name: "SMITH, John"
  - name: "DOE, Jane Marie"
    email: ignored@example.org
`

	got, err := ParseStudents([]byte(input))

	if err != nil {
		t.Fatalf("ParseStudents: %v", err)
	}
	want := []Student{{Name: "SMITH, John"}, {Name: "DOE, Jane Marie"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseStudents = %v, want %v", got, want)
	}
}

func TestParseStudentsEmpty(t *testing.T) {
	got, err := ParseStudents([]byte("students: []\n"))

	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("ParseStudents = %#v, %v; want an empty, non-nil list", got, err)
	}
}

func TestParseStudentsInvalid(t *testing.T) {
	if _, err := ParseStudents([]byte("students: [")); err == nil {
		t.Error("ParseStudents with invalid YAML: want an error")
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
