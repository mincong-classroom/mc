package evaluation

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mincong-classroom/mc/rubric"
	"github.com/mincong-classroom/mc/sheet"
)

// evaluationOf returns the evaluation of a made-up team, with its rules.
func evaluationOf(team string, rules ...Rule) *File {
	return &File{Team: team, Commit: "0a1b2c3d4e5f", TimeSpent: "Session only", ActionItems: "Run it yourself", Rules: rules}
}

// rule returns the evaluation of a rule, with its completeness.
func rule(id string, completeness float64) Rule {
	return Rule{
		ID: id, Completeness: completeness, DoneWell: "The Pod runs", ToImprove: "Show the logs.",
		Analysis: "short", WritingStyle: "own words",
	}
}

// ruleOf returns a rule of the YAML.
func ruleOf(id, name, exercise string, coefficient float64) rubric.Rule {
	lab, _, _ := strings.Cut(id, "_")
	return rubric.Rule{
		ID: id, Lab: lab, Name: name, Exercise: exercise, Coefficient: coefficient,
		Description: "The team is expected to\nbuild the image  of the application.",
	}
}

// lastExport returns an export of the tab Evaluation, with the layout of mc and the rows given in
// JSON, and the tab Rules given, if any.
func lastExport(t *testing.T, rows string, rules ...map[string]any) *sheet.Export {
	t.Helper()
	tab := EvaluationLayout()
	if rows != "" {
		decoder := json.NewDecoder(strings.NewReader("[" + rows + "]"))
		decoder.UseNumber()
		if err := decoder.Decode(&tab.Rows); err != nil {
			t.Fatal(err)
		}
	}
	r := RulesLayout()
	r.Rows = rules
	return &sheet.Export{Tabs: []*sheet.Tab{r, tab}}
}

// changes returns the changes of the result, one line each.
func changes(result *ExportResult) []string {
	lines := make([]string, len(result.Changes))
	for i, c := range result.Changes {
		lines[i] = fmt.Sprintf("%s %s %s: %v -> %v", c.Tab, c.Row, c.Header, c.From, c.To)
	}
	return lines
}

// rowsOf returns the rows of a tab of the export, in JSON, one line each.
func rowsOf(t *testing.T, result *ExportResult, name string) []string {
	t.Helper()
	tab := result.Export.Tab(name)
	lines := make([]string, len(tab.Rows))
	for i, row := range tab.Rows {
		b, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		lines[i] = string(b)
	}
	return lines
}

func TestExportAddsTheRowsOfALab(t *testing.T) {
	last := lastExport(t, `{"Team": "teal", "Lab": "L1", "Rule": "L1_BLD", "TA Completeness": 1}`)
	withPenalty := rule("L2_LOG", 1)
	withPenalty.Penalty = &Penalty{Kind: PenaltyRule, Remove: 0.1, Why: "The logs come from another Pod"}

	result, err := Export(ExportOptions{
		Spreadsheet: "1AbC",
		Labs: []LabEvaluations{{
			Lab:   "L2",
			Files: []*File{evaluationOf("teal", rule("L2_POD", 0.8), withPenalty), evaluationOf("coral", rule("L2_POD", 0.5))},
			Facts: map[string]map[string]RuleFacts{"coral": {"L2_POD": {Images: "none", Screenshots: 2}}},
		}},
		Last: last,
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		`{"Lab":"L1","Rule":"L1_BLD","TA Completeness":1,"Team":"teal"}`,
		`{"Lab":"L2","Rule":"L2_POD","TA Action Items":"Run it yourself","TA Analysis":"short","TA Comment":"Done well: The Pod runs. To improve: Show the logs.","TA Commit":"0a1b2c3","TA Completeness":0.5,"TA Docker Images":"none","TA Screenshots":2,"TA Time Spent":"Session only","TA Writing Style":"own words","Team":"coral"}`,
		`{"Lab":"L2","Rule":"L2_POD","TA Action Items":"Run it yourself","TA Analysis":"short","TA Comment":"Done well: The Pod runs. To improve: Show the logs.","TA Commit":"0a1b2c3","TA Completeness":0.8,"TA Time Spent":"Session only","TA Writing Style":"own words","Team":"teal"}`,
		`{"Lab":"L2","Rule":"L2_LOG","TA Action Items":"Run it yourself","TA Analysis":"short","TA Comment":"Done well: The Pod runs. To improve: Show the logs. Penalty: The logs come from another Pod.","TA Commit":"0a1b2c3","TA Completeness":1,"TA Penalty":"=-0.1*IF(ISBLANK(H{row}),G{row},H{row})*F{row}","TA Time Spent":"Session only","TA Writing Style":"own words","Team":"teal"}`,
	}
	if got := rowsOf(t, result, SheetTab); !reflect.DeepEqual(got, want) {
		t.Errorf("rows =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if got, want := result.Added[SheetTab], []string{"coral L2 L2_POD", "teal L2 L2_POD", "teal L2 L2_LOG"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Added = %q, want %q", got, want)
	}
	if len(result.Changes) != 0 {
		t.Errorf("Changes = %q, want none: the cells of a new row are not changes", changes(result))
	}
	if result.Export.Spreadsheet != "1AbC" {
		t.Errorf("Spreadsheet = %q, want 1AbC", result.Export.Spreadsheet)
	}
}

func TestExportAPassAfterAFormerOne(t *testing.T) {
	last := lastExport(t, `{"Team": "teal", "Lab": "L2", "Rule": "L2_POD", "TA Completeness": 0.6, "TA Comment": "Done well: old.", "TA Analysis": "short", "TA Commit": "0a1b2c3"},
		{"Team": "teal", "Lab": "L2", "Rule": "L2_LOG", "TA Completeness": 1, "TA Analysis": "short"},
		{"Team": "coral", "Lab": "L2", "Rule": "L2_POD", "TA Completeness": 0.4, "TA Penalty": "=-0.2*IF(ISBLANK(H{row}),G{row},H{row})*F{row}"},
		{"Team": "olive", "Lab": "L3", "Rule": "L3_SVC", "TA Completeness": 0.7}`)
	detailed := rule("L2_POD", 0.9)
	detailed.Analysis = "detailed"

	result, err := Export(ExportOptions{
		Labs: []LabEvaluations{{
			Lab:   "L2",
			Files: []*File{evaluationOf("teal", detailed, rule("L2_LOG", 0.8)), evaluationOf("coral", rule("L2_POD", 0.7))},
		}},
		Last: last,
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		// coral: a higher completeness, and a penalty that the pass doesn't give, which goes.
		"Evaluation coral L2 L2_POD TA Completeness: 0.4 -> 0.7",
		"Evaluation coral L2 L2_POD TA Comment: <nil> -> Done well: The Pod runs. To improve: Show the logs.",
		"Evaluation coral L2 L2_POD TA Commit: <nil> -> 0a1b2c3",
		"Evaluation coral L2 L2_POD TA Analysis: <nil> -> short",
		"Evaluation coral L2 L2_POD TA Writing Style: <nil> -> own words",
		"Evaluation coral L2 L2_POD TA Time Spent: <nil> -> Session only",
		"Evaluation coral L2 L2_POD TA Action Items: <nil> -> Run it yourself",
		"Evaluation coral L2 L2_POD TA Penalty: =-0.2*IF(ISBLANK(H{row}),G{row},H{row})*F{row} -> <nil>",
		// teal L2_POD: a higher completeness, with its comment.
		"Evaluation teal L2 L2_POD TA Completeness: 0.6 -> 0.9",
		"Evaluation teal L2 L2_POD TA Comment: Done well: old. -> Done well: The Pod runs. To improve: Show the logs.",
		"Evaluation teal L2 L2_POD TA Analysis: short -> detailed",
		"Evaluation teal L2 L2_POD TA Writing Style: <nil> -> own words",
		"Evaluation teal L2 L2_POD TA Time Spent: <nil> -> Session only",
		"Evaluation teal L2 L2_POD TA Action Items: <nil> -> Run it yourself",
		// teal L2_LOG: a lower completeness replaces the former one too.
		"Evaluation teal L2 L2_LOG TA Completeness: 1 -> 0.8",
		"Evaluation teal L2 L2_LOG TA Comment: <nil> -> Done well: The Pod runs. To improve: Show the logs.",
		"Evaluation teal L2 L2_LOG TA Commit: <nil> -> 0a1b2c3",
		"Evaluation teal L2 L2_LOG TA Writing Style: <nil> -> own words",
		"Evaluation teal L2 L2_LOG TA Time Spent: <nil> -> Session only",
		"Evaluation teal L2 L2_LOG TA Action Items: <nil> -> Run it yourself",
	}
	if got := changes(result); !reflect.DeepEqual(got, want) {
		t.Errorf("Changes =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	var lower []string
	for _, c := range result.Changes {
		if c.Lower() {
			lower = append(lower, c.Row)
		}
	}
	if want := []string{"teal L2 L2_LOG"}; !reflect.DeepEqual(lower, want) {
		t.Errorf("the lower completenesses = %q, want %q", lower, want)
	}
	rows := rowsOf(t, result, SheetTab)
	if len(rows) != 4 || strings.Contains(rows[2], `"TA Penalty"`) || !strings.Contains(rows[1], `"TA Completeness":0.8`) ||
		rows[3] != `{"Lab":"L3","Rule":"L3_SVC","TA Completeness":0.7,"Team":"olive"}` {
		t.Errorf("rows =\n%s\nwant the 4 rows of the last export, the lower completeness, no penalty, and the row of L3 as it was", strings.Join(rows, "\n"))
	}
	if len(result.Added[SheetTab]) != 0 {
		t.Errorf("Added = %q, want none", result.Added[SheetTab])
	}

	// The same pass again changes nothing.
	again, err := Export(ExportOptions{
		Labs: []LabEvaluations{{
			Lab:   "L2",
			Files: []*File{evaluationOf("teal", detailed, rule("L2_LOG", 0.8)), evaluationOf("coral", rule("L2_POD", 0.7))},
		}},
		Last: result.Export,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := changes(again); len(got) != 0 {
		t.Errorf("Changes of the same pass again = %q, want none", got)
	}
}

func TestExportClearsWhatTheEvaluationNoLongerSays(t *testing.T) {
	last := lastExport(t, `{"Team": "teal", "Lab": "L1", "Rule": "L1_TAG", "TA Completeness": 1, "TA To Check": "Check the logs.",
		"TA Docker Images": "1.0.0 b80767bd1a2c", "TA Writing Style": "own words", "TA Comment": "Done well: The Pod runs. To improve: Show the logs."}`)
	bare := rule("L1_TAG", 1)
	bare.WritingStyle = ""
	result, err := Export(ExportOptions{
		Labs: []LabEvaluations{{
			Lab:   "L1",
			Files: []*File{evaluationOf("teal", bare)},
			Facts: map[string]map[string]RuleFacts{"teal": {"L1_TAG": {}}},
		}},
		Last: last,
	})
	if err != nil {
		t.Fatal(err)
	}
	var cleared []string
	for _, c := range result.Changes {
		if c.To == nil {
			cleared = append(cleared, c.Header)
		}
	}
	if want := []string{"TA To Check", "TA Writing Style", "TA Docker Images"}; !reflect.DeepEqual(cleared, want) {
		t.Errorf("the cells cleared = %q, want %q", cleared, want)
	}
	for _, header := range []string{"TA To Check", "TA Writing Style", "TA Docker Images"} {
		if v, ok := result.Export.Tab(SheetTab).Rows[0][header]; ok {
			t.Errorf("the row still has %s = %v, want the cell cleared", header, v)
		}
	}
}

func TestChangeLower(t *testing.T) {
	for _, tc := range []struct {
		c    Change
		want bool
	}{
		{Change{Tab: SheetTab, Header: headerCompleteness, From: json.Number("1"), To: json.Number("0.7")}, true},
		{Change{Tab: SheetTab, Header: headerCompleteness, From: json.Number("0.7"), To: json.Number("1")}, false},
		{Change{Tab: SheetTab, Header: headerCompleteness, From: nil, To: json.Number("0.5")}, false},
		{Change{Tab: SheetTab, Header: headerScreenshots, From: json.Number("2"), To: json.Number("0")}, false},
		{Change{Tab: RulesTab, Header: headerCompleteness, From: json.Number("1"), To: json.Number("0.7")}, false},
	} {
		if got := tc.c.Lower(); got != tc.want {
			t.Errorf("%+v.Lower() = %v, want %v", tc.c, got, tc.want)
		}
	}
}

func TestExportWritesTheFactsOfMC(t *testing.T) {
	last := lastExport(t, `{"Team": "teal", "Lab": "L1", "Rule": "L1_TAG", "TA Completeness": 1, "TA Docker Images": "1.0.0 b80767bd1a2c"},
		{"Team": "coral", "Lab": "L1", "Rule": "L1_TAG", "TA Completeness": 1, "TA Screenshots": 1}`)
	result, err := Export(ExportOptions{
		Labs: []LabEvaluations{{
			Lab:   "L1",
			Files: []*File{evaluationOf("teal", rule("L1_TAG", 1)), evaluationOf("coral", rule("L1_TAG", 1))},
			Facts: map[string]map[string]RuleFacts{
				"teal":  {"L1_TAG": {Images: "1.0.0 b80767bd1a2c, 1.0.1 0f3e5d6c7b8a"}},
				"coral": {"L1_TAG": {}},
			},
		}},
		Last: last,
	})
	if err != nil {
		t.Fatal(err)
	}
	var facts []string
	for _, line := range changes(result) {
		if strings.Contains(line, "Docker Images") || strings.Contains(line, "Screenshots") {
			facts = append(facts, line)
		}
	}
	want := []string{
		"Evaluation coral L1 L1_TAG TA Screenshots: 1 -> 0",
		"Evaluation teal L1 L1_TAG TA Docker Images: 1.0.0 b80767bd1a2c -> 1.0.0 b80767bd1a2c, 1.0.1 0f3e5d6c7b8a",
		"Evaluation teal L1 L1_TAG TA Screenshots: <nil> -> 0",
	}
	if !reflect.DeepEqual(facts, want) {
		t.Errorf("Changes of the facts =\n%s\nwant\n%s", strings.Join(facts, "\n"), strings.Join(want, "\n"))
	}
}

func TestExportTheRules(t *testing.T) {
	last := lastExport(t, "",
		map[string]any{"Lab Session": "L1", "Rule": "BLD", "Rule Name": "Build Test", "Coefficient": json.Number("0.6"), "Exercise": json.Number("1"),
			"Rule Description": "The team is expected to build the image of the application."},
		map[string]any{"Lab Session": "L3", "Rule": "OLD", "Rule Name": "Former Test"},
	)
	result, err := Export(ExportOptions{
		Rules: []rubric.Rule{ruleOf("L1_BLD", "Build Test", "1", 0.5), ruleOf("L2_POD", "Pod Tests", "2.1", 1)},
		Last:  last,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`{"Coefficient":0.5,"Exercise":1,"Lab Session":"L1","Rule":"BLD","Rule Description":"The team is expected to build the image of the application.","Rule Name":"Build Test"}`,
		`{"Coefficient":1,"Exercise":2.1,"Lab Session":"L2","Rule":"POD","Rule Description":"The team is expected to build the image of the application.","Rule Name":"Pod Tests"}`,
	}
	if got := rowsOf(t, result, RulesTab); !reflect.DeepEqual(got, want) {
		t.Errorf("rows =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if got, want := changes(result), []string{"Rules L1_BLD Coefficient: 0.6 -> 0.5"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Changes = %q, want %q", got, want)
	}
	if got, want := result.Added[RulesTab], []string{"L2_POD"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Added = %q, want %q", got, want)
	}
	if got, want := result.Removed, []string{"L3_OLD"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Removed = %q, want %q", got, want)
	}
	tab := result.Export.Tab(RulesTab)
	if got, want := tab.Formulas["Score"], "=20*E{row}"; got != want {
		t.Errorf("the formula of the Score = %q, want %q", got, want)
	}
}

// TestTheFormulasFollowTheLayout checks the letters that the formulas use against the layout.
func TestTheFormulasFollowTheLayout(t *testing.T) {
	letters := map[string]string{
		"E": headerRule, "F": headerRuleScore, "G": headerCompleteness, "I": headerScore, "T": headerPenalty,
		"A": headerTeam, "D": headerLab,
	}
	for letter, header := range letters {
		if got := sheetColumns[letter]; got != header {
			t.Errorf("the column %s is %q, whereas the formulas use it as %q", letter, got, header)
		}
	}
	if _, ok := sheetColumns["H"]; ok {
		t.Error("the column H is the completeness of the teacher, which mc never writes")
	}
	if got := EvaluationLayout().Confirmed; !reflect.DeepEqual(got, map[string]string{"H": "Completeness"}) {
		t.Errorf("the confirmed column = %v, want H, the completeness of the teacher", got)
	}
	if got, want := scoreFormula("7"), "=IF(ISBLANK(H7),G7,H7)*F7+T7"; got != want {
		t.Errorf("scoreFormula = %q, want %q", got, want)
	}
}

func TestExportRefuses(t *testing.T) {
	moved := lastExport(t, "")
	moved.Tabs[1].Columns["W"] = "TA Extra"
	formula := rule("L2_POD", 1)
	formula.ToCheck = "=1+1"
	for _, tc := range []struct {
		name string
		opts ExportOptions
		want string
	}{
		{"another layout", ExportOptions{Last: moved}, "the tab Evaluation of the last export has another layout"},
		{"a formula in a text", ExportOptions{Labs: []LabEvaluations{{Lab: "L2", Files: []*File{evaluationOf("teal", formula)}}}}, "starts with =, which the tab would read as a formula"},
		{"a rule ID without its lab", ExportOptions{Rules: []rubric.Rule{{ID: "L2_POD", Lab: "L1"}}}, "its ID is not its lab L1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Export(tc.opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Export = %v, want an error with %q", err, tc.want)
			}
		})
	}
}
