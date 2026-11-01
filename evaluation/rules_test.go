package evaluation

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mincong-classroom/mc/rubric"
	"github.com/mincong-classroom/mc/sheet"
)

// rulesTab returns a file of the tab Rules, with the columns of mc and the rows given, in JSON.
func rulesTab(t *testing.T, rows string) *sheet.File {
	t.Helper()
	columns, err := json.Marshal(rulesLayout.columns)
	if err != nil {
		t.Fatal(err)
	}
	var f sheet.File
	data := `{"tab": "Rules", "keys": ["Lab Session", "Rule"], "columns": ` + string(columns) + `, "rows": [` + rows + `]}`
	if err := json.Unmarshal([]byte(data), &f); err != nil {
		t.Fatal(err)
	}
	return &f
}

// ruleOf returns a rule of the YAML.
func ruleOf(id, name, exercise string, coefficient float64) rubric.Rule {
	lab, _, _ := strings.Cut(id, "_")
	return rubric.Rule{
		ID: id, Lab: lab, Name: name, Exercise: exercise, Coefficient: coefficient,
		Description: "The team is expected to\nbuild the image  of the application.",
	}
}

// ruleRow returns a row of the tab Rules, as the script reads it: every value is a text.
func ruleRow(number, lab, end, name, coefficient, points, exercise string) string {
	return `{"_row": ` + number + `, "Lab Session": "` + lab + `", "Rule": "` + end + `", "Rule ID": "` + lab + "_" + end +
		`", "Rule Name": "` + name + `", "Coefficient": "` + coefficient + `", "Score": "` + points +
		`", "Exercise": "` + exercise + `", "Exercise ID": "` + lab + "_EX" + exercise +
		`", "Rule Description": "The team is expected to build the image of the application."}`
}

// ruleChanges returns the changes of the result, one line each.
func ruleChanges(result *RulesSyncResult) []string {
	return changes(&SyncResult{Changes: result.Changes})
}

func TestSyncRulesWritesWhatDiffers(t *testing.T) {
	read := rulesTab(t, strings.Join([]string{
		ruleRow("2", "L1", "BLD", "Build Test", "0.6", "12", "1"),
		ruleRow("3", "L1", "TAG", "Publish Test", "0.5", "10", "2"),
		ruleRow("4", "L2", "POD", "Pod Tests", "1", "20", "1"),
		ruleRow("5", "L3", "OLD", "Former Test", "0.2", "4", "3"),
	}, ","))
	// mc wrote the coefficient of L1_TAG last, but not the name of L2_POD.
	exported := rulesTab(t, `{"_row": 3, "Lab Session": "L1", "Rule": "TAG", "Coefficient": 0.5}`)
	rules := []rubric.Rule{
		ruleOf("L1_BLD", "Build Test", "1", 0.6),
		ruleOf("L1_TAG", "Publish Test", "2", 0.4),
		ruleOf("L2_POD", "Pod Test", "1", 1),
		ruleOf("L2_LOG", "Logs Test", "2.1", 0.1),
	}

	result, err := SyncRules(RulesSyncOptions{Rules: rules, Read: read, Exported: exported})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"Rules!E3 Coefficient: 0.5 -> 0.4",
		`Rules!D4 Rule Name: Pod Tests -> Pod Test, check: the tab has "Pod Tests", which mc didn't write`,
	}
	if got := ruleChanges(result); !reflect.DeepEqual(got, want) {
		t.Errorf("Changes =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if want := []RuleRow{{ID: "L2_LOG", Row: 6}}; !reflect.DeepEqual(result.Added, want) {
		t.Errorf("Added = %+v, want %+v", result.Added, want)
	}
	if want := []RuleRow{{ID: "L3_OLD", Row: 5}}; !reflect.DeepEqual(result.Extra, want) {
		t.Errorf("Extra = %+v, want %+v", result.Extra, want)
	}

	rows := result.Pending.Rows
	if len(rows) != 3 || rows[0].Number != 3 || rows[1].Number != 4 || rows[2].Number != 6 {
		t.Fatalf("the rows to write are %+v, want the rows 3, 4 and 6", rows)
	}
	if want := map[string]any{"Lab Session": "L1", "Rule": "TAG", "Coefficient": json.Number("0.4")}; !reflect.DeepEqual(rows[0].Cells, want) || rows[0].Check != nil {
		t.Errorf("the row 3 = %v, check %v, want %v and no check", rows[0].Cells, rows[0].Check, want)
	}
	if _, ok := rows[1].Check["Rule Name"]; !ok {
		t.Errorf("the row 4 has no check on its name: %v", rows[1].Check)
	}
	want6 := map[string]any{
		"Lab Session":      "L2",
		"Rule":             "LOG",
		"Rule ID":          `=CONCAT(A6, CONCAT("_", B6))`,
		"Rule Name":        "Logs Test",
		"Coefficient":      json.Number("0.1"),
		"Score":            "=20*E6",
		"Exercise":         json.Number("2.1"),
		"Exercise ID":      `=CONCAT(CONCAT(A6, "_"), CONCAT("EX", G6))`,
		"Rule Description": "The team is expected to build the image of the application.",
	}
	if !rows[2].New || !reflect.DeepEqual(rows[2].Cells, want6) {
		t.Errorf("the new row 6 = %v, new %v, want\n%v", rows[2].Cells, rows[2].New, want6)
	}
}

func TestSyncRulesLeavesATabUpToDate(t *testing.T) {
	read := rulesTab(t, ruleRow("2", "L1", "BLD", "Build Test", "0.6", "12", "1")+","+
		ruleRow("3", "L2", "LOG", "Logs Test", "0.1", "2", "2.1"))
	rules := []rubric.Rule{ruleOf("L1_BLD", "Build Test", "1", 0.6), ruleOf("L2_LOG", "Logs Test", "2.1", 0.1)}

	result, err := SyncRules(RulesSyncOptions{Rules: rules, Read: read})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Pending.Rows) != 0 || len(result.Changes) != 0 || len(result.Added) != 0 || len(result.Extra) != 0 {
		t.Errorf("SyncRules of a tab up to date = %+v, want nothing to write", result)
	}
}

func TestSyncRulesWritesAFormulaTypedOver(t *testing.T) {
	// A Rule ID typed by hand, and a score that is not 20 times the coefficient.
	read := rulesTab(t, strings.Replace(ruleRow("2", "L1", "BLD", "Build Test", "0.6", "10", "1"), `"L1_BLD"`, `"L1-BLD"`, 1))
	result, err := SyncRules(RulesSyncOptions{Rules: []rubric.Rule{ruleOf("L1_BLD", "Build Test", "1", 0.6)}, Read: read})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`Rules!C2 Rule ID: L1-BLD -> =CONCAT(A2, CONCAT("_", B2)), check: the tab has "L1-BLD", whereas the formula gives "L1_BLD"`,
		`Rules!F2 Score: 10 -> =20*E2, check: the tab has "10", whereas the formula gives "12"`,
	}
	if got := ruleChanges(result); !reflect.DeepEqual(got, want) {
		t.Errorf("Changes =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestSyncRulesRefusesAnotherLayout(t *testing.T) {
	rules := []rubric.Rule{ruleOf("L1_BLD", "Build Test", "1", 0.6)}
	missing := rulesTab(t, "")
	delete(missing.Columns, "I")
	renamed := rulesTab(t, "")
	renamed.Columns["E"] = "Weight"
	for _, tc := range []struct {
		name string
		read *sheet.File
		want string
	}{
		{"a column missing", missing, `the tab read has no column I, "Rule Description": read the tab with the script of mc rule sync`},
		{"a column renamed", renamed, `the column E of the tab read is "Weight", whereas mc writes "Coefficient" there`},
		{"the tab Evaluation", tab(t, ""), `the tab read is the tab "Evaluation"`},
	} {
		_, err := SyncRules(RulesSyncOptions{Rules: rules, Read: tc.read})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("SyncRules of %s = %v, want %q", tc.name, err, tc.want)
		}
	}
}

func TestSyncRulesRefusesAFormulaInAText(t *testing.T) {
	rule := ruleOf("L1_BLD", "=Build Test", "1", 0.6)
	_, err := SyncRules(RulesSyncOptions{Rules: []rubric.Rule{rule}, Read: rulesTab(t, "")})
	if want := `the rule L1_BLD, Rule Name: "=Build Test" starts with =`; err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("SyncRules of a name that starts with = = %v, want %q", err, want)
	}
}
