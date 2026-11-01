package evaluation

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/mincong-classroom/mc/rubric"
)

// CheckOptions are the options of Check.
type CheckOptions struct {
	Rubric *rubric.Rubric
	Lab    string // The ID of the lab of the file, such as L2.
	Team   string // The team of the file, from its name: teal for teal.yaml.
	// Commit is the commit of the facts that mc evidence gathered for the team and the lab, in
	// full. When empty, there are no facts: the check says to gather them first.
	Commit string
}

// actionItems are the values of the action items: what the teacher should do with the team.
var actionItems = []string{"Run it yourself", "Talk with them", "Less attention", "Late work to decide"}

var commitPattern = regexp.MustCompile(`^[0-9a-f]{7,64}$`)

// Check returns the problems of the evaluation, in the order of their lines.
func (f *File) Check(opts CheckOptions) []Problem {
	c := checker{file: f, opts: opts}
	c.checkTeam()
	c.checkCommit()
	c.checkActionItems()
	c.checkRules()
	sort.SliceStable(c.problems, func(i, j int) bool { return c.problems[i].Line < c.problems[j].Line })
	return c.problems
}

type checker struct {
	file     *File
	opts     CheckOptions
	problems []Problem
}

func (c *checker) add(path, id, format string, args ...any) {
	c.problems = append(c.problems, Problem{
		Line:    c.file.lines.line(path),
		Rule:    id,
		Message: fmt.Sprintf(format, args...),
	})
}

func (c *checker) checkTeam() {
	switch c.file.Team {
	case "":
		c.add("team", "", "no team: the name of the file, %s", c.opts.Team)
	case c.opts.Team:
	default:
		c.add("team", "", "the team %q is not the one of the name of the file, %s", c.file.Team, c.opts.Team)
	}
}

func (c *checker) checkCommit() {
	commit := c.file.Commit
	switch {
	case commit == "":
		c.add("commit", "", "no commit: the commit evaluated, the one of the facts of mc evidence")
	case !commitPattern.MatchString(commit):
		c.add("commit", "", "the commit %q is not a hash of at least 7 hexadecimal digits, such as 0a1b2c3", commit)
	}
	switch {
	case c.opts.Commit == "":
		c.add("commit", "", "no facts of mc evidence for the team %s in %s: run mc evidence -l %s -t %s first",
			c.opts.Team, c.opts.Lab, c.opts.Lab, c.opts.Team)
	case commitPattern.MatchString(commit) && !strings.HasPrefix(c.opts.Commit, commit):
		c.add("commit", "", "the commit %s is not the one of the facts of mc evidence, %s: evaluate that commit, or gather the facts again",
			commit, short(c.opts.Commit))
	}
}

func (c *checker) checkActionItems() {
	if items := c.file.ActionItems; items != "" && !slices.Contains(actionItems, items) {
		c.add("action_items", "", "the action items %q are not one of %s", items, quoteAll(actionItems))
	}
}

func (c *checker) checkRules() {
	rules := map[string]rubric.Rule{}
	for _, rule := range c.opts.Rubric.Rules {
		rules[rule.ID] = rule
	}
	evaluated := map[string]bool{}
	for _, rule := range c.file.Rules {
		evaluated[rule.ID] = true
		at := "rules." + rule.ID
		spec, ok := rules[rule.ID]
		switch {
		case !ok:
			c.add(at, rule.ID, "not a rule of the evaluation rules")
		case spec.Lab != c.opts.Lab:
			c.add(at, rule.ID, "a rule of %s, not of %s", spec.Lab, c.opts.Lab)
		default:
			c.checkRule(rule, spec)
		}
	}
	for _, spec := range c.opts.Rubric.LabRules(c.opts.Lab) {
		if !spec.Optional() && !evaluated[spec.ID] {
			c.add("rules", spec.ID, "no evaluation of this rule, which is mandatory")
		}
	}
}

func (c *checker) checkRule(rule Rule, spec rubric.Rule) {
	at := "rules." + rule.ID
	if !c.file.lines.has(at + ".completeness") {
		c.add(at+".completeness", rule.ID, "no completeness: from 0 to 1, by steps of 0.1")
	} else if !isCompleteness(rule.Completeness) {
		c.add(at+".completeness", rule.ID, "the completeness %v is not from 0 to 1 by steps of 0.1", rule.Completeness)
	}
	if blank(rule.DoneWell) {
		c.add(at+".done_well", rule.ID, "no done_well: what the team did well, which the comment of the sheet starts with")
	}
	if blank(rule.ToImprove) {
		c.add(at+".to_improve", rule.ID, "no to_improve: what the team can improve, which the comment of the sheet ends with")
	}
	levels := c.analysisLevels()
	switch {
	case rule.Analysis == "":
		c.add(at+".analysis", rule.ID, "no analysis: one of %s", quoteAll(levels))
	case !slices.Contains(levels, rule.Analysis):
		c.add(at+".analysis", rule.ID, "the analysis %q is not one of the analysis levels, %s", rule.Analysis, quoteAll(levels))
	}
	if blank(rule.WritingStyle) {
		c.add(at+".writing_style", rule.ID, `no writing_style, such as "own words" or "~30%% AI-assisted"`)
	}
	if rule.Penalty != nil {
		c.checkPenalty(at+".penalty", rule, spec)
	}
}

// checkPenalty checks the penalty of the rule: a kind of the rules, and the share that the rules
// give for that kind.
func (c *checker) checkPenalty(at string, rule Rule, spec rubric.Rule) {
	penalty := *rule.Penalty
	kinds := c.penaltyKinds(spec)
	if blank(penalty.Why) {
		c.add(at+".why", rule.ID, "no why: the reason of the penalty, for the comment of the sheet")
	}
	if penalty.Kind == "" {
		c.add(at+".kind", rule.ID, "no kind of penalty: one of %s", quoteAll(kinds))
		return
	}
	if !slices.Contains(kinds, penalty.Kind) {
		c.add(at+".kind", rule.ID, "the penalty %q is not one of the penalties of the rule, %s", penalty.Kind, quoteAll(kinds))
		return
	}
	if penalty.Remove <= 0 || penalty.Remove > 1 {
		c.add(at+".remove", rule.ID, "a penalty removes more than 0 and at most 1 of the score: got %v", penalty.Remove)
		return
	}
	switch penalty.Kind {
	case PenaltyLate:
		late := c.opts.Rubric.Scoring.LatePenalty
		if penalty.Remove < late.Min-epsilon || penalty.Remove > late.Max+epsilon {
			c.add(at+".remove", rule.ID, "the late penalty removes from %v to %v of the score: got %v", late.Min, late.Max, penalty.Remove)
		}
	case PenaltyRule:
		var shares []string
		for _, p := range spec.Penalties {
			if equal(p.Remove, penalty.Remove) {
				return
			}
			shares = append(shares, formatFloat(p.Remove))
		}
		c.add(at+".remove", rule.ID, "the penalty of the rule removes %s of the score: got %v", strings.Join(shares, " or "), penalty.Remove)
	default:
		for _, p := range c.opts.Rubric.Scoring.Penalties {
			if p.Name == penalty.Kind && !equal(p.Remove, penalty.Remove) {
				c.add(at+".remove", rule.ID, "the penalty %s removes %v of the score: got %v", p.Name, p.Remove, penalty.Remove)
			}
		}
	}
}

// penaltyKinds returns the kinds of penalties of a rule: those of every rule, its own if any, and
// the late penalty.
func (c *checker) penaltyKinds(spec rubric.Rule) []string {
	var kinds []string
	for _, p := range c.opts.Rubric.Scoring.Penalties {
		kinds = append(kinds, p.Name)
	}
	if len(spec.Penalties) > 0 {
		kinds = append(kinds, PenaltyRule)
	}
	return append(kinds, PenaltyLate)
}

func (c *checker) analysisLevels() []string {
	levels := make([]string, len(c.opts.Rubric.AnalysisLevels))
	for i, level := range c.opts.Rubric.AnalysisLevels {
		levels[i] = level.Key
	}
	return levels
}

const epsilon = 1e-9

// isCompleteness reports whether x is from 0 to 1, by steps of 0.1.
func isCompleteness(x float64) bool {
	steps := x * 10
	return x >= 0 && x <= 1 && math.Abs(steps-math.Round(steps)) < epsilon
}

func equal(x, y float64) bool {
	return math.Abs(x-y) < epsilon
}

func formatFloat(x float64) string {
	return strconv.FormatFloat(x, 'g', -1, 64)
}

func blank(s string) bool {
	return strings.TrimSpace(s) == ""
}

// short returns the short form of a commit, as git prints it.
func short(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

// quoteAll returns the values quoted, as a list in a sentence: "a", "b" or "c".
func quoteAll(values []string) string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = strconv.Quote(v)
	}
	if len(quoted) < 2 {
		return strings.Join(quoted, "")
	}
	return strings.Join(quoted[:len(quoted)-1], ", ") + " or " + quoted[len(quoted)-1]
}
