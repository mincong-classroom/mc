package evaluation

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mincong-classroom/mc/rubric"
)

// valid is a valid evaluation of the made-up Lab 2 of rubric/testdata, which each case of
// TestCheck breaks in one place.
const valid = `team: teal
commit: 0a1b2c3
time_spent: Session only, done at 11:21
action_items: Run it yourself
rules:
  L2_POD:
    completeness: 0.8
    done_well: The Pod runs the image of Lab 1.
    to_improve: The report doesn't show the logs of the Pod.
    to_check: ""
    analysis: none
    writing_style: own words
    penalty: null
  L2_LOG:
    completeness: 1
    done_well: The logs show the requests of the browser.
    to_improve: The report could say which command follows them.
    analysis: short
    writing_style: ~30% AI-assisted
    penalty: {kind: late, remove: 0.1, why: Pushed after the deadline.}
`

// factsCommit is the commit of the facts of the team teal, which the commit of valid starts with.
const factsCommit = "0a1b2c3d4e5f60718293a4b5c6d7e8f901234567"

// loadRubric returns the made-up rules of the tests of the package rubric.
func loadRubric(t *testing.T) *rubric.Rubric {
	t.Helper()
	r, err := rubric.Load("../rubric/testdata/evaluation-rules.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestCheck(t *testing.T) {
	r := loadRubric(t)
	for _, c := range []struct {
		name     string
		old, new string // The change of the file; old is empty for the valid file.
		noFacts  bool   // Whether mc eval gather-facts has gathered no facts for the team.
		want     []string
	}{
		{name: "valid"},
		{
			name: "an unknown key of a rule",
			old:  `    to_check: ""`,
			new:  `    to_chek: ""`,
			want: []string{`10: L2_POD: unknown key "to_chek"`},
		},
		{
			name: "an unknown key of the file",
			old:  "time_spent:",
			new:  "time_spend:",
			want: []string{`3: unknown key "time_spend"`},
		},
		{
			name: "a rule twice",
			old:  "  L2_LOG:",
			new:  "  L2_POD:",
			want: []string{`14: L2_POD: mapping key "L2_POD" already defined at line 6`},
		},
		{
			name: "a completeness that is not a number",
			old:  "completeness: 0.8",
			new:  "completeness: high",
			want: []string{"7: L2_POD: cannot unmarshal !!str `high` into float64"},
		},
		{
			name: "invalid YAML",
			old:  "    penalty: {kind: late, remove: 0.1, why: Pushed after the deadline.}",
			new:  "    penalty: {kind: late, remove: 0.1",
			want: []string{"yaml: line 19: did not find expected ',' or '}'"},
		},
		{
			name: "the team of another file",
			old:  "team: teal",
			new:  "team: coral",
			want: []string{`1: the team "coral" is not the one of the name of the file, teal`},
		},
		{
			name: "no team",
			old:  "team: teal\n",
			want: []string{"no team: the name of the file, teal"},
		},
		{
			name: "no commit",
			old:  "commit: 0a1b2c3\n",
			want: []string{"no commit: the commit evaluated, the one of the facts"},
		},
		{
			name: "a commit too short",
			old:  "commit: 0a1b2c3",
			new:  "commit: 0a1b",
			want: []string{`2: the commit "0a1b" is not a hash of at least 7 hexadecimal digits, such as 0a1b2c3`},
		},
		{
			name: "the full commit",
			old:  "commit: 0a1b2c3",
			new:  "commit: " + factsCommit,
		},
		{
			name: "another commit than the one of the facts",
			old:  "commit: 0a1b2c3",
			new:  "commit: 9f8e7d6",
			want: []string{"2: the commit 9f8e7d6 is not the one of the facts, 0a1b2c3: evaluate that commit, or gather the facts again"},
		},
		{
			name:    "no facts",
			noFacts: true,
			want:    []string{"2: no facts for the team teal in L2: run mc eval gather-facts -l L2 -t teal first"},
		},
		{
			name: "unknown action items",
			old:  "action_items: Run it yourself",
			new:  "action_items: Run it",
			want: []string{`4: the action items "Run it" are not one of "Run it yourself", "Talk with them", "Less attention" or "Late work to decide"`},
		},
		{
			name: "no action items",
			old:  "action_items: Run it yourself\n",
		},
		{
			name: "no mandatory rule",
			old:  valid[strings.Index(valid, "  L2_POD:"):strings.Index(valid, "  L2_LOG:")],
			want: []string{"5: L2_POD: no evaluation of this rule, which is mandatory"},
		},
		{
			name: "no optional rule",
			old:  valid[strings.Index(valid, "  L2_LOG:"):],
		},
		{
			name: "a rule of another lab",
			old:  "  L2_LOG:",
			new:  "  L1_TAG:",
			want: []string{"14: L1_TAG: a rule of L1, not of L2"},
		},
		{
			name: "an unknown rule",
			old:  "  L2_LOG:",
			new:  "  L2_XYZ:",
			want: []string{"14: L2_XYZ: not a rule of the evaluation rules"},
		},
		{
			name: "no completeness",
			old:  "    completeness: 0.8\n",
			want: []string{"6: L2_POD: no completeness: from 0 to 1, by steps of 0.1"},
		},
		{
			name: "a completeness without a value",
			old:  "completeness: 0.8",
			new:  "completeness:",
			want: []string{"7: L2_POD: no completeness: from 0 to 1, by steps of 0.1"},
		},
		{
			name: "a completeness of 0",
			old:  "completeness: 0.8",
			new:  "completeness: 0",
		},
		{
			name: "a completeness between 2 steps",
			old:  "completeness: 0.8",
			new:  "completeness: 0.85",
			want: []string{"7: L2_POD: the completeness 0.85 is not from 0 to 1 by steps of 0.1"},
		},
		{
			name: "a completeness above 1",
			old:  "completeness: 1\n",
			new:  "completeness: 1.2\n",
			want: []string{"15: L2_LOG: the completeness 1.2 is not from 0 to 1 by steps of 0.1"},
		},
		{
			name: "nothing done well",
			old:  "done_well: The Pod runs the image of Lab 1.",
			new:  `done_well: ""`,
			want: []string{"8: L2_POD: no done_well: what the team did well, which the comment of the sheet starts with"},
		},
		{
			name: "nothing to improve",
			old:  "    to_improve: The report doesn't show the logs of the Pod.\n",
			want: []string{"6: L2_POD: no to_improve: what the team can improve, which the comment of the sheet ends with"},
		},
		{
			name: "an unknown analysis level",
			old:  "analysis: none",
			new:  "analysis: detailed",
			want: []string{`11: L2_POD: the analysis "detailed" is not one of the analysis levels, "none" or "short"`},
		},
		{
			name: "no analysis",
			old:  "    analysis: none\n",
			want: []string{`6: L2_POD: no analysis: one of "none" or "short"`},
		},
		{
			name: "no writing style",
			old:  "writing_style: own words",
			new:  "writing_style: ' '",
			want: []string{`12: L2_POD: no writing_style, such as "own words" or "~30% AI-assisted"`},
		},
		{
			name: "the penalty of every rule",
			old:  "penalty: null",
			new:  "penalty: {kind: secret, remove: 0.3, why: The report shows a token.}",
		},
		{
			name: "the penalty of every rule, with another share",
			old:  "penalty: null",
			new:  "penalty: {kind: secret, remove: 0.2, why: The report shows a token.}",
			want: []string{"13: L2_POD: the penalty secret removes 0.3 of the score: got 0.2"},
		},
		{
			name: "an unknown penalty",
			old:  "penalty: null",
			new:  "penalty: {kind: language, remove: 0.2, why: The answer is in French.}",
			want: []string{`13: L2_POD: the penalty "language" is not one of the penalties of the rule, "secret" or "late"`},
		},
		{
			name: "the penalty of a rule without one",
			old:  "penalty: null",
			new:  "penalty: {kind: rule, remove: 0.2, why: The manifest is only in the report.}",
			want: []string{`13: L2_POD: the penalty "rule" is not one of the penalties of the rule, "secret" or "late"`},
		},
		{
			name: "no kind of penalty",
			old:  "{kind: late, remove: 0.1,",
			new:  "{remove: 0.1,",
			want: []string{`20: L2_LOG: no kind of penalty: one of "secret" or "late"`},
		},
		{
			name: "a late penalty out of its range",
			old:  "remove: 0.1,",
			new:  "remove: 0.3,",
			want: []string{"20: L2_LOG: the late penalty removes from 0.1 to 0.2 of the score: got 0.3"},
		},
		{
			name: "a penalty that removes more than the score",
			old:  "remove: 0.1,",
			new:  "remove: 1.5,",
			want: []string{"20: L2_LOG: a penalty removes more than 0 and at most 1 of the score: got 1.5"},
		},
		{
			name: "a penalty without a reason",
			old:  "why: Pushed after the deadline.",
			new:  `why: ""`,
			want: []string{"20: L2_LOG: no why: the reason of the penalty, for the comment of the sheet"},
		},
		{
			name: "an unknown key of a penalty",
			old:  "why: Pushed after the deadline.",
			new:  "reason: Pushed after the deadline.",
			want: []string{`20: L2_LOG: unknown key "reason"`},
		},
		{
			name: "several problems, in the order of their lines",
			old:  "commit: 0a1b2c3\ntime_spent: Session only, done at 11:21\naction_items: Run it yourself\n",
			new:  "commit: 9f8e7d6\ntime_spent: Session only, done at 11:21\naction_items: Later\n",
			want: []string{
				"2: the commit 9f8e7d6 is not the one of the facts, 0a1b2c3: evaluate that commit, or gather the facts again",
				`4: the action items "Later" are not one of "Run it yourself", "Talk with them", "Less attention" or "Late work to decide"`,
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			content := valid
			if c.old != "" {
				if !strings.Contains(valid, c.old) {
					t.Fatalf("the valid file has no %q", c.old)
				}
				content = strings.Replace(valid, c.old, c.new, 1)
			}
			opts := CheckOptions{Rubric: r, Lab: "L2", Team: "teal", Commit: factsCommit}
			if c.noFacts {
				opts.Commit = ""
			}
			if got := problems(t, content, opts); !slices.Equal(got, c.want) {
				t.Errorf("the problems of the file are\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(c.want, "\n"))
			}
		})
	}
}

// TestCheckThePenaltyOfTheRule checks the penalty of the rule itself, which the rules give without
// a name: the made-up L1_BLD removes 0.2 of the score when the Dockerfile is only in the report.
func TestCheckThePenaltyOfTheRule(t *testing.T) {
	r := loadRubric(t)
	const file = `team: coral
commit: 2e88112
rules:
  L1_BLD:
    completeness: 0.9
    done_well: The image builds from a base image of Java 21.
    to_improve: The Dockerfile is only in the report.
    analysis: none
    writing_style: own words
    penalty: {kind: rule, remove: 0.2, why: The Dockerfile is only in the report.}
  L1_TAG:
    completeness: 1
    done_well: The image 1.0.0 is on Docker Hub.
    to_improve: The report could say why a tag is immutable.
    analysis: short
    writing_style: own words
`
	for _, c := range []struct {
		name     string
		old, new string
		want     []string
	}{
		{name: "its share"},
		{
			name: "another share",
			old:  "remove: 0.2",
			new:  "remove: 0.3",
			want: []string{"10: L1_BLD: the penalty of the rule removes 0.2 of the score: got 0.3"},
		},
		{
			name: "a penalty of every rule",
			old:  "kind: rule",
			new:  "kind: secret",
			want: []string{"10: L1_BLD: the penalty secret removes 0.3 of the score: got 0.2"},
		},
		{
			name: "an unknown penalty",
			old:  "kind: rule",
			new:  "kind: language",
			want: []string{`10: L1_BLD: the penalty "language" is not one of the penalties of the rule, "secret", "rule" or "late"`},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			content := strings.Replace(file, c.old, c.new, 1)
			// The commit 2e88112 reads as a number in YAML 1.1, and in a spreadsheet: it stays a text.
			opts := CheckOptions{Rubric: r, Lab: "L1", Team: "coral", Commit: "2e88112c94e0a9a1e0a7a54d0a4f0e5a4e8d2f71"}
			if got := problems(t, content, opts); !slices.Equal(got, c.want) {
				t.Errorf("the problems of the file are\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(c.want, "\n"))
			}
		})
	}
}

// problems returns the problems of the content, from the decoder or from Check.
func problems(t *testing.T, content string, opts CheckOptions) []string {
	t.Helper()
	f, err := parse([]byte(content))
	var decoded Problems
	if errors.As(err, &decoded) {
		return texts(decoded)
	}
	if err != nil {
		t.Fatal(err)
	}
	return texts(f.Check(opts))
}

// texts returns the problems as their String writes them, such as "7: L2_POD: ...".
func texts(problems []Problem) []string {
	var result []string
	for _, p := range problems {
		result = append(result, p.String())
	}
	return result
}
