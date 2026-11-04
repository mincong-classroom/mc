package sheet

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// export returns an export of a made-up tab, with the rows given.
func export(rows ...map[string]any) *Export {
	return &Export{
		Spreadsheet: "1AbC",
		Tabs: []*Tab{{
			Name:     "Evaluation",
			Keys:     []string{"Team", "Rule"},
			Columns:  map[string]string{"A": "Team", "B": "Rule", "D": "TA Completeness", "F": "Score", "AA": "TA Comment"},
			Formulas: map[string]string{"Score": "=D{row}*2"},
			Fill:     []string{"C"},
			Rows:     rows,
		}},
	}
}

func TestExportSavesAndLoads(t *testing.T) {
	e := export(map[string]any{
		"Team": "blue", "Rule": "L1_BLD", "TA Completeness": json.Number("0.8"), "TA Comment": "Done well: <b> & co.",
	})
	path := filepath.Join(t.TempDir(), "export.json")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "spreadsheet": "1AbC",
  "tabs": [
    {
      "name": "Evaluation",
      "keys": [
        "Team",
        "Rule"
      ],
      "columns": {
        "A": "Team",
        "B": "Rule",
        "D": "TA Completeness",
        "F": "Score",
        "AA": "TA Comment"
      },
      "formulas": {
        "Score": "=D{row}*2"
      },
      "fill": [
        "C"
      ],
      "rows": [
        {
          "Team": "blue",
          "Rule": "L1_BLD",
          "TA Completeness": 0.8,
          "TA Comment": "Done well: <b> & co."
        }
      ]
    }
  ]
}
`
	if string(data) != want {
		t.Errorf("Save wrote\n%s\nwant\n%s", data, want)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("the mode of the export = %v (%v), want 0600: it holds the evaluations", info.Mode().Perm(), err)
	}
	loaded, err := LoadExport(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, e) {
		t.Errorf("LoadExport = %+v, want %+v", loaded, e)
	}
}

func TestExportProblems(t *testing.T) {
	e := export(
		map[string]any{"Team": "blue", "Rule": "L1_BLD", "Score": "=1", "TA Comment": ""},
		map[string]any{"Team": "blue", "Rule": "L1_BLD", "Students": "x"},
		map[string]any{"Rule": "L1_RUN", "TA Completeness": []any{1}},
	)
	e.Tabs[0].Fill = []string{"D"}
	e.Tabs[0].Formulas["Team"] = "Team"
	want := []string{
		`tabs[0] (Evaluation): formulas: "Team" is a key`,
		`tabs[0] (Evaluation): formulas: "Team": "Team" doesn't start with =`,
		`tabs[0] (Evaluation): fill: the column D is the one of "TA Completeness", which the import writes`,
		`tabs[0] (Evaluation): rows[0]: "Score" has the formula of its column`,
		`tabs[0] (Evaluation): rows[0]: "TA Comment": an empty text "": leave the header out for an empty cell`,
		`tabs[0] (Evaluation): rows[0]: "Team" has the formula of its column`,
		`tabs[0] (Evaluation): rows[1]: "Students" is not a header of the columns`,
		`tabs[0] (Evaluation): rows[1]: "Team" has the formula of its column`,
		`tabs[0] (Evaluation): rows[1]: the keys of rows[0] too, blue L1_BLD`,
		`tabs[0] (Evaluation): rows[2]: "TA Completeness": [1] is not a text, a number or a boolean`,
		`tabs[0] (Evaluation): rows[2]: no key "Team", a text`,
	}
	if got := e.Problems(); !reflect.DeepEqual(got, want) {
		t.Errorf("Problems =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestTheConfirmedColumn(t *testing.T) {
	e := export(map[string]any{"Team": "blue", "Rule": "L1_BLD", "TA Completeness": json.Number("0.8")})
	e.Tabs[0].Confirmed = map[string]string{"E": "Completeness"}
	if problems := e.Problems(); len(problems) != 0 {
		t.Errorf("Problems = %q, want none", problems)
	}
	path := filepath.Join(t.TempDir(), "export.json")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadExport(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Tabs[0].Confirmed; !reflect.DeepEqual(got, map[string]string{"E": "Completeness"}) {
		t.Errorf("the confirmed column after a load = %v, want E", got)
	}

	e.Tabs[0].Confirmed = map[string]string{"C": "", "D": "Completeness"}
	want := []string{
		"tabs[0] (Evaluation): confirmed: 2 columns, at most one",
		"tabs[0] (Evaluation): confirmed: the column C has no header",
		"tabs[0] (Evaluation): confirmed: the column C is a column to fill",
		`tabs[0] (Evaluation): confirmed: the column D is the one of "TA Completeness", which the import writes`,
	}
	if got := e.Problems(); !reflect.DeepEqual(got, want) {
		t.Errorf("Problems =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestLoadExportRefusesAnUnknownAttribute(t *testing.T) {
	path := filepath.Join(t.TempDir(), "export.json")
	if err := os.WriteFile(path, []byte(`{"spreadsheet": "", "tabs": [{"name": "Rules", "kyes": []}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadExport(path); err == nil || !strings.Contains(err.Error(), `unknown field "kyes"`) {
		t.Errorf("LoadExport = %v, want an error on the unknown field", err)
	}
}

func TestSpreadsheetID(t *testing.T) {
	if got := SpreadsheetID("https://docs.google.com/spreadsheets/d/1AbC-d_E/edit#gid=0"); got != "1AbC-d_E" {
		t.Errorf("SpreadsheetID = %q, want 1AbC-d_E", got)
	}
	if got := SpreadsheetID("https://example.com"); got != "" {
		t.Errorf("SpreadsheetID of another URL = %q, want nothing", got)
	}
}

func TestFormula(t *testing.T) {
	if got := Formula("=G{row}*F{row}", 12); got != "=G12*F12" {
		t.Errorf("Formula = %q, want =G12*F12", got)
	}
}

// TestImportScript runs the tests of the script of ImportScript with node, when node is there:
// the script runs in Apps Script, whose services the tests leave out.
func TestImportScript(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("no node to run the tests of import.gs")
	}
	out, err := exec.Command(node, "--test", "import_test.js").CombinedOutput()
	if err != nil {
		t.Fatalf("node --test import_test.js: %v\n%s", err, out)
	}
	if !strings.Contains(ImportScript(), "function mcImport(text)") {
		t.Error("ImportScript doesn't hold the function mcImport")
	}
}
