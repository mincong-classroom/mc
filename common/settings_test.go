package common

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadSettings(t *testing.T) {
	for _, c := range []struct {
		name    string
		content string // The file of the settings; none when empty.
		want    Settings
		wantErr string // A part of the error, none when empty.
	}{
		{name: "no file"},
		{name: "empty file", content: "# Nothing set yet.\n"},
		{
			name:    "year and rules",
			content: "year: 2030\nrules: ~/course/{year}/evaluation-rules.yaml\n",
			want:    Settings{Year: 2030, Rules: "~/course/{year}/evaluation-rules.yaml"},
		},
		{
			name:    "agents",
			content: "agents:\n  eval_run: claude -p \"/evaluate-lab {lab} {teams}\"\n",
			want:    Settings{Agents: Agents{EvalRun: `claude -p "/evaluate-lab {lab} {teams}"`}},
		},
		{name: "unknown key", content: "yaer: 2030\n", wantErr: "field yaer not found"},
		{name: "unknown agent", content: "agents:\n  eval_rnu: claude\n", wantErr: "field eval_rnu not found"},
		{name: "not a year", content: "year: -1\n", wantErr: "the year -1 is not a year"},
	} {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			if c.content != "" {
				if err := os.Mkdir(filepath.Join(home, ".mc"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(home, ".mc", "settings.yaml"), []byte(c.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := ReadSettings()
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Errorf("ReadSettings() = %v, want an error with %q", err, c.wantErr)
				}
				return
			}
			if err != nil || got != c.want {
				t.Errorf("ReadSettings() = %+v, %v, want %+v", got, err, c.want)
			}
		})
	}
}

func TestRulesOf(t *testing.T) {
	t.Setenv("HOME", "/home/teacher")
	got, err := Settings{Rules: "~/course/{year}/evaluation-rules.yaml"}.RulesOf(2030)
	if want := "/home/teacher/course/2030/evaluation-rules.yaml"; err != nil || got != want {
		t.Errorf("RulesOf(2030) = %q, %v, want %q", got, err, want)
	}
	if _, err := (Settings{}).RulesOf(2030); err == nil || !strings.Contains(err.Error(), "--rules-file") {
		t.Errorf("RulesOf(2030) without rules = %v, want an error that names --rules-file", err)
	}
}

func TestRulesPathOfAFile(t *testing.T) {
	t.Setenv("HOME", "/home/teacher")
	for file, want := range map[string]string{
		"~/rules.yaml":         "/home/teacher/rules.yaml",
		"testdata/rules.yaml":  "testdata/rules.yaml",
		"/tmp/rules-2030.yaml": "/tmp/rules-2030.yaml",
	} {
		if got, err := RulesPath(file); err != nil || got != want {
			t.Errorf("RulesPath(%q) = %q, %v, want %q", file, got, err, want)
		}
	}
}
