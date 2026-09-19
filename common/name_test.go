package common

import (
	"strings"
	"testing"
)

func TestParseName(t *testing.T) {
	tests := []struct {
		name    string
		want    Name
		wantErr string
	}{
		{name: "SMITH John", want: Name{Last: "SMITH", First: "John"}},
		{name: "  SMITH   John ", want: Name{Last: "SMITH", First: "John"}},
		{name: "DE LA FONTAINE Jean Pierre", want: Name{Last: "DE LA FONTAINE", First: "Jean Pierre"}},
		{name: "DUPONT-MOREAU Jean-Pierre", want: Name{Last: "DUPONT-MOREAU", First: "Jean-Pierre"}},
		{name: "D'ALMEIDA Anne", want: Name{Last: "D'ALMEIDA", First: "Anne"}},
		{name: "ADÉ Soudémè Houéffa", want: Name{Last: "ADÉ", First: "Soudémè Houéffa"}},
		{name: "SMITH, John", wantErr: "remove the comma"},
		{name: "Smith John", wantErr: "it must start with the last name in upper case"},
		{name: "SMITH", wantErr: "the first name is missing"},
		{name: "SMITH JOHN", wantErr: "the first name is missing"},
		{name: "SMITH john", wantErr: `"john" is not in Pascal case`},
		{name: "SMITH JoHn", wantErr: `"JoHn" is not in Pascal case`},
		{name: "SMITH John DOE", wantErr: `"DOE" is not in Pascal case`},
		{name: "", wantErr: "it must start with the last name in upper case"},
	}
	for _, tt := range tests {
		got, err := ParseName(tt.name)
		if tt.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("ParseName(%q): error = %v, want %q", tt.name, err, tt.wantErr)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("ParseName(%q) = %+v, %v; want %+v", tt.name, got, err, tt.want)
		}
		if got.String() != strings.Join(strings.Fields(tt.name), " ") {
			t.Errorf("ParseName(%q).String() = %q", tt.name, got.String())
		}
	}
}

func TestSameName(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"SMITH John", "SMITH John", true},
		{"SMITH John", "smith  john", true},
		{"SMITH John", "SMITH, John", true}, // the former format
		{"SMITH John", "SMITH Jane", false},
	}
	for _, tt := range tests {
		if got := SameName(tt.a, tt.b); got != tt.want {
			t.Errorf("SameName(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
