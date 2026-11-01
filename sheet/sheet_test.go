package sheet

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestColumns(t *testing.T) {
	for letters, index := range map[string]int{"A": 0, "G": 6, "Z": 25, "AA": 26, "AZ": 51, "BA": 52, "ZZ": 701, "AAA": 702} {
		if got, err := columnIndex(letters); err != nil || got != index {
			t.Errorf("columnIndex(%q) = %d, %v, want %d", letters, got, err, index)
		}
		if got := columnLetters(index); got != letters {
			t.Errorf("columnLetters(%d) = %q, want %q", index, got, letters)
		}
	}
	for _, bad := range []string{"", "a", "A1", "AAAA"} {
		if _, err := columnIndex(bad); err == nil {
			t.Errorf("columnIndex(%q): no error", bad)
		}
	}
}

// pending is a file of the tab Evaluation, with made-up teams, whose attributes are out of order.
const pending = `{
  "tab": "Evaluation",
  "keys": ["Team", "Lab", "Rule"],
  "columns": {
    "N": "TA Screenshots", "A": "Team", "D": "Lab", "E": "Rule", "G": "Completeness",
    "K": "TA To Check", "O": "TA Analysis", "S": "TA Action Items"
  },
  "rows": [
    {"TA Analysis": "short", "TA Screenshots": 2, "_row": 80, "Team": "teal", "Lab": "L2", "Rule": "L2_CTL", "TA Action Items": "Run it yourself"},
    {"_row": 81, "Team": "teal", "Lab": "L2", "Rule": "L2_RUN", "TA To Check": "The tag 2026.0 is not on Docker Hub.", "TA Screenshots": 0, "TA Analysis": "none", "TA Action Items": "Run it yourself"},
    {"_row": 82, "Team": "teal", "Lab": "L2", "Rule": "L2_FBP", "Completeness": 0.9, "TA Screenshots": 1, "TA Analysis": "detailed", "TA Action Items": "Run it yourself",
     "_check": {"Completeness": "0.7 at the last export, and no read of the tab since"}},
    {"_row": 83, "Team": "coral", "Lab": "L2", "Rule": "L2_CTL", "TA To Check": "-0.2: a penalty <to confirm> & more", "TA Screenshots": 3, "TA Analysis": "short", "TA Action Items": "Less attention"},
    {"_row": 84, "Team": "coral", "Lab": "L2", "Rule": "L2_RUN", "TA To Check": "2e88112", "TA Screenshots": 1, "TA Analysis": "short", "TA Action Items": "Less attention"}
  ]
}
`

func load(t *testing.T, content string) *File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "evaluation.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestSaveOrdersTheAttributesLikeTheColumns(t *testing.T) {
	f := load(t, pending)
	f.Rows = f.Rows[2:4]
	path := filepath.Join(t.TempDir(), "sheet", "evaluation.json")
	if err := f.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "tab": "Evaluation",
  "keys": [
    "Team",
    "Lab",
    "Rule"
  ],
  "columns": {
    "A": "Team",
    "D": "Lab",
    "E": "Rule",
    "G": "Completeness",
    "K": "TA To Check",
    "N": "TA Screenshots",
    "O": "TA Analysis",
    "S": "TA Action Items"
  },
  "rows": [
    {
      "_row": 82,
      "Team": "teal",
      "Lab": "L2",
      "Rule": "L2_FBP",
      "Completeness": 0.9,
      "TA Screenshots": 1,
      "TA Analysis": "detailed",
      "TA Action Items": "Run it yourself",
      "_check": {
        "Completeness": "0.7 at the last export, and no read of the tab since"
      }
    },
    {
      "_row": 83,
      "Team": "coral",
      "Lab": "L2",
      "Rule": "L2_CTL",
      "TA To Check": "-0.2: a penalty <to confirm> & more",
      "TA Screenshots": 3,
      "TA Analysis": "short",
      "TA Action Items": "Less attention"
    }
  ]
}
`
	if string(got) != want {
		t.Errorf("Save wrote:\n%s\nwant:\n%s", got, want)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("Save: the mode of the file is %v, %v, want -rw-------", info.Mode(), err)
	}
	if _, err := Load(path); err != nil {
		t.Errorf("Load of the saved file: %v", err)
	}
}

func TestProblems(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evaluation.json")
	content := `{
  "tab": "Evaluation",
  "keys": ["Team", "Rule"],
  "columns": {"A": "Team", "E": "Rule", "G": "Completeness", "a1": "Comment", "K": "Team"},
  "rows": [
    {"_row": 1, "Team": "teal", "Rule": "L2_CTL"},
    {"_row": 80, "Team": "teal", "Rule": "L2_RUN", "Completness": 0.9, "Completeness": ""},
    {"_row": 80, "Team": "teal", "Rule": "L2_RUN", "_check": {"Completeness": "why"}},
    {"_row": 81, "Rule": "L2_FBP", "Completeness": null}
  ]
}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("Load: no error")
	}
	for _, want := range []string{
		`columns: "a1" is not a column`,
		`columns: the header "Team" is the one of A and K`,
		`rows[0]: the attribute _row is 1`,
		`rows[1] (row 80): "Completness" is not a header of the columns`,
		`rows[1] (row 80): Completeness: an empty text "" would clear the cell`,
		`rows[2] (row 80): the row of rows[1] too`,
		`rows[2] (row 80): _check: "Completeness" is not a cell of the row`,
		`rows[2] (row 80): the keys of rows[1] too, teal L2_RUN`,
		`rows[3] (row 81): Completeness: <nil> is not a text, a number or a boolean`,
		`rows[3] (row 81): no key "Team", a text`,
	} {
		if !strings.Contains(err.Error(), path+": "+want) {
			t.Errorf("Load: the error misses %q:\n%v", want, err)
		}
	}

	if err := os.WriteFile(path, []byte(`{"tab": "Evaluation", "keys": ["Team"], "columns": {"A": "Team"}, "rows": [{"_row": 2, "_rwo": 3, "Team": "teal"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "rows[0]: _rwo: unknown attribute") {
		t.Errorf("Load of an unknown attribute: %v", err)
	}
}

func TestPasteText(t *testing.T) {
	for _, tt := range []struct {
		value any
		want  string
	}{
		{value: "detailed", want: "detailed"},
		{value: "Session + 2 h, until 14:28", want: "Session + 2 h, until 14:28"},
		{value: "1.1.0 0f3e5d6c7b8a", want: "1.1.0 0f3e5d6c7b8a"},
		{value: "=G2*F2+S2", want: "=G2*F2+S2"},
		{value: "2e88112", want: "'2e88112"},
		{value: "0123", want: "'0123"},
		{value: "1.1.0", want: "'1.1.0"},
		{value: "-0.2: a penalty", want: "'-0.2: a penalty"},
		{value: "+1", want: "'+1"},
		{value: "'quoted", want: "''quoted"},
		{value: "true", want: "'true"},
		{value: "50%", want: "'50%"},
		{value: "30/10", want: "'30/10"},
		{value: "14:28", want: "'14:28"},
		{value: json.Number("0.9"), want: "0.9"},
		{value: true, want: "TRUE"},
	} {
		if got, err := pasteText(tt.value); err != nil || got != tt.want {
			t.Errorf("pasteText(%#v) = %q, %v, want %q", tt.value, got, err, tt.want)
		}
	}
	for _, bad := range []any{"", "a\tb", "two\nlines", nil, 1.5} {
		if got, err := pasteText(bad); err == nil {
			t.Errorf("pasteText(%#v) = %q, want an error", bad, got)
		}
	}
}

func TestBlocks(t *testing.T) {
	blocks, err := Blocks(load(t, pending))
	if err != nil {
		t.Fatal(err)
	}
	var ranges []string
	for _, b := range blocks {
		ranges = append(ranges, b.Range()+" "+b.Size())
	}
	// K: the rows 81, 83 and 84 only. N and O: every row. G: the row 82, with a check, last.
	want := []string{
		"Evaluation!K81 1 × 1",
		"Evaluation!K83:K84 2 × 1",
		"Evaluation!N80:O84 5 × 2",
		"Evaluation!S80:S84 5 × 1",
		"Evaluation!G82 1 × 1",
	}
	if !reflect.DeepEqual(ranges, want) {
		t.Errorf("Blocks = %q, want %q", ranges, want)
	}
	if got, want := blocks[1].TSV(), "'-0.2: a penalty <to confirm> & more\n'2e88112\n"; got != want {
		t.Errorf("TSV of K83:K84 = %q, want %q", got, want)
	}
	if got, want := blocks[2].TSV(), "2\tshort\n0\tnone\n1\tdetailed\n3\tshort\n1\tshort\n"; got != want {
		t.Errorf("TSV of N80:O84 = %q, want %q", got, want)
	}
	if got, want := blocks[4].Check, []string{"G82 Completeness: 0.7 at the last export, and no read of the tab since"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Check of G82 = %q, want %q", got, want)
	}
	if b := (Block{Tab: "Scores Final", FirstRow: 2, LastRow: 2, FirstColumn: 0, LastColumn: 1}); b.Range() != "'Scores Final'!A2:B2" {
		t.Errorf("Range of a tab with a space = %q", b.Range())
	}
}

func TestRecordPasted(t *testing.T) {
	p := load(t, pending)
	blocks, err := Blocks(p)
	if err != nil {
		t.Fatal(err)
	}
	exported := NewExport(p)
	// The teacher pastes every block but the one to check, G82.
	count, err := RecordPasted(p, exported, blocks[:4])
	if err != nil {
		t.Fatal(err)
	}
	if count != 1+2+10+5 {
		t.Errorf("RecordPasted moved %d cells, want 18", count)
	}
	if len(p.Rows) != 1 || p.Rows[0].Number != 82 || !reflect.DeepEqual(p.Rows[0].Cells, map[string]any{
		"Team": "teal", "Lab": "L2", "Rule": "L2_FBP", "Completeness": json.Number("0.9"),
	}) || len(p.Rows[0].Check) != 1 {
		t.Errorf("RecordPasted left in pending: %+v", p.Rows)
	}
	if len(exported.Rows) != 5 || exported.Rows[2].Number != 82 || exported.Rows[2].Cells["Completeness"] != nil {
		t.Errorf("RecordPasted exported: %+v", exported.Rows)
	}

	// Then the last block: the row 82 takes its completeness, and nothing waits anymore.
	rest, err := Blocks(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RecordPasted(p, exported, rest); err != nil {
		t.Fatal(err)
	}
	if len(p.Rows) != 0 || exported.Rows[2].Cells["Completeness"] != json.Number("0.9") || exported.Rows[2].Check != nil {
		t.Errorf("RecordPasted of the last block: pending %+v, exported %+v", p.Rows, exported.Rows[2])
	}

	other := NewExport(p)
	other.Columns["S"] = "TA Penalty"
	if _, err := RecordPasted(load(t, pending), other, nil); err == nil || !strings.Contains(err.Error(), "the layout of the tab changed") {
		t.Errorf("RecordPasted on another layout: %v", err)
	}
}

func TestVerify(t *testing.T) {
	exported := load(t, `{
  "tab": "Evaluation",
  "keys": ["Team", "Lab", "Rule"],
  "columns": {"A": "Team", "D": "Lab", "E": "Rule", "G": "Completeness", "H": "Score", "K": "TA To Check", "S": "TA Action Items"},
  "rows": [
    {"_row": 80, "Team": "teal", "Lab": "L2", "Rule": "L2_CTL", "Completeness": 0.9, "Score": "=G80*F80+T80", "TA To Check": "2e88112"},
    {"_row": 81, "Team": "teal", "Lab": "L2", "Rule": "L2_RUN", "Completeness": 1, "TA Action Items": "Run it yourself"},
    {"_row": 82, "Team": "coral", "Lab": "L2", "Rule": "L2_CTL", "Completeness": 0.5}
  ]
}
`)
	// As the script reads the tab: texts, the headers of the tab, and a row inserted above row 81.
	read := load(t, `{
  "tab": "Evaluation",
  "keys": ["Team", "Lab", "Rule"],
  "columns": {"A": "Team", "D": "Lab", "E": "Rule", "G": "Completeness", "H": "Score", "K": "TA To Check", "S": "TA Penalty"},
  "rows": [
    {"_row": 80, "Team": "teal", "Lab": "L2", "Rule": "L2_CTL", "Completeness": "0.90", "Score": "5.4", "TA To Check": "2e88112"},
    {"_row": 82, "Team": "teal", "Lab": "L2", "Rule": "L2_RUN", "Completeness": "0.8", "TA Penalty": "Run it yourself"}
  ]
}
`)
	got, err := Verify(read, exported)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`column S: "TA Penalty" in the tab, "TA Action Items" at the export`,
		"teal L2 L2_RUN: row 82 in the tab, row 81 at the export",
		`Evaluation!G82 teal L2 L2_RUN, Completeness: "0.8" in the tab, 1 at the export`,
		"coral L2 L2_CTL: no row of the tab has these keys",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Verify =\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestSnippet(t *testing.T) {
	var b strings.Builder
	err := Snippet(&b, load(t, pending), SnippetOptions{
		Sheet: "https://docs.google.com/spreadsheets/d/0123456789/edit#gid=42",
		Path:  "~/.mc/2026/sheet/evaluation.read.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := b.String()
	for _, want := range []string{
		`// mc sheet snippet: reads the columns A, D, E, G, K, N, O, S of the tab "Evaluation"`,
		`  tab: "Evaluation",`,
		`  keys: ["Team","Lab","Rule"],`,
		`  columns: { "A": "Team", "D": "Lab", "E": "Rule", "G": "Completeness", "K": "TA To Check", "N": "TA Screenshots", "O": "TA Analysis", "S": "TA Action Items" },`,
		`  sheet: "0123456789",`,
		`  path: "~/.mc/2026/sheet/evaluation.read.json",`,
		"\n}, typeof copy === \"function\" ? copy : null);\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Snippet misses %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "CONFIG") {
		t.Errorf("Snippet left its placeholder:\n%s", got)
	}
}

func TestNewRow(t *testing.T) {
	p := load(t, `{
  "tab": "Evaluation",
  "keys": ["Team", "Lab", "Rule"],
  "columns": {"A": "Team", "D": "Lab", "E": "Rule", "G": "Completeness", "H": "Score"},
  "rows": [
    {"_row": 80, "Team": "teal", "Lab": "L2", "Rule": "L2_CTL", "Completeness": 0.9},
    {"_row": 81, "_new": true, "Team": "teal", "Lab": "L2", "Rule": "L2_RUN", "Completeness": 1, "Score": "=G81*F81+S81"}
  ]
}`)
	if !p.Rows[1].New || p.Rows[0].New {
		t.Fatalf("_new of the rows = %t, %t, want false, true", p.Rows[0].New, p.Rows[1].New)
	}
	raw, err := p.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"_row":81,"_new":true,"Team":"teal","Lab":"L2","Rule":"L2_RUN","Completeness":1,"Score":"=G81*F81+S81"}`; !strings.Contains(string(raw), want) {
		t.Errorf("MarshalJSON = %s, want the row %s", raw, want)
	}

	// The keys of the new row are written, and those of the row of the tab are not.
	blocks, err := Blocks(p)
	if err != nil {
		t.Fatal(err)
	}
	var ranges []string
	for _, b := range blocks {
		ranges = append(ranges, b.Range())
	}
	if want := []string{"Evaluation!A81", "Evaluation!G80:G81", "Evaluation!D81:E81", "Evaluation!H81"}; !reflect.DeepEqual(sorted(ranges), sorted(want)) {
		t.Errorf("Blocks = %q, want %q", ranges, want)
	}

	// A paste of every block but the keys of the new row: it stays new, with its keys only.
	var withoutKeys []Block
	for _, b := range blocks {
		if b.FirstColumn >= 6 {
			withoutKeys = append(withoutKeys, b)
		}
	}
	exported := NewExport(p)
	if _, err := RecordPasted(p, exported, withoutKeys); err != nil {
		t.Fatal(err)
	}
	if len(p.Rows) != 1 || !p.Rows[0].New || !reflect.DeepEqual(p.Rows[0].Cells, map[string]any{"Team": "teal", "Lab": "L2", "Rule": "L2_RUN"}) {
		t.Errorf("RecordPasted without the keys left in pending: %+v", p.Rows)
	}

	// Then its keys: the row is in the tab, and nothing waits anymore.
	rest, err := Blocks(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RecordPasted(p, exported, rest); err != nil {
		t.Fatal(err)
	}
	if len(p.Rows) != 0 || len(exported.Rows) != 2 || exported.Rows[1].Number != 81 || exported.Rows[1].New {
		t.Errorf("RecordPasted of the keys: pending %+v, exported %+v", p.Rows, exported.Rows)
	}
}

func sorted(s []string) []string {
	s = append([]string(nil), s...)
	sort.Strings(s)
	return s
}
