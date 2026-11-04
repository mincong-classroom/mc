package evaluation

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/mincong-classroom/mc/rubric"
	"github.com/mincong-classroom/mc/sheet"
)

// The tab of the evaluation sheet, and the headers of its columns.
const (
	SheetTab = "Evaluation"

	headerTeam         = "Team"
	headerLab          = "Lab"
	headerConfirmed    = "Completeness" // The completeness of the teacher, which confirms a row.
	headerRule         = "Rule"
	headerRuleScore    = "Rule Score"
	headerCompleteness = "TA Completeness"
	headerScore        = "Score"
	headerToCheck      = "TA To Check"
	headerCommit       = "TA Commit"
	headerImages       = "TA Docker Images"
	headerScreenshots  = "TA Screenshots"
	headerAnalysis     = "TA Analysis"
	headerWritingStyle = "TA Writing Style"
	headerTimeSpent    = "TA Time Spent"
	headerLabFlag      = "TA Lab Flag"
	headerPenalty      = "TA Penalty"
	headerActionItems  = "TA Action Items"
	headerComment      = "TA Comment"
)

// sheetKeys are the headers whose values find a row of the tab.
var sheetKeys = []string{headerTeam, headerLab, headerRule}

// sheetColumns are the columns of mc in the tab Evaluation, by their letters: the keys, the
// formulas, and the TA columns, which the import rewrites on every row but the confirmed ones. The
// others are never written: B and C hold the students of the team, whose formulas a new row copies
// from the row above, and H, J and K are the teacher's: the completeness of the teacher, which the
// "Score" takes when it is filled, the "Comment" and the "Troubleshooting Instructions".
var sheetColumns = map[string]string{
	"A": headerTeam,
	"D": headerLab,
	"E": headerRule,
	"F": headerRuleScore,
	"G": headerCompleteness,
	"I": headerScore,
	"L": headerToCheck,
	"M": headerCommit,
	"N": headerImages,
	"O": headerScreenshots,
	"P": headerAnalysis,
	"Q": headerWritingStyle,
	"R": headerTimeSpent,
	"S": headerLabFlag,
	"T": headerPenalty,
	"U": headerActionItems,
	"V": headerComment,
}

// fillColumns are the letters of the columns of the students, which a new row copies from the row
// above, like a fill down: mc never reads them.
var fillColumns = []string{"B", "C"}

// confirmedColumn is the completeness of the teacher, by its letter. Once the teacher fills it, the
// teacher reviewed the evaluation of the rule and confirmed it: the import never writes the row.
var confirmedColumn = map[string]string{"H": headerConfirmed}

// The formulas of the tab Evaluation, for the row n: sheet.RowPlaceholder in an export. The "Score"
// takes the completeness of the teacher, H, when the teacher filled it, and the one of mc, G,
// otherwise.
func ruleScoreFormula(n string) string {
	return "=VLOOKUP(E" + n + ",Rules!C:F,4,FALSE)"
}

func scoreFormula(n string) string {
	return "=" + completenessFormula(n) + "*F" + n + "+T" + n
}

func labFlagFormula(n string) string {
	sum := "SUMIFS($I:$I,$A:$A,A" + n + ",$D:$D,D" + n + ")"
	return `=IF(` + sum + `<12,"Below 12/20 ("&ROUND(` + sum + `,1)&")","")`
}

func penaltyFormula(remove float64, n string) string {
	return "=-" + formatFloat(remove) + "*" + completenessFormula(n) + "*F" + n
}

// completenessFormula returns the completeness that the "Score" takes: the one of the teacher, else
// the one of mc.
func completenessFormula(n string) string {
	return "IF(ISBLANK(H" + n + "),G" + n + ",H" + n + ")"
}

// EvaluationLayout returns the layout of the tab Evaluation in an export: a tab without rows.
func EvaluationLayout() *sheet.Tab {
	p := sheet.RowPlaceholder
	return &sheet.Tab{
		Name:    SheetTab,
		Keys:    slices.Clone(sheetKeys),
		Columns: maps.Clone(sheetColumns),
		Formulas: map[string]string{
			headerRuleScore: ruleScoreFormula(p),
			headerScore:     scoreFormula(p),
			headerLabFlag:   labFlagFormula(p),
		},
		Fill:      slices.Clone(fillColumns),
		Confirmed: maps.Clone(confirmedColumn),
	}
}

// RuleFacts are the facts of mc for a rule of a team, at the commit of its evaluation.
type RuleFacts struct {
	// Images are the tags of the images of the rule on Docker Hub, with the start of their digests,
	// such as "1.0.0 b80767bd1a2c, 1.0.1 0f3e5d6c7b8a", or "none": "TA Docker Images". Empty when
	// the rule has no image.
	Images string
	// Screenshots is the number of screenshots of the team for the exercise of the rule:
	// "TA Screenshots".
	Screenshots int
}

// ExportOptions are the options of Export.
type ExportOptions struct {
	// Spreadsheet is the ID of the spreadsheet, empty if unknown.
	Spreadsheet string
	// Rules are the rules of the YAML, in their order: the tab Rules.
	Rules []rubric.Rule
	// Labs are the evaluations to export. The rows of the other labs, and of the other teams, stay
	// as they are in the last export.
	Labs []LabEvaluations
	// Last is the last export, nil before the first one: the rules of a pass compare the
	// evaluations with it.
	Last *sheet.Export
}

// LabEvaluations are the evaluations of a lab to export.
type LabEvaluations struct {
	Lab string // The ID of the lab, such as L2.
	// Files are the evaluations of the lab, one per team, checked.
	Files []*File
	// Facts are the facts of mc for the columns that the evaluator doesn't write, by team, then by
	// the ID of the rule. A team or a rule without facts leaves these columns as they are.
	Facts map[string]map[string]RuleFacts
}

// ExportResult is what Export computed.
type ExportResult struct {
	// Export is the new export: the tabs Rules and Evaluation.
	Export *sheet.Export
	// Changes are the cells that change since the last export, tab by tab, in the order of the rows.
	// The cells of a new row are not among them.
	Changes []Change
	// Added are the new rows, which the last export doesn't have, by tab: "teal L2 L2_POD" in the
	// tab Evaluation, "L2_FBP" in the tab Rules.
	Added map[string][]string
	// Removed are the rules of the last export that the YAML no longer has: the import keeps their
	// rows, cleared but for their keys, to delete by hand.
	Removed []string
}

// Change is a cell that changes since the last export.
type Change struct {
	Tab    string
	Row    string // The keys of the row: "teal L2 L2_POD", or "L2_FBP" in the tab Rules.
	Header string
	From   any // The value of the last export, nil when the cell is empty.
	To     any // The value exported, nil when the cell becomes empty.
}

// Lower reports whether the change lowers a "TA Completeness", such as 1 -> 0.7: a pass that gives
// less than the former one, which the teacher checks before the import.
func (c Change) Lower() bool {
	if c.Tab != SheetTab || c.Header != headerCompleteness {
		return false
	}
	from, okFrom := number(c.From)
	to, okTo := number(c.To)
	return okFrom && okTo && to < from-epsilon
}

// Export computes the export of the evaluation sheet: the tab Rules from the YAML, and the tab
// Evaluation from the evaluations, against the last export:
//
//   - A row is found by its team, its lab and its rule. A row that the last export doesn't have is
//     added after the last row.
//   - Every TA column of the row follows the evaluation, and replaces the one of the last export:
//     the "TA Completeness", even when lower, with its "TA Comment", what the team did well and
//     what it can improve. A text that the evaluation leaves empty clears its cell.
//   - "TA Docker Images" and "TA Screenshots" follow the facts of mc, the same way. A team or a
//     rule without facts leaves them as they are.
//   - A penalty becomes the formula of "TA Penalty", and its reason goes to the "TA Comment".
//     Without a penalty, the cell is cleared.
//
// The rows of the labs and the teams that the options don't give stay as they are. The import
// never writes a confirmed row, whose completeness of the teacher is filled.
func Export(opts ExportOptions) (*ExportResult, error) {
	result := &ExportResult{Added: map[string][]string{}}
	var lastRules, lastEvaluation *sheet.Tab
	if opts.Last != nil {
		lastRules, lastEvaluation = opts.Last.Tab(RulesTab), opts.Last.Tab(SheetTab)
	}
	rules, err := exportRules(opts.Rules, lastRules, result)
	if err != nil {
		return nil, err
	}
	evaluation, err := exportEvaluations(opts.Labs, lastEvaluation, result)
	if err != nil {
		return nil, err
	}
	result.Export = &sheet.Export{Spreadsheet: opts.Spreadsheet, Tabs: []*sheet.Tab{rules, evaluation}}
	if problems := result.Export.Problems(); len(problems) > 0 {
		return nil, fmt.Errorf("the export is not valid: %s", strings.Join(problems, "; "))
	}
	return result, nil
}

// exportEvaluations returns the tab Evaluation: the rows of the last export, with the evaluations
// of the labs.
func exportEvaluations(labs []LabEvaluations, last *sheet.Tab, result *ExportResult) (*sheet.Tab, error) {
	tab := EvaluationLayout()
	index := map[string]int{} // The row of each key.
	if last != nil {
		if err := sameLayout(tab, last); err != nil {
			return nil, err
		}
		for _, row := range last.Rows {
			index[rowKey(row)] = len(tab.Rows)
			tab.Rows = append(tab.Rows, maps.Clone(row))
		}
	}
	for _, lab := range labs {
		files := slices.Clone(lab.Files)
		sort.SliceStable(files, func(i, j int) bool { return files[i].Team < files[j].Team })
		for _, f := range files {
			for _, rule := range f.Rules {
				k := key3(f.Team, lab.Lab, rule.ID)
				p := pass{file: f, rule: rule, key: f.Team + " " + lab.Lab + " " + rule.ID, result: result}
				p.facts, p.hasFacts = lab.Facts[f.Team][rule.ID]
				i, ok := index[k]
				if ok {
					p.last = tab.Rows[i]
					p.cells = maps.Clone(p.last)
					if err := p.row(); err != nil {
						return nil, err
					}
					tab.Rows[i] = p.cells
					continue
				}
				p.cells = map[string]any{headerTeam: f.Team, headerLab: lab.Lab, headerRule: rule.ID}
				if err := p.row(); err != nil {
					return nil, err
				}
				index[k] = len(tab.Rows)
				tab.Rows = append(tab.Rows, p.cells)
				result.Added[SheetTab] = append(result.Added[SheetTab], p.key)
			}
		}
	}
	return tab, nil
}

// sameLayout returns an error if the tab of the last export doesn't have the layout of mc. The
// confirmed column is not compared: it says which rows the import writes, not what a row holds.
func sameLayout(want, last *sheet.Tab) error {
	if !slices.Equal(want.Keys, last.Keys) || !maps.Equal(want.Columns, last.Columns) || !maps.Equal(want.Formulas, last.Formulas) ||
		!slices.Equal(want.Fill, last.Fill) {
		return fmt.Errorf("the tab %s of the last export has another layout than the one of mc: migrate it first", last.Name)
	}
	return nil
}

// pass computes the row of a rule of a team from its evaluation.
type pass struct {
	file     *File
	rule     Rule
	facts    RuleFacts // The facts of mc for the rule, when hasFacts.
	hasFacts bool
	key      string         // The keys of the row: teal L2 L2_POD.
	last     map[string]any // The row of the last export, nil for a new row.
	cells    map[string]any // The row exported.
	result   *ExportResult
}

// row computes every TA cell of the row from the evaluation, in place of the last export.
func (p *pass) row() error {
	p.set(headerCompleteness, completenessCell(p.rule.Completeness))
	texts := []struct{ header, value string }{
		{headerComment, p.comment()},
		{headerToCheck, p.rule.ToCheck},
		{headerCommit, short(p.file.Commit)},
		{headerAnalysis, p.rule.Analysis},
		{headerWritingStyle, p.rule.WritingStyle},
		{headerTimeSpent, p.file.TimeSpent},
		{headerActionItems, p.file.ActionItems},
	}
	for _, t := range texts {
		if err := p.setText(t.header, t.value); err != nil {
			return err
		}
	}
	if p.hasFacts {
		if err := p.setText(headerImages, p.facts.Images); err != nil {
			return err
		}
		p.set(headerScreenshots, json.Number(strconv.Itoa(p.facts.Screenshots)))
	}
	if pen := p.rule.Penalty; pen != nil {
		p.set(headerPenalty, penaltyFormula(pen.Remove, sheet.RowPlaceholder))
	} else {
		p.set(headerPenalty, nil)
	}
	return nil
}

// setText sets a text on one line. An empty text clears the cell.
func (p *pass) setText(header, value string) error {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		p.set(header, nil)
		return nil
	}
	if strings.HasPrefix(value, "=") {
		return fmt.Errorf("%s, %q: %q starts with =, which the tab would read as a formula", p.key, header, value)
	}
	p.set(header, value)
	return nil
}

// set sets a cell, or clears it when the value is nil, and records the change when the last export
// had another value.
func (p *pass) set(header string, value any) {
	from := p.last[header]
	if value == nil {
		delete(p.cells, header)
	} else {
		p.cells[header] = value
	}
	if p.last == nil || sameCell(from, value) {
		return
	}
	p.result.Changes = append(p.result.Changes, Change{Tab: SheetTab, Row: p.key, Header: header, From: from, To: value})
}

// comment returns the "TA Comment" of the rule: what the team did well, what it can improve, and
// the reason of the penalty, if any.
func (p *pass) comment() string {
	parts := []string{"Done well: " + sentence(p.rule.DoneWell), "To improve: " + sentence(p.rule.ToImprove)}
	if pen := p.rule.Penalty; pen != nil && !blank(pen.Why) {
		parts = append(parts, "Penalty: "+sentence(pen.Why))
	}
	return strings.Join(parts, " ")
}

// sameCell reports whether two values of a cell are the same: two numbers by their value, and
// texts as they are.
func sameCell(a, b any) bool {
	if isEmpty(a) || isEmpty(b) {
		return isEmpty(a) && isEmpty(b)
	}
	x, okA := a.(json.Number)
	y, okB := b.(json.Number)
	if okA && okB {
		fx, errX := x.Float64()
		fy, errY := y.Float64()
		return errX == nil && errY == nil && equal(fx, fy)
	}
	return text(a) == text(b)
}

func rowKey(row map[string]any) string {
	return key3(text(row[headerTeam]), text(row[headerLab]), text(row[headerRule]))
}

func key3(team, lab, rule string) string {
	return team + "\x1f" + lab + "\x1f" + rule
}

// text returns the value of a cell as a text, empty for an empty cell.
func text(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return v
	case json.Number:
		return v.String()
	}
	return fmt.Sprint(v)
}

func isEmpty(v any) bool {
	return strings.TrimSpace(text(v)) == ""
}

// number returns the number of a cell, false if it is empty or not a number.
func number(v any) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(text(v)), 64)
	return f, err == nil
}

func completenessCell(x float64) json.Number {
	return json.Number(formatFloat(x))
}

// sentence returns s on one line, ending with a period unless it ends with another mark already.
func sentence(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" || strings.HasSuffix(s, ".") || strings.HasSuffix(s, "!") || strings.HasSuffix(s, "?") {
		return s
	}
	return s + "."
}
