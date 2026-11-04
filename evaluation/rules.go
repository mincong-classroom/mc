package evaluation

import (
	"encoding/json"
	"fmt"
	"maps"
	"strconv"
	"strings"

	"github.com/mincong-classroom/mc/rubric"
	"github.com/mincong-classroom/mc/sheet"
)

// RulesTab is the tab of the evaluation sheet that holds a copy of the rules of the YAML, where
// the tab Evaluation looks up the score of each rule.
const RulesTab = "Rules"

// The headers of the columns of the tab Rules, from A to I, as the README of the package describes
// them: one row per rule of the YAML. The examples are the rules L1_BLD and L2_LOG of the made-up
// file rubric/testdata/evaluation-rules.yaml.
const (
	// headerLabSession is the lab of the rule, the key lab of the YAML. With the "Rule", it finds
	// the row of a rule.
	//
	// column: A
	// examples: L1, L2
	headerLabSession = "Lab Session"

	// headerRuleEnd is the end of the "Rule ID", after the lab and the underscore. With the
	// "Lab Session", it finds the row of a rule.
	//
	// column: B
	// examples: BLD, LOG
	headerRuleEnd = "Rule"

	// headerRuleID is the ID of the rule, a formula that joins A and B,
	// =CONCAT(A2, CONCAT("_", B2)). The "Rule Score" of the tab Evaluation looks it up.
	//
	// column: C
	// examples: L1_BLD, L2_LOG
	headerRuleID = "Rule ID"

	// headerRuleName is the name of the rule, the key name of the YAML.
	//
	// column: D
	// examples: Build Test, Logs Test
	headerRuleName = "Rule Name"

	// headerCoefficient is the share of the 20 points of the lab that the rule gives, the key
	// coefficient of the YAML. A bonus, such as L2_LOG, counts on top of the 20 points.
	//
	// column: E
	// examples: 0.6, 0.1
	headerCoefficient = "Coefficient"

	// headerRulePoints is the points of the rule, a formula, =20*E2.
	//
	// column: F
	// examples: 12, 2
	headerRulePoints = "Score"

	// headerExercise is the exercise of the rule, the key exercise of the YAML, as a number.
	//
	// column: G
	// examples: 1, 2.1 for the part 2.1 of Exercise 2
	headerExercise = "Exercise"

	// headerExerciseID is the ID of the exercise, a formula that joins A and G,
	// =CONCAT(CONCAT(A2, "_"), CONCAT("EX", G2)).
	//
	// column: H
	// examples: L1_EX1, L2_EX2.1
	headerExerciseID = "Exercise ID"

	// headerDescription is what the team is expected to deliver, the key description of the YAML,
	// on one line.
	//
	// column: I
	// example: The team is expected to follow the logs of the Pod. This rule is a bonus.
	headerDescription = "Rule Description"
)

// rulesColumns are the columns of the tab Rules, by their letters: all of them are mc's, from the
// YAML. Its column J, "Troubleshooting Instructions", is the teacher's: mc never writes it.
var rulesColumns = map[string]string{
	"A": headerLabSession,
	"B": headerRuleEnd,
	"C": headerRuleID,
	"D": headerRuleName,
	"E": headerCoefficient,
	"F": headerRulePoints,
	"G": headerExercise,
	"H": headerExerciseID,
	"I": headerDescription,
}

// RulesLayout returns the layout of the tab Rules in an export: a tab without rows.
func RulesLayout() *sheet.Tab {
	p := sheet.RowPlaceholder
	return &sheet.Tab{
		Name:    RulesTab,
		Keys:    []string{headerLabSession, headerRuleEnd},
		Columns: maps.Clone(rulesColumns),
		Formulas: map[string]string{
			headerRuleID:     `=CONCAT(A` + p + `, CONCAT("_", B` + p + `))`,
			headerRulePoints: "=20*E" + p,
			headerExerciseID: `=CONCAT(CONCAT(A` + p + `, "_"), CONCAT("EX", G` + p + `))`,
		},
		Fill: []string{},
	}
}

// exportRules returns the tab Rules: one row per rule of the YAML, in its order. It records the
// cells that change since the last export, the new rules, and the rules that the YAML no longer
// has.
func exportRules(rules []rubric.Rule, last *sheet.Tab, result *ExportResult) (*sheet.Tab, error) {
	tab := RulesLayout()
	former := map[string]map[string]any{}
	if last != nil {
		if err := sameLayout(tab, last); err != nil {
			return nil, err
		}
		for _, row := range last.Rows {
			former[ruleKey(text(row[headerLabSession]), text(row[headerRuleEnd]))] = row
		}
	}
	exported := map[string]bool{}
	for _, rule := range rules {
		lab, end, ok := strings.Cut(rule.ID, "_")
		if !ok || lab != rule.Lab || end == "" {
			return nil, fmt.Errorf("the rule %s: its ID is not its lab %s, an underscore, then its letters", rule.ID, rule.Lab)
		}
		cells, err := ruleCells(rule)
		if err != nil {
			return nil, err
		}
		row := map[string]any{headerLabSession: lab, headerRuleEnd: end}
		for _, c := range cells {
			row[c.header] = c.value
		}
		k := ruleKey(lab, end)
		exported[k] = true
		tab.Rows = append(tab.Rows, row)
		before, ok := former[k]
		if !ok {
			if last != nil {
				result.Added[RulesTab] = append(result.Added[RulesTab], rule.ID)
			}
			continue
		}
		for _, letter := range tab.Letters() {
			header := tab.Columns[letter]
			if _, formula := tab.Formulas[header]; formula || sameCell(before[header], row[header]) {
				continue
			}
			result.Changes = append(result.Changes, Change{Tab: RulesTab, Row: rule.ID, Header: header, From: before[header], To: row[header]})
		}
	}
	if last != nil {
		for _, row := range last.Rows {
			lab, end := text(row[headerLabSession]), text(row[headerRuleEnd])
			if !exported[ruleKey(lab, end)] {
				result.Removed = append(result.Removed, lab+"_"+end)
			}
		}
	}
	return tab, nil
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
			return nil, fmt.Errorf("the rule %s, %q: %s starts with =, which the tab would read as a formula", rule.ID, c.header, brief(s))
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

func ruleKey(lab, end string) string {
	return lab + "\x1f" + end
}

// oneLine returns a text on one line, its spaces and line breaks squeezed to one space.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// brief returns the value of a cell for a message, quoted, and cut after 40 characters.
func brief(v any) string {
	if isEmpty(v) {
		return "nothing"
	}
	s := []rune(text(v))
	if len(s) > 40 {
		return strconv.Quote(string(s[:40]) + "...")
	}
	return strconv.Quote(string(s))
}
