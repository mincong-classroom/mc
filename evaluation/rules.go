package evaluation

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/mincong-classroom/mc/rubric"
	"github.com/mincong-classroom/mc/sheet"
)

// RulesTab is the tab of the evaluation sheet that holds a copy of the rules of the YAML, where
// the tab Evaluation looks up the score of each rule.
const RulesTab = "Rules"

// The headers of the columns of the tab Rules.
const (
	headerLabSession  = "Lab Session"
	headerRuleEnd     = "Rule" // The end of the Rule ID: FBP for L2_FBP.
	headerRuleID      = "Rule ID"
	headerRuleName    = "Rule Name"
	headerCoefficient = "Coefficient"
	headerRulePoints  = "Score"
	headerExercise    = "Exercise"
	headerExerciseID  = "Exercise ID"
	headerDescription = "Rule Description"
)

// rulesLayout is the layout of the tab Rules. Its column J, Troubleshooting Instructions, is the
// teacher's: mc never reads it.
var rulesLayout = layout{
	tab:  RulesTab,
	keys: []string{headerLabSession, headerRuleEnd},
	columns: map[string]string{
		"A": headerLabSession,
		"B": headerRuleEnd,
		"C": headerRuleID,
		"D": headerRuleName,
		"E": headerCoefficient,
		"F": headerRulePoints,
		"G": headerExercise,
		"H": headerExerciseID,
		"I": headerDescription,
	},
	command: "mc rule sync",
}

// RulesSheetFile returns an empty file of the tab Rules, with the columns that mc reads: the
// layout of the script that reads the tab.
func RulesSheetFile() *sheet.File {
	return rulesLayout.file()
}

// RulesSyncOptions are the options of SyncRules.
type RulesSyncOptions struct {
	// Rules are the rules of the YAML, in their order.
	Rules []rubric.Rule
	// Read is the tab, as read after the last paste.
	Read *sheet.File
	// Exported is what mc wrote last in the tab, nil before the first export: a cell of the tab
	// that differs from it was changed by someone else, such as the teacher.
	Exported *sheet.File
}

// RulesSyncResult is what SyncRules computed.
type RulesSyncResult struct {
	// Pending is what waits to be written now: the file of mc sheet.
	Pending *sheet.File
	// Changes are the cells of the rows of the tab that the sync writes, in the order of the rules.
	Changes []Change
	// Added are the rules that the tab doesn't have yet, each one in a new row.
	Added []RuleRow
	// Extra are the rows of the tab whose rule the YAML no longer has: mc never deletes them.
	Extra []RuleRow
}

// RuleRow is a rule and its row in the tab.
type RuleRow struct {
	ID  string // Such as L2_FBP.
	Row int
}

// SyncRules computes the cells that write the rules of the YAML in the tab Rules:
//
//   - A row is found by its Lab Session and its Rule, the end of the Rule ID. A missing rule gets
//     a new row after the last one, with the formulas of C, F and H.
//   - The name, the coefficient, the exercise and the description follow the YAML. A cell that mc
//     didn't write, such as a coefficient of the teacher, waits with a check.
//   - A formula whose value is not the one that it gives, such as a Rule ID typed over it, is
//     written again with a check.
//   - A row whose rule the YAML no longer has is listed, never deleted.
func SyncRules(opts RulesSyncOptions) (*RulesSyncResult, error) {
	if opts.Read == nil {
		return nil, errors.New("no read of the tab")
	}
	if err := rulesLayout.check("the tab read", opts.Read, true); err != nil {
		return nil, err
	}
	read := ruleRowsByKey(opts.Read)
	exported := map[string]sheet.Row{}
	if opts.Exported != nil {
		exported = ruleRowsByKey(opts.Exported)
	}
	last := 0
	for _, row := range opts.Read.Rows {
		last = max(last, row.Number)
	}

	result := &RulesSyncResult{Pending: RulesSheetFile()}
	synced := map[string]bool{}
	for _, rule := range opts.Rules {
		lab, end, ok := strings.Cut(rule.ID, "_")
		if !ok || lab != rule.Lab || end == "" {
			return nil, fmt.Errorf("the rule %s: its ID is not its lab %s, an underscore, then its letters", rule.ID, rule.Lab)
		}
		cells, err := ruleCells(rule)
		if err != nil {
			return nil, err
		}
		k := ruleKey(lab, end)
		synced[k] = true
		r := ruleSync{rule: rule, result: result}
		if row, ok := read[k]; ok {
			r.row = sheet.Row{Number: row.Number, Cells: map[string]any{}}
			r.read, r.exported = row, exported[k]
			r.existing(cells)
		} else {
			last++
			r.row = sheet.Row{Number: last, Cells: map[string]any{}, New: true}
			r.added(cells)
			result.Added = append(result.Added, RuleRow{ID: rule.ID, Row: last})
		}
		if len(r.row.Cells) == 0 {
			continue
		}
		r.row.Cells[headerLabSession] = lab
		r.row.Cells[headerRuleEnd] = end
		result.Pending.Rows = append(result.Pending.Rows, r.row)
	}
	for _, row := range opts.Read.Rows {
		lab, end := text(row.Cells[headerLabSession]), text(row.Cells[headerRuleEnd])
		if !synced[ruleKey(lab, end)] {
			result.Extra = append(result.Extra, RuleRow{ID: lab + "_" + end, Row: row.Number})
		}
	}

	pending := result.Pending
	sort.SliceStable(pending.Rows, func(i, j int) bool { return pending.Rows[i].Number < pending.Rows[j].Number })
	if problems := pending.Problems(); len(problems) > 0 {
		return nil, fmt.Errorf("the rows to write are not valid: %s", strings.Join(problems, "; "))
	}
	return result, nil
}

// ruleCell is a cell of a rule that follows the YAML.
type ruleCell struct {
	header string
	value  any
}

// ruleCells returns the cells of a rule that follow the YAML, in the order of the columns. An empty
// value has no cell, since a cell is never cleared.
func ruleCells(rule rubric.Rule) ([]ruleCell, error) {
	var cells []ruleCell
	for _, c := range []ruleCell{
		{headerRuleName, oneLine(rule.Name)},
		{headerCoefficient, json.Number(formatFloat(rule.Coefficient))},
		{headerExercise, exerciseValue(rule.Exercise)},
		{headerDescription, oneLine(rule.Description)},
	} {
		s, ok := c.value.(string)
		switch {
		case ok && s == "":
			continue
		case ok && strings.HasPrefix(s, "="):
			return nil, fmt.Errorf("the rule %s, %s: %s starts with =, which the tab would read as a formula", rule.ID, c.header, brief(s))
		}
		cells = append(cells, c)
	}
	return cells, nil
}

// exerciseValue returns the exercise of a rule as a number, such as 6 or 4.1, which the tab holds
// as a number, or as a text when it is not one.
func exerciseValue(exercise string) any {
	exercise = strings.TrimSpace(exercise)
	if _, err := strconv.ParseFloat(exercise, 64); err == nil {
		return json.Number(exercise)
	}
	return exercise
}

// ruleSync computes the cells of the row of a rule.
type ruleSync struct {
	rule     rubric.Rule
	result   *RulesSyncResult
	row      sheet.Row // The cells to write.
	read     sheet.Row // The row of the tab, empty for a new row.
	exported sheet.Row // The row of the last export, empty if none.
}

// added writes a new row: its cells, then its formulas, the same as those of the rows of the tab.
func (r *ruleSync) added(cells []ruleCell) {
	for _, c := range cells {
		r.row.Cells[c.header] = c.value
	}
	n := strconv.Itoa(r.row.Number)
	r.row.Cells[headerRuleID] = `=CONCAT(A` + n + `, CONCAT("_", B` + n + `))`
	r.row.Cells[headerRulePoints] = "=20*E" + n
	r.row.Cells[headerExerciseID] = `=CONCAT(CONCAT(A` + n + `, "_"), CONCAT("EX", G` + n + `))`
}

// existing writes the cells of a row of the tab that change, then the formulas that the tab lost.
func (r *ruleSync) existing(cells []ruleCell) {
	for _, c := range cells {
		r.set(c.header, c.value)
	}
	n := strconv.Itoa(r.row.Number)
	lab, end := text(r.read.Cells[headerLabSession]), text(r.read.Cells[headerRuleEnd])
	exercise := text(r.read.Cells[headerExercise])
	r.formula(headerRuleID, `=CONCAT(A`+n+`, CONCAT("_", B`+n+`))`, lab+"_"+end)
	r.formula(headerExerciseID, `=CONCAT(CONCAT(A`+n+`, "_"), CONCAT("EX", G`+n+`))`, lab+"_EX"+exercise)
	if coefficient, ok := number(r.read.Cells[headerCoefficient]); ok {
		points := strconv.FormatFloat(20*coefficient, 'f', -1, 64)
		if got, ok := number(r.read.Cells[headerRulePoints]); !ok || math.Abs(got-20*coefficient) > epsilon {
			r.rewrite(headerRulePoints, "=20*E"+n, points)
		}
	}
}

// set writes a cell, unless the tab holds its value already. A cell that holds another value, which
// mc didn't write, such as one of the teacher, waits with a check.
func (r *ruleSync) set(header string, value any) {
	from, written := r.read.Cells[header], r.exported.Cells[header]
	if !isEmpty(from) && sheet.SameValue(from, value) {
		return
	}
	r.row.Cells[header] = value
	c := Change{Cell: r.cell(header), Row: r.rule.ID, Header: header, From: from, To: value}
	if !isEmpty(from) && (written == nil || !sheet.SameValue(from, written)) {
		c.Check = fmt.Sprintf("the tab has %s, which mc didn't write", brief(from))
		r.check(header, c.Check)
	}
	r.result.Changes = append(r.result.Changes, c)
}

// formula writes a formula again when the cell doesn't hold the text that it gives.
func (r *ruleSync) formula(header, formula, want string) {
	if strings.TrimSpace(text(r.read.Cells[header])) != want {
		r.rewrite(header, formula, want)
	}
}

// rewrite writes a formula over the value of the tab, with a check: someone may have typed it.
func (r *ruleSync) rewrite(header, formula, want string) {
	from := r.read.Cells[header]
	r.row.Cells[header] = formula
	why := fmt.Sprintf("the tab has %s, whereas the formula gives %s", brief(from), strconv.Quote(want))
	r.check(header, why)
	r.result.Changes = append(r.result.Changes, Change{
		Cell: r.cell(header), Row: r.rule.ID, Header: header, From: from, To: formula, Check: why,
	})
}

// check records why the teacher should look at a cell before it is written.
func (r *ruleSync) check(header, why string) {
	if r.row.Check == nil {
		r.row.Check = map[string]string{}
	}
	r.row.Check[header] = why
}

// cell returns the address of the cell of the row under the header, such as Rules!E5.
func (r *ruleSync) cell(header string) string {
	return RulesTab + "!" + rulesLayout.letterOf(header) + strconv.Itoa(r.row.Number)
}

// ruleRowsByKey returns the rows of a file of the tab Rules by their keys.
func ruleRowsByKey(f *sheet.File) map[string]sheet.Row {
	rows := make(map[string]sheet.Row, len(f.Rows))
	for _, row := range f.Rows {
		rows[ruleKey(text(row.Cells[headerLabSession]), text(row.Cells[headerRuleEnd]))] = row
	}
	return rows
}

func ruleKey(lab, end string) string {
	return lab + "\x1f" + end
}

// oneLine returns a text on one line, its spaces and line breaks squeezed to one space.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
