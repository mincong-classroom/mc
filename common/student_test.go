package common

import "testing"

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
