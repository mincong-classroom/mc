package evaluation

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mincong-classroom/mc/sheet"
)

// tab returns a file of the tab Evaluation, with the columns of mc and the rows given, in JSON.
func tab(t *testing.T, rows string) *sheet.File {
	t.Helper()
	columns, err := json.Marshal(sheetColumns)
	if err != nil {
		t.Fatal(err)
	}
	var f sheet.File
	data := `{"tab": "Evaluation", "keys": ["Team", "Lab", "Rule"], "columns": ` + string(columns) + `, "rows": [` + rows + `]}`
	if err := json.Unmarshal([]byte(data), &f); err != nil {
		t.Fatal(err)
	}
	return &f
}

// evaluationOf returns the evaluation of the team, with its rules.
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

// changes returns the changes of the result, one line each.
func changes(result *SyncResult) []string {
	lines := make([]string, len(result.Changes))
	for i, c := range result.Changes {
		lines[i] = fmt.Sprintf("%s %s: %v -> %v", c.Cell, c.Header, c.From, c.To)
		if c.Kept != "" {
			lines[i] += ", kept: " + c.Kept
		}
		if c.Check != "" {
			lines[i] += ", check: " + c.Check
		}
	}
	return lines
}

func TestSyncAddsTheRowsOfALab(t *testing.T) {
	read := tab(t, `{"_row": 2, "Team": "teal", "Lab": "L1", "Rule": "L1_BLD", "Completeness": "1"},
		{"_row": 3, "Team": "coral", "Lab": "L1", "Rule": "L1_BLD", "Completeness": "0.5"}`)
	withPenalty := rule("L2_LOG", 1)
	withPenalty.Penalty = &Penalty{Kind: PenaltyRule, Remove: 0.1, Why: "The logs come from another Pod"}
	toCheck := rule("L2_POD", 0.5)
	toCheck.ToCheck = "The tag 2026.0 is not on Docker Hub."

	result, err := Sync(SyncOptions{
		Lab:   "L2",
		Files: []*File{evaluationOf("teal", rule("L2_POD", 0.8), withPenalty), evaluationOf("coral", toCheck)},
		Read:  read,
	})
	if err != nil {
		t.Fatal(err)
	}

	// coral first, then teal, after the last row of the tab.
	if want := []Added{{Team: "coral", First: 4, Last: 4}, {Team: "teal", First: 5, Last: 6}}; !reflect.DeepEqual(result.Added, want) {
		t.Errorf("Added = %+v, want %+v", result.Added, want)
	}
	if result.FillFrom != 3 || result.LastNewRow != 6 {
		t.Errorf("FillFrom, LastNewRow = %d, %d, want 3, 6", result.FillFrom, result.LastNewRow)
	}
	if len(result.Changes) != 0 {
		t.Errorf("Changes = %q, want none: a new row has no former value", changes(result))
	}
	rows := result.Pending.Rows
	if len(rows) != 3 {
		t.Fatalf("Pending has %d rows, want 3: %+v", len(rows), rows)
	}
	want := map[string]any{
		"Team": "coral", "Lab": "L2", "Rule": "L2_POD",
		"Rule Score":       "=VLOOKUP(E4,Rules!C:F,4,FALSE)",
		"Completeness":     json.Number("0.5"),
		"Score":            "=G4*F4+S4",
		"TA To Check":      "The tag 2026.0 is not on Docker Hub.",
		"TA Commit":        "0a1b2c3",
		"TA Analysis":      "short",
		"TA Writing Style": "own words",
		"TA Time Spent":    "Session only",
		"TA Lab Flag":      `=IF(SUMIFS($H:$H,$A:$A,A4,$D:$D,D4)<12,"Below 12/20 ("&ROUND(SUMIFS($H:$H,$A:$A,A4,$D:$D,D4),1)&")","")`,
		"TA Action Items":  "Run it yourself",
		"TA Comment":       "Done well: The Pod runs. To improve: Show the logs.",
	}
	if rows[0].Number != 4 || !rows[0].New || rows[0].Check != nil || !reflect.DeepEqual(rows[0].Cells, want) {
		t.Errorf("row of coral = %d, new %t, check %v, cells\n%v\nwant 4, new, no check, cells\n%v", rows[0].Number, rows[0].New, rows[0].Check, rows[0].Cells, want)
	}
	if got := rows[2].Cells["TA Penalty"]; got != "=-0.1*G6*F6" {
		t.Errorf("TA Penalty of teal L2_LOG = %v, want =-0.1*G6*F6", got)
	}
	if got, want := rows[2].Cells["TA Comment"], "Done well: The Pod runs. To improve: Show the logs. Penalty: The logs come from another Pod."; got != want {
		t.Errorf("TA Comment of teal L2_LOG = %q, want %q", got, want)
	}
	if _, ok := rows[1].Cells["TA Penalty"]; ok {
		t.Errorf("teal L2_POD has a TA Penalty without a penalty: %v", rows[1].Cells)
	}
}

func TestSyncAPassAfterAFormerOne(t *testing.T) {
	read := tab(t, `{"_row": 2, "Team": "teal", "Lab": "L2", "Rule": "L2_POD", "Completeness": "0.6", "TA Comment": "Done well: old.", "TA Analysis": "short", "TA Commit": "0a1b2c3"},
		{"_row": 3, "Team": "teal", "Lab": "L2", "Rule": "L2_LOG", "Completeness": "1", "TA Analysis": "short"},
		{"_row": 4, "Team": "coral", "Lab": "L2", "Rule": "L2_POD", "Completeness": "0.4", "TA Penalty": "-0.2"},
		{"_row": 5, "Team": "olive", "Lab": "L2", "Rule": "L2_POD", "TA Action Items": "Talk with them"}`)
	exported := tab(t, `{"_row": 2, "Team": "teal", "Lab": "L2", "Rule": "L2_POD", "Completeness": 0.6, "TA Comment": "Done well: old.", "TA Analysis": "detailed"},
		{"_row": 3, "Team": "teal", "Lab": "L2", "Rule": "L2_LOG", "Completeness": 1}`)
	detailed := rule("L2_POD", 0.9)
	detailed.Analysis = "detailed"

	result, err := Sync(SyncOptions{
		Lab:      "L2",
		Files:    []*File{evaluationOf("teal", detailed, rule("L2_LOG", 0.8)), evaluationOf("coral", rule("L2_POD", 0.7)), evaluationOf("olive", rule("L2_POD", 1))},
		Read:     read,
		Exported: exported,
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		// coral: a completeness that mc didn't write, and a penalty that the pass doesn't give.
		"Evaluation!G4 Completeness: 0.4 -> 0.7, kept: not written by mc, such as a value of the teacher",
		"Evaluation!K4 TA To Check: <nil> -> The pass of 0a1b2c3 gives 0.7, whereas the tab has 0.4, which mc didn't write: it is kept.",
		"Evaluation!S4 TA Penalty: -0.2 -> <nil>, kept: a penalty that this pass doesn't give: clear it by hand if it no longer applies",
		"Evaluation!L4 TA Commit: <nil> -> 0a1b2c3",
		"Evaluation!O4 TA Analysis: <nil> -> short",
		"Evaluation!P4 TA Writing Style: <nil> -> own words",
		"Evaluation!Q4 TA Time Spent: <nil> -> Session only",
		"Evaluation!T4 TA Action Items: <nil> -> Run it yourself",
		// olive: an empty completeness, and action items that mc didn't write.
		"Evaluation!G5 Completeness: <nil> -> 1",
		"Evaluation!U5 TA Comment: <nil> -> Done well: The Pod runs. To improve: Show the logs.",
		"Evaluation!L5 TA Commit: <nil> -> 0a1b2c3",
		"Evaluation!O5 TA Analysis: <nil> -> short",
		"Evaluation!P5 TA Writing Style: <nil> -> own words",
		"Evaluation!Q5 TA Time Spent: <nil> -> Session only",
		`Evaluation!T5 TA Action Items: Talk with them -> Run it yourself, check: the tab has "Talk with them", which mc didn't write`,
		// teal L2_POD: a higher completeness over the one of mc, with its comment.
		"Evaluation!G2 Completeness: 0.6 -> 0.9",
		"Evaluation!U2 TA Comment: Done well: old. -> Done well: The Pod runs. To improve: Show the logs.",
		`Evaluation!O2 TA Analysis: short -> detailed, check: the tab has "short", which mc didn't write`,
		"Evaluation!P2 TA Writing Style: <nil> -> own words",
		"Evaluation!Q2 TA Time Spent: <nil> -> Session only",
		"Evaluation!T2 TA Action Items: <nil> -> Run it yourself",
		// teal L2_LOG: a lower completeness keeps the former one.
		"Evaluation!G3 Completeness: 1 -> 0.8, kept: lower than the former pass",
		"Evaluation!K3 TA To Check: <nil> -> The pass of 0a1b2c3 gives 0.8, lower than 1, which is kept: what changed?",
		"Evaluation!L3 TA Commit: <nil> -> 0a1b2c3",
		"Evaluation!P3 TA Writing Style: <nil> -> own words",
		"Evaluation!Q3 TA Time Spent: <nil> -> Session only",
		"Evaluation!T3 TA Action Items: <nil> -> Run it yourself",
	}
	if got := changes(result); !reflect.DeepEqual(got, want) {
		t.Errorf("Changes =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if len(result.Added) != 0 || result.FillFrom != 0 || result.LastNewRow != 0 {
		t.Errorf("Added %+v, FillFrom %d, LastNewRow %d, want no new row", result.Added, result.FillFrom, result.LastNewRow)
	}
	for _, row := range result.Pending.Rows {
		if row.New {
			t.Errorf("row %d is new, whereas the tab has it", row.Number)
		}
		if row.Number == 5 && !reflect.DeepEqual(row.Check, map[string]string{"TA Action Items": `the tab has "Talk with them", which mc didn't write`}) {
			t.Errorf("Check of the row 5 = %v", row.Check)
		}
	}
}

func TestSyncReplacesWhatWaited(t *testing.T) {
	read := tab(t, `{"_row": 2, "Team": "teal", "Lab": "L1", "Rule": "L1_BLD", "Completeness": "1"}`)
	pending := tab(t, `{"_row": 3, "_new": true, "Team": "teal", "Lab": "L2", "Rule": "L2_POD", "Completeness": 0.5},
		{"_row": 4, "_new": true, "Team": "coral", "Lab": "L2", "Rule": "L2_POD", "Completeness": 0.6},
		{"_row": 5, "_new": true, "Team": "olive", "Lab": "L3", "Rule": "L3_SVC", "Completeness": 0.7},
		{"_row": 2, "Team": "teal", "Lab": "L1", "Rule": "L1_BLD", "TA Analysis": "short"}`)

	result, err := Sync(SyncOptions{Lab: "L2", Files: []*File{evaluationOf("teal", rule("L2_POD", 0.9), rule("L2_LOG", 1))}, Read: read, Pending: pending})
	if err != nil {
		t.Fatal(err)
	}

	// The row 3 of teal again, then a new row after those that wait; coral and olive wait still.
	var got []string
	for _, row := range result.Pending.Rows {
		got = append(got, fmt.Sprintf("%d %s %s %s %v", row.Number, row.Cells["Team"], row.Cells["Lab"], row.Cells["Rule"], row.Cells["Completeness"]))
	}
	want := []string{"2 teal L1 L1_BLD <nil>", "3 teal L2 L2_POD 0.9", "4 coral L2 L2_POD 0.6", "5 olive L3 L3_SVC 0.7", "6 teal L2 L2_LOG 1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Pending =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if result.Replaced != 1 || result.FillFrom != 2 || result.LastNewRow != 6 {
		t.Errorf("Replaced %d, FillFrom %d, LastNewRow %d, want 1, 2, 6", result.Replaced, result.FillFrom, result.LastNewRow)
	}
	if want := []Added{{Team: "teal", First: 3, Last: 3}, {Team: "teal", First: 6, Last: 6}}; !reflect.DeepEqual(result.Added, want) {
		t.Errorf("Added = %+v, want %+v", result.Added, want)
	}
}

func TestSyncRefuses(t *testing.T) {
	teal := []*File{evaluationOf("teal", rule("L2_POD", 1))}
	withoutI := tab(t, "")
	delete(withoutI.Columns, "I")
	renamed := tab(t, "")
	renamed.Columns["K"] = "To Check"
	formula := rule("L2_POD", 1)
	formula.ToCheck = "=1+1"

	for _, tt := range []struct {
		name string
		opts SyncOptions
		want string
	}{
		{name: "no read", opts: SyncOptions{Lab: "L2", Files: teal}, want: "no read of the tab"},
		{name: "a column not read", opts: SyncOptions{Lab: "L2", Files: teal, Read: withoutI}, want: `the tab read has no column I, "Comment"`},
		{name: "a header changed", opts: SyncOptions{Lab: "L2", Files: teal, Read: renamed}, want: `the column K of the tab read is "To Check", whereas mc writes "TA To Check" there`},
		{
			name: "a new row that waits, whereas the tab has its row now",
			opts: SyncOptions{Lab: "L2", Files: teal, Read: tab(t, `{"_row": 2, "Team": "coral", "Lab": "L1", "Rule": "L1_BLD"}`),
				Pending: tab(t, `{"_row": 2, "_new": true, "Team": "olive", "Lab": "L3", "Rule": "L3_SVC", "Completeness": 1}`)},
			want: "the new row 2 of olive L3 L3_SVC waits, whereas the tab has the row 2 of coral L1 L1_BLD now: sync L3 again",
		},
		{
			name: "a row that waits, whereas the tab moved it",
			opts: SyncOptions{Lab: "L2", Files: teal, Read: tab(t, `{"_row": 3, "Team": "olive", "Lab": "L3", "Rule": "L3_SVC"}`),
				Pending: tab(t, `{"_row": 2, "Team": "olive", "Lab": "L3", "Rule": "L3_SVC", "Completeness": 1}`)},
			want: "the row 2 of olive L3 L3_SVC waits, whereas the tab has it in the row 3 now: sync L3 again",
		},
		{
			name: "a text read as a formula",
			opts: SyncOptions{Lab: "L2", Files: []*File{evaluationOf("teal", formula)}, Read: tab(t, "")},
			want: `teal L2_POD, TA To Check: "=1+1" starts with =`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Sync(tt.opts); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Sync = %v, want an error with %q", err, tt.want)
			}
		})
	}
}
