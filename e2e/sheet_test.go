package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// export is the file of mc eval export, as the script of the spreadsheet reads it.
type export struct {
	Spreadsheet string `json:"spreadsheet"`
	Tabs        []struct {
		Name     string            `json:"name"`
		Keys     []string          `json:"keys"`
		Columns  map[string]string `json:"columns"`
		Formulas map[string]string `json:"formulas"`
		Fill     []string          `json:"fill"`
		Rows     []map[string]any  `json:"rows"`
	} `json:"tabs"`
}

// readExport reads the export of mc eval export, and checks that the clipboard holds it.
func (e *env) readExport(t *testing.T) export {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(e.home, ".mc", "2026", "sheet", "export.json"))
	if err != nil {
		t.Fatal(err)
	}
	if copied := e.readClipboard(t); copied != string(data) {
		t.Errorf("the clipboard holds another text than the export:\n%s", copied)
	}
	var x export
	if err := json.Unmarshal(data, &x); err != nil {
		t.Fatal(err)
	}
	return x
}

// readClipboard returns what mc copied to the clipboard last.
func (e *env) readClipboard(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(e.clipboard)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// rowOf returns the row of a tab of the export with the given first key.
func rowOf(t *testing.T, x export, tab, key string) map[string]any {
	t.Helper()
	for _, tb := range x.Tabs {
		if tb.Name != tab {
			continue
		}
		for _, row := range tb.Rows {
			if row[tb.Keys[0]] == key {
				return row
			}
		}
	}
	t.Fatalf("no row %s in the tab %s of the export", key, tab)
	return nil
}

// TestEvalExport exports the evaluations of Lab 2 three times: a first export, a pass that raises
// the completeness of coral and lowers the one of teal, then the same pass again.
func TestEvalExport(t *testing.T) {
	e := newEnv(t)
	flags := e.writeEvaluations(t, []string{"teal", "coral"}, map[string]string{
		"teal":  "0a1b2c3d4e5f60718293a4b5c6d7e8f901234567",
		"coral": "0a1b2c3ffffffffffffffffffffffffffffffff",
	})
	// The facts of teal give the images and the screenshots of L2_POD.
	e.writeFile(t, ".mc/2026/teacher/k8s-teal/docs/lab-2.facts.md", "---\ncommit: 0a1b2c3d4e5f60718293a4b5c6d7e8f901234567\nmc: 949d7aa740ab\n"+
		"rules:\n  L2_POD:\n    images: \"1.1.0 b80767bd1a2c\"\n    screenshots: 2\n---\n\n# Lab 2\n")
	e.writeFile(t, "teams.yaml", "evaluation_sheet: https://docs.google.com/spreadsheets/d/0123456789/edit\n"+
		"teams:\n  - name: teal\n    members: []\n  - name: coral\n    members: []\n")
	file := filepath.Join(e.home, ".mc", "2026", "sheet", "export.json")
	exportCmd := append([]string{"eval", "export"}, flags...)
	next := "Next, in the spreadsheet: the menu \"mc\", then \"Import\", paste it in the text box, then Import.\n" +
		"The first time, add the script of mc sheet script: \"Extensions\", then \"Apps Script\".\n"

	// The first export: every row is new.
	r := e.run(t, "", exportCmd...)
	want := "Exported 4 rules and 2 rows of the tab Evaluation to " + file + ", from 2 evaluations of L2.\n" +
		"No last export: every row is new.\n"
	if r.exitCode != 0 || !strings.HasPrefix(r.stdout, want) || !strings.HasSuffix(r.stdout, next) || r.stderr != "" {
		t.Errorf("mc eval export = %d, stdout\n%s\nstderr %q, want 0, stdout\n%s...\n%s", r.exitCode, r.stdout, r.stderr, want, next)
	}
	x := e.readExport(t)
	if x.Spreadsheet != "0123456789" || len(x.Tabs) != 2 || x.Tabs[0].Name != "Rules" || x.Tabs[1].Name != "Evaluation" {
		t.Fatalf("the export has the spreadsheet %q and %d tabs, want 0123456789, then Rules and Evaluation", x.Spreadsheet, len(x.Tabs))
	}
	evaluation := x.Tabs[1]
	for _, letter := range []string{"B", "C", "H", "J", "K"} {
		if header, ok := evaluation.Columns[letter]; ok {
			t.Errorf("the export writes the column %s, %q, which is not of mc", letter, header)
		}
	}
	if !reflect.DeepEqual(evaluation.Fill, []string{"B", "C"}) || evaluation.Columns["G"] != "TA Completeness" || evaluation.Columns["V"] != "TA Comment" {
		t.Errorf("the tab Evaluation fills %v, with G %q and V %q, want B and C, TA Completeness and TA Comment",
			evaluation.Fill, evaluation.Columns["G"], evaluation.Columns["V"])
	}
	teal := rowOf(t, x, "Evaluation", "teal")
	for header, want := range map[string]any{
		"Lab": "L2", "Rule": "L2_POD", "TA Completeness": 0.8, "TA Commit": "0a1b2c3",
		"TA Docker Images": "1.1.0 b80767bd1a2c", "TA Screenshots": 2.0, "TA Action Items": "Run it yourself",
	} {
		if got := teal[header]; got != want {
			t.Errorf("teal L2_POD, %s = %v, want %v", header, got, want)
		}
	}
	rowOf(t, x, "Rules", "L2")

	// A pass: a higher completeness for coral, and a lower one for teal, which keeps the former one.
	e.writeFile(t, ".mc/2026/evaluations/lab-2/coral.yaml",
		strings.NewReplacer("team: teal", "team: coral", "completeness: 0.8", "completeness: 1").Replace(evaluationOfTeal))
	e.writeFile(t, ".mc/2026/evaluations/lab-2/teal.yaml", strings.Replace(evaluationOfTeal, "completeness: 0.8", "completeness: 0.5", 1))
	r = e.run(t, "", append(exportCmd, "-l", "L2")...)
	want = "Exported 4 rules and 2 rows of the tab Evaluation to " + file + ", from 2 evaluations of L2.\n\n" +
		"TAB         ROW              COLUMN           CHANGE\n" +
		"Evaluation  coral L2 L2_POD  TA Completeness  0.8 -> 1\n" +
		"Evaluation  teal L2 L2_POD   TA Completeness  kept 0.8, not 0.5: lower than the former pass\n" +
		"Evaluation  teal L2 L2_POD   TA To Check      nothing -> \"The pass of 0a1b2c3 gives 0.5, lower tha...\"\n\n" +
		"Copied the export to the clipboard, "
	if r.exitCode != 0 || !strings.HasPrefix(r.stdout, want) || r.stderr != "" {
		t.Errorf("mc eval export of a pass = %d, stdout\n%s\nstderr %q, want 0, stdout\n%s...", r.exitCode, r.stdout, r.stderr, want)
	}
	x = e.readExport(t)
	if got := rowOf(t, x, "Evaluation", "teal")["TA Completeness"]; got != 0.8 {
		t.Errorf("teal L2_POD, TA Completeness = %v after a lower one, want 0.8", got)
	}

	// The same pass again changes no cell, but says again that a completeness is kept.
	r = e.run(t, "", exportCmd...)
	want = "\n\nTAB         ROW             COLUMN           CHANGE\n" +
		"Evaluation  teal L2 L2_POD  TA Completeness  kept 0.8, not 0.5: lower than the former pass\n\nCopied the export"
	if r.exitCode != 0 || !strings.Contains(r.stdout, want) {
		t.Errorf("mc eval export of the same pass = %d, stdout\n%s\nstderr %q, want the completeness kept only", r.exitCode, r.stdout, r.stderr)
	}

	// Without evaluation of coral, its row stays as it was in the last export.
	if err := os.Remove(filepath.Join(e.home, ".mc", "2026", "evaluations", "lab-2", "coral.yaml")); err != nil {
		t.Fatal(err)
	}
	r = e.run(t, "", exportCmd...)
	if got := rowOf(t, e.readExport(t), "Evaluation", "coral")["TA Completeness"]; r.exitCode != 0 || got != 1.0 {
		t.Errorf("mc eval export without coral = %d, coral L2_POD, TA Completeness = %v, want 1, stderr %q", r.exitCode, got, r.stderr)
	}
}

func TestEvalExportRefusesAnExportOfAnotherLayout(t *testing.T) {
	e := newEnv(t)
	flags := e.writeEvaluations(t, []string{"teal"}, map[string]string{"teal": "0a1b2c3d4e5f60718293a4b5c6d7e8f901234567"})
	// The tab Evaluation as before the column of the teacher: the completeness of mc in G, named
	// Completeness.
	e.writeFile(t, ".mc/2026/sheet/export.json", `{
  "tabs": [
    {"name": "Evaluation", "keys": ["Team", "Lab", "Rule"], "columns": {"A": "Team", "D": "Lab", "E": "Rule", "G": "Completeness"},
     "rows": [{"Team": "teal", "Lab": "L2", "Rule": "L2_POD", "Completeness": 0.5}]}
  ]
}
`)
	r := e.run(t, "", append([]string{"eval", "export"}, flags...)...)
	want := "the tab Evaluation of the last export has another layout than the one of mc: migrate it first"
	if r.exitCode != 1 || !strings.Contains(r.stderr, want) {
		t.Errorf("mc eval export after an export of another layout = %d, stderr %q, want 1 and %q", r.exitCode, r.stderr, want)
	}
	if _, err := os.Stat(e.clipboard); err == nil {
		t.Error("mc eval export copied an export, though it stopped")
	}
}

func TestEvalExportOfATeamWithoutEvaluation(t *testing.T) {
	e := newEnv(t)
	flags := e.writeEvaluations(t, []string{"teal"}, map[string]string{"teal": "0a1b2c3d4e5f60718293a4b5c6d7e8f901234567"})
	r := e.run(t, "", append([]string{"eval", "export", "-t", "olive"}, flags...)...)
	if r.exitCode != 1 || !strings.Contains(r.stderr, "no evaluation of olive") {
		t.Errorf("mc eval export -t olive = %d, stderr %q, want 1 and no evaluation of olive", r.exitCode, r.stderr)
	}
}

// TestSheetScript prints the script of the spreadsheet, then copies it to the clipboard.
func TestSheetScript(t *testing.T) {
	e := newEnv(t)
	r := e.run(t, "", "sheet", "script")
	if r.exitCode != 0 || !strings.Contains(r.stdout, "@OnlyCurrentDoc") || !strings.Contains(r.stdout, "function mcImport(") {
		t.Errorf("mc sheet script = %d, stdout\n%s\nstderr %q, want the script", r.exitCode, r.stdout, r.stderr)
	}
	script := r.stdout

	r = e.run(t, "", "sheet", "script", "--copy")
	if r.exitCode != 0 || !strings.HasPrefix(r.stdout, "Copied the script to the clipboard, ") {
		t.Errorf("mc sheet script --copy = %d, stdout %q, stderr %q", r.exitCode, r.stdout, r.stderr)
	}
	if copied := e.readClipboard(t); copied != script {
		t.Errorf("mc sheet script --copy copied another text than the script:\n%s", copied)
	}
}
