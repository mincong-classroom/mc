package evaluation

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/mincong-classroom/mc/rubric"
)

func TestLoad(t *testing.T) {
	p := filepath.Join(t.TempDir(), "teal.yaml")
	if err := os.WriteFile(p, []byte(valid), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	want := &File{
		Team:        "teal",
		Commit:      "0a1b2c3",
		TimeSpent:   "Session only, done at 11:21",
		ActionItems: "Run it yourself",
		Rules: []Rule{ // In the order of the file, not of the rules.
			{
				ID:           "L2_POD",
				Completeness: 0.8,
				DoneWell:     "The Pod runs the image of Lab 1.",
				ToImprove:    "The report doesn't show the logs of the Pod.",
				Analysis:     "none",
				WritingStyle: "own words",
			},
			{
				ID:           "L2_LOG",
				Completeness: 1,
				DoneWell:     "The logs show the requests of the browser.",
				ToImprove:    "The report could say which command follows them.",
				Analysis:     "short",
				WritingStyle: "~30% AI-assisted",
				Penalty:      &Penalty{Kind: PenaltyLate, Remove: 0.1, Why: "Pushed after the deadline."},
			},
		},
	}
	got.lines = nil // The lines are for the problems only.
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load() =\n%+v\nwant\n%+v", got, want)
	}
}

func TestLoadAnEmptyFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "teal.yaml")
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(p)
	var problems Problems
	want := "1: the file is not a mapping of keys, such as team, commit and rules"
	if !errors.As(err, &problems) || err.Error() != want {
		t.Errorf("Load() of an empty file = %v, want the problem %q", err, want)
	}
}

func TestTeams(t *testing.T) {
	dir := t.TempDir()
	lab := LabDir(dir, rubric.Lab{ID: "L2"})
	if want := filepath.Join(dir, "lab-2"); lab != want {
		t.Errorf("LabDir() = %q, want %q", lab, want)
	}
	if got := Path(lab, "teal"); got != filepath.Join(dir, "lab-2", "teal.yaml") {
		t.Errorf("Path() = %q, want the file teal.yaml of %s", got, lab)
	}

	if got, err := Teams(lab); err != nil || got != nil {
		t.Errorf("Teams() of a directory that doesn't exist = %q, %v, want none", got, err)
	}

	for _, name := range []string{"teal.yaml", "coral.yaml", "notes.md", ".draft.yaml", "teal.yaml.bak"} {
		p := filepath.Join(lab, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(lab, "old.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Teams(lab)
	if want := []string{"coral", "teal"}; err != nil || !slices.Equal(got, want) {
		t.Errorf("Teams() = %q, %v, want %q", got, err, want)
	}
}
