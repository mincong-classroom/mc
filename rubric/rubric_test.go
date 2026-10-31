package rubric

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update the golden files of testdata")

const fixture = "testdata/evaluation-rules.yaml"

func mustLoad(t *testing.T) *Rubric {
	t.Helper()
	r, err := Load(fixture)
	if err != nil {
		t.Fatalf("Load(%q): %v", fixture, err)
	}
	return r
}

func TestLoadKeepsTheOrderOfTheFile(t *testing.T) {
	r := mustLoad(t)

	var labs, rules, kinds, levels []string
	for _, lab := range r.Labs {
		labs = append(labs, lab.ID)
	}
	for _, rule := range r.Rules {
		rules = append(rules, rule.ID)
	}
	for _, kind := range r.Evidence.Kinds {
		kinds = append(kinds, kind.Key)
	}
	for _, level := range r.AnalysisLevels {
		levels = append(levels, level.Key)
	}
	for _, c := range []struct {
		name      string
		got, want []string
	}{
		{"labs", labs, []string{"L1", "L2"}},
		{"rules", rules, []string{"L1_BLD", "L1_TAG", "L2_POD", "L2_LOG"}},
		{"evidence.kinds", kinds, []string{"command", "output", "file", "image"}},
		{"analysis_levels", levels, []string{"none", "short"}},
	} {
		if !slices.Equal(c.got, c.want) {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

func TestLoadReadsTheFields(t *testing.T) {
	r := mustLoad(t)

	rule := r.Rules[0]
	if got, want := rule.Evidence.Asked, []EvidenceItem{{Kind: "file", What: "the Dockerfile", line: 79}, {Kind: "command", What: "the command docker build", line: 80}}; !slices.Equal(got, want) {
		t.Errorf("L1_BLD evidence.asked = %+v, want %+v", got, want)
	}
	if got, want := rule.Penalties, []Penalty{{Remove: 0.2, When: "the Dockerfile is only in the report"}}; !slices.Equal(got, want) {
		t.Errorf("L1_BLD penalties = %+v, want %+v", got, want)
	}
	if got, want := r.Scoring.Penalties, []Penalty{{Name: "secret", Remove: 0.3, When: "The repository exposes a token.", How: "Once per rule."}}; !slices.Equal(got, want) {
		t.Errorf("scoring.penalties = %+v, want %+v", got, want)
	}
	if got := r.LabRules("L2"); len(got) != 2 || got[0].ID != "L2_POD" || !got[1].Optional() {
		t.Errorf("LabRules(L2) = %+v, want L2_POD and the optional L2_LOG", got)
	}
	lab, ok := r.Lab("L1")
	if !ok {
		t.Fatal("Lab(L1) not found")
	}
	deadline, err := lab.DeadlineTime()
	if err != nil {
		t.Fatalf("L1 DeadlineTime(): %v", err)
	}
	if got, want := deadline.UTC().Format("2006-01-02T15:04"), "2030-03-04T07:30"; got != want {
		t.Errorf("L1 deadline in UTC = %s, want %s", got, want)
	}
}

func TestLoadReportsUnknownKeys(t *testing.T) {
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte("    penalties:\n      - remove: 0.2"), []byte("    penalty:\n      - remove: 0.2"), 1)

	_, err = Parse(data)

	got, want := errorString(err), `72: L1_BLD: unknown key "penalty"`
	if got != want {
		t.Errorf("Parse() with a typo = %q, want %q", got, want)
	}
}

func TestLabID(t *testing.T) {
	for in, want := range map[string]string{"L1": "L1", "l2": "L2", "3": "L3", "": ""} {
		if got := LabID(in); got != want {
			t.Errorf("LabID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRender(t *testing.T) {
	r := mustLoad(t)
	var page bytes.Buffer

	if err := r.Render(&page, RenderOptions{Source: "evaluation-rules.yaml", Root: ".."}); err != nil {
		t.Fatalf("Render(): %v", err)
	}

	golden := filepath.Join("testdata", "evaluation-rules.md")
	if *update {
		if err := os.WriteFile(golden, page.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v: run go test ./rubric -update", err)
	}
	if got := page.String(); got != string(want) {
		t.Errorf("Render() differs from %s: run go test ./rubric -update, then review the diff.\ngot:\n%s", golden, got)
	}
}

func TestWrap(t *testing.T) {
	w := wrapper{width: 20}
	for _, c := range []struct {
		name, in, first, indent, want string
	}{
		{"fits", "a short line", "", "", "a short line"},
		{"wraps", "one two three four five six", "", "", "one two three four\nfive six"},
		{"item", "one two three four five six", "- ", "  ", "- one two three four\n  five six"},
		{"no list marker at the start of a line", "aaaa bbbb cccc dddd - eeee", "", "", "aaaa bbbb cccc\ndddd - eeee"},
		{"no ordered marker at the start of a line", "one two three four 2. five", "", "", "one two three\nfour 2. five"},
	} {
		if got := w.wrap(c.in, c.first, c.indent); got != c.want {
			t.Errorf("%s: wrap(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
	for _, line := range strings.Split(w.wrap(strings.Repeat("word ", 30), "", ""), "\n") {
		if len(line) > 20 {
			t.Errorf("wrap() gives a line of %d characters, more than 20: %q", len(line), line)
		}
	}
}

func errorString(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}
