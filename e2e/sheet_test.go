package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSheetPasteThenVerify writes a tab by blocks, records the paste, then compares the tab read
// again with the last export, in ~/.mc/2026/sheet/.
func TestSheetPasteThenVerify(t *testing.T) {
	e := newEnv(t)
	dir := filepath.Join(e.home, ".mc", "2026", "sheet")
	pending := `{
  "tab": "Evaluation",
  "keys": ["Team", "Lab", "Rule"],
  "columns": {"A": "Team", "D": "Lab", "E": "Rule", "G": "Completeness", "L": "TA Commit", "O": "TA Analysis"},
  "rows": [
    {"_row": 2, "Team": "red", "Lab": "L2", "Rule": "L2_CTL", "TA Commit": "2e88112", "TA Analysis": "short"},
    {"_row": 3, "Team": "red", "Lab": "L2", "Rule": "L2_RUN", "TA Commit": "2e88112", "TA Analysis": "none",
     "Completeness": 0.9, "_check": {"Completeness": "0.7 at the last export"}}
  ]
}
`
	write(t, filepath.Join(dir, "evaluation.json"), pending)

	r := e.run(t, "", "sheet", "blocks", "evaluation")
	for _, want := range []string{
		"3 blocks of the tab Evaluation wait in " + filepath.Join(dir, "evaluation.json"),
		"1      Evaluation!L2:L3  2 × 1  \n",
		"2      Evaluation!O2:O3  2 × 1  \n",
		"3      Evaluation!G3     1 × 1  G3 Completeness: 0.7 at the last export\n",
	} {
		if r.exitCode != 0 || !strings.Contains(r.stdout, want) {
			t.Errorf("mc sheet blocks: exit code %d, stdout misses %q:\n%s\nstderr:\n%s", r.exitCode, want, r.stdout, r.stderr)
		}
	}

	r = e.run(t, "", "sheet", "copy", "evaluation", "1", "--print")
	if r.exitCode != 0 || r.stdout != "'2e88112\n'2e88112\n" || !strings.Contains(r.stderr, "Block 1 of 3, Evaluation!L2:L3 (2 × 1)") {
		t.Errorf("mc sheet copy --print: exit code %d, stdout %q, stderr:\n%s", r.exitCode, r.stdout, r.stderr)
	}

	// The teacher pastes the blocks 1 and 2, and leaves the completeness to check alone.
	r = e.run(t, "", "sheet", "pasted", "evaluation", "1", "2")
	if r.exitCode != 0 || !strings.Contains(r.stdout, "Recorded 4 cells of 2 blocks as the last export") ||
		!strings.Contains(r.stdout, "1 block waits still in") {
		t.Errorf("mc sheet pasted: exit code %d, stdout:\n%s\nstderr:\n%s", r.exitCode, r.stdout, r.stderr)
	}

	// The tab, as the script of mc sheet snippet copies it: texts, and a cell that the teacher
	// changed.
	read := `{
  "tab": "Evaluation",
  "keys": ["Team", "Lab", "Rule"],
  "columns": {"A": "Team", "D": "Lab", "E": "Rule", "G": "Completeness", "L": "TA Commit", "O": "TA Analysis"},
  "rows": [
    {"_row": 2, "Team": "red", "Lab": "L2", "Rule": "L2_CTL", "TA Commit": "2e88112", "TA Analysis": "short"},
    {"_row": 3, "Team": "red", "Lab": "L2", "Rule": "L2_RUN", "Completeness": "0.7", "TA Commit": "2e88112", "TA Analysis": "short"}
  ]
}
`
	write(t, filepath.Join(dir, "evaluation.read.json"), read)
	r = e.run(t, "", "sheet", "verify", "evaluation")
	if want := `Evaluation!O3 red L2 L2_RUN, TA Analysis: "short" in the tab, "none" at the export` + "\n"; r.exitCode != 1 || r.stdout != want {
		t.Errorf("mc sheet verify: exit code %d, stdout:\n%s\nwant:\n%s", r.exitCode, r.stdout, want)
	}

	write(t, filepath.Join(dir, "evaluation.read.json"), strings.Replace(read, `"TA Analysis": "short"}`+"\n  ]", `"TA Analysis": "none"}`+"\n  ]", 1))
	r = e.run(t, "", "sheet", "verify", "evaluation")
	if r.exitCode != 0 || r.stdout != "The tab Evaluation holds the last export: 2 rows.\n" {
		t.Errorf("mc sheet verify of the tab as written: exit code %d, stdout:\n%s\nstderr:\n%s", r.exitCode, r.stdout, r.stderr)
	}
}

func TestSheetSnippetRefusesAnotherSpreadsheet(t *testing.T) {
	e := newEnv(t)
	write(t, filepath.Join(e.home, ".mc", "2026", "sheet", "rules.exported.json"), `{
  "tab": "Rules",
  "keys": ["Rule ID"],
  "columns": {"C": "Rule ID", "E": "Coefficient"},
  "rows": []
}
`)
	teamFile := filepath.Join(t.TempDir(), "teams.yaml")
	write(t, teamFile, `evaluation_sheet: https://docs.google.com/spreadsheets/d/0123456789/edit
teams: []
`)
	r := e.run(t, "", "sheet", "snippet", "rules", "--team-file", teamFile)
	for _, want := range []string{
		`// mc sheet snippet: reads the columns C, E of the tab "Rules"`,
		`  sheet: "0123456789",`,
		`  path: "` + filepath.Join(e.home, ".mc", "2026", "sheet", "rules.read.json") + `",`,
	} {
		if r.exitCode != 0 || !strings.Contains(r.stdout, want) {
			t.Errorf("mc sheet snippet: exit code %d, stdout misses %q:\n%s\nstderr:\n%s", r.exitCode, want, r.stdout, r.stderr)
		}
	}
	r = e.run(t, "", "sheet", "snippet", "scores")
	if r.exitCode != 1 || !strings.Contains(r.stderr, "no columns to read: neither") {
		t.Errorf("mc sheet snippet of a tab without files: exit code %d, stderr:\n%s", r.exitCode, r.stderr)
	}
}

// write writes a file, and its directory.
func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
