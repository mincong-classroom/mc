package evaluation

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mincong-classroom/mc/rubric"
)

func TestStepsRoundTrip(t *testing.T) {
	labDir := filepath.Join(t.TempDir(), "lab-2")

	// No record yet: every step runs.
	s, err := ReadSteps(labDir, "teal")
	if err != nil || s.Report != nil || s.Grade != nil {
		t.Fatalf("ReadSteps without a record = %+v, %v, want an empty record", s, err)
	}

	done := time.Date(2030, 3, 4, 9, 30, 0, 0, time.UTC)
	want := Steps{Report: &StepRecord{
		Inputs: Inputs{Facts: "f1", Files: map[string]string{"skills/report-lab/SKILL.md": "s1"}},
		Done:   done,
	}}
	if err := WriteSteps(labDir, "teal", want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadSteps(labDir, "teal")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("ReadSteps = %+v, %v, want %+v", got, err, want)
	}

	// The record is not a team, and leaves no temporary file.
	if teams, err := Teams(labDir); err != nil || len(teams) != 0 {
		t.Errorf("Teams of a directory with records only = %v, %v, want none", teams, err)
	}
	if _, err := os.Stat(StepsPath(labDir, "teal") + ".tmp"); err == nil {
		t.Error("WriteSteps left its temporary file")
	}
}

func TestReadStepsOfABrokenRecord(t *testing.T) {
	labDir := t.TempDir()
	file := StepsPath(labDir, "teal")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSteps(labDir, "teal"); err == nil || !strings.Contains(err.Error(), "delete it to run every step again") {
		t.Errorf("ReadSteps of a broken record = %v, want an error that says to delete it", err)
	}
}

func TestChanges(t *testing.T) {
	former := Inputs{
		Facts:  "f1",
		Report: "r1",
		Rules:  "l1",
		Files:  map[string]string{"skills/a.md": "a1", "skills/b.md": "b1", "skills/c.md": "c1"},
	}
	for _, c := range []struct {
		name string
		now  Inputs
		want []string
	}{
		{name: "the same", now: former},
		{
			name: "everything",
			now: Inputs{
				Facts:  "f2",
				Report: "r2",
				Rules:  "l2",
				Files:  map[string]string{"skills/a.md": "a2", "skills/c.md": "c1", "skills/d.md": "d1"},
			},
			want: []string{
				"the facts changed", "the report changed", "the rules of the lab changed",
				"skills/a.md changed", "skills/b.md removed", "skills/d.md added",
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := Changes(former, c.now); !reflect.DeepEqual(got, c.want) {
				t.Errorf("Changes = %q, want %q", got, c.want)
			}
		})
	}
}

func TestDigestFiles(t *testing.T) {
	root := t.TempDir()
	for file, content := range map[string]string{
		"skills/report-lab/SKILL.md":     "# Report",
		"skills/report-lab/template.md":  "# Template",
		"skills/report-lab/.DS_Store":    "hidden",
		"skills/report-lab/.cache/a.txt": "hidden too",
		"rules.yaml":                     "rules: {}",
	} {
		p := filepath.Join(root, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := DigestFiles(root, []string{"skills/report-lab", "rules.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"rules.yaml", "skills/report-lab/SKILL.md", "skills/report-lab/template.md"}
	if keys := sortedKeys(got); !reflect.DeepEqual(keys, want) {
		t.Errorf("the files of DigestFiles = %q, want %q", keys, want)
	}
	if got["skills/report-lab/SKILL.md"] == got["skills/report-lab/template.md"] {
		t.Error("2 files of another content have the same digest")
	}

	if _, err := DigestFiles(root, []string{"skills/reprot-lab"}); err == nil || !strings.Contains(err.Error(), "the input skills/reprot-lab") {
		t.Errorf("DigestFiles of an input that doesn't exist = %v, want an error that names it", err)
	}
	if files, err := DigestFiles(root, nil); err != nil || files != nil {
		t.Errorf("DigestFiles without an input = %v, %v, want none", files, err)
	}
}

func TestDigestRules(t *testing.T) {
	rules := func(edit func(r *rubric.Rubric)) string {
		t.Helper()
		r, err := rubric.Load("../rubric/testdata/evaluation-rules.yaml")
		if err != nil {
			t.Fatal(err)
		}
		edit(r)
		d, err := DigestRules(r, "L2")
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	ruleOf := func(r *rubric.Rubric, id string) *rubric.Rule {
		for i := range r.Rules {
			if r.Rules[i].ID == id {
				return &r.Rules[i]
			}
		}
		t.Fatalf("no rule %s in the test rules", id)
		return nil
	}
	same := rules(func(*rubric.Rubric) {})

	for name, edit := range map[string]func(r *rubric.Rubric){
		"a coefficient of the lab": func(r *rubric.Rubric) { ruleOf(r, "L2_POD").Coefficient = 0.9 },
		"a rule of another lab":    func(r *rubric.Rubric) { ruleOf(r, "L1_BLD").Description = "Another text." },
	} {
		if got := rules(edit); got != same {
			t.Errorf("DigestRules after a change of %s = %s, want the same digest %s", name, got, same)
		}
	}
	for name, edit := range map[string]func(r *rubric.Rubric){
		"a description of the lab":    func(r *rubric.Rubric) { ruleOf(r, "L2_POD").Description = "Another text." },
		"the penalties of every rule": func(r *rubric.Rubric) { r.Scoring = rubric.Scoring{} },
	} {
		if got := rules(edit); got == same {
			t.Errorf("DigestRules after a change of %s = the same digest, want another one", name)
		}
	}
}
