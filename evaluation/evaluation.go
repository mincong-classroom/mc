// Package evaluation reads the evaluation of a lab for a team, as the evaluator writes it, and
// checks it against the evaluation rules, before mc turns it into the rows of the evaluation
// sheet.
//
// There is one YAML file per team and lab, so that several evaluators can work at once. The files
// of a lab are in a directory of their own, and each file is named after its team, such as
// ~/.mc/2026/evaluations/lab-2/teal.yaml for the team teal in Lab 2.
package evaluation

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/mincong-classroom/mc/rubric"
)

// File is the evaluation of a lab for a team.
type File struct {
	Team   string
	Commit string // The commit evaluated, in full or short, such as 0a1b2c3.
	// TimeSpent says when the team worked on the lab, such as "Session only, done at 11:21".
	TimeSpent string
	// ActionItems say what the teacher should do with the team, such as "Talk with them", or
	// nothing.
	ActionItems string
	Rules       []Rule // In the order of the file.

	lines lines // The lines of the keys, for the problems.
}

// Rule is the evaluation of a rule of the lab.
type Rule struct {
	ID           string  `yaml:"-"`
	Completeness float64 `yaml:"completeness"` // From 0 to 1, by steps of 0.1.
	DoneWell     string  `yaml:"done_well"`    // What the team did well.
	ToImprove    string  `yaml:"to_improve"`   // What the team can improve.
	ToCheck      string  `yaml:"to_check"`     // What the teacher should check, if anything.
	Analysis     string  `yaml:"analysis"`     // A key of the analysis levels of the rules.
	// WritingStyle says who wrote the answer, such as "own words" or "~30% AI-assisted".
	WritingStyle string   `yaml:"writing_style"`
	Penalty      *Penalty `yaml:"penalty"` // Nil without a penalty.
}

// Penalty is a share of the score of a rule that the evaluator removes.
type Penalty struct {
	// Kind is a penalty of every rule, such as secret, or PenaltyRule, or PenaltyLate.
	Kind   string  `yaml:"kind"`
	Remove float64 `yaml:"remove"` // The share of the score removed, more than 0 and at most 1.
	Why    string  `yaml:"why"`
}

// The kinds of penalties besides those of every rule, which the rules list by name.
const (
	// PenaltyLate is the penalty of work pushed after the deadline: the late penalty of the rules.
	PenaltyLate = "late"
	// PenaltyRule is the penalty of the rule itself, which the rules give without a name.
	PenaltyRule = "rule"
)

// file is the layout of the YAML, decoded with the known fields only. The rules are a Go map here:
// their order comes from the yaml.Node of the file.
type file struct {
	Team        string          `yaml:"team"`
	Commit      string          `yaml:"commit"`
	TimeSpent   string          `yaml:"time_spent"`
	ActionItems string          `yaml:"action_items"`
	Rules       map[string]Rule `yaml:"rules"`
}

// Problem is an error of a file.
type Problem struct {
	Line    int    // The line of the file, 0 if unknown.
	Rule    string // The ID of the rule, if any.
	Message string
}

func (p Problem) String() string {
	var b strings.Builder
	if p.Line > 0 {
		b.WriteString(strconv.Itoa(p.Line))
		b.WriteString(": ")
	}
	if p.Rule != "" {
		b.WriteString(p.Rule)
		b.WriteString(": ")
	}
	b.WriteString(p.Message)
	return b.String()
}

// Problems are the errors of a file that cannot be read.
type Problems []Problem

func (p Problems) Error() string {
	messages := make([]string, len(p))
	for i, problem := range p {
		messages[i] = problem.String()
	}
	return strings.Join(messages, "\n")
}

// ext is the extension of a file, after the name of its team.
const ext = ".yaml"

// LabDir returns the directory of the evaluations of a lab, under the directory of the evaluations
// of the year: lab-2 for L2.
func LabDir(dir string, lab rubric.Lab) string {
	return filepath.Join(dir, "lab-"+lab.Number())
}

// Path returns the file of the team in the directory of the evaluations of a lab.
func Path(labDir, team string) string {
	return filepath.Join(labDir, team+ext)
}

// Teams returns the teams that have a file in the directory of the evaluations of a lab, in the
// order of their names. It returns none when the directory doesn't exist.
func Teams(labDir string) ([]string, error) {
	entries, err := os.ReadDir(labDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var teams []string
	for _, entry := range entries { // Sorted by name.
		team, ok := strings.CutSuffix(entry.Name(), ext)
		if ok && team != "" && !strings.HasPrefix(team, ".") && entry.Type().IsRegular() {
			teams = append(teams, team)
		}
	}
	return teams, nil
}

// Load reads the file of an evaluation. Its error is Problems when the file is not valid YAML, or
// has an unknown key, a key twice, or a value of the wrong type.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parse(data)
}

// parse reads the content of the file of an evaluation.
func parse(data []byte) (*File, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		// The line of a syntax error is left in the message: yaml.v3 gives the line before the
		// error for some of them, such as a flow mapping that isn't closed.
		return nil, Problems{{Message: err.Error()}}
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, Problems{{Line: 1, Message: "the file is not a mapping of keys, such as team, commit and rules"}}
	}
	top := doc.Content[0]
	lines := keyLines(top)

	var f file
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&f); err != nil {
		var typeError *yaml.TypeError
		if !errors.As(err, &typeError) {
			return nil, err
		}
		return nil, decodeProblems(typeError, lines)
	}

	result := &File{
		Team:        f.Team,
		Commit:      f.Commit,
		TimeSpent:   f.TimeSpent,
		ActionItems: f.ActionItems,
		lines:       lines,
	}
	for _, id := range ruleKeys(top) {
		rule := f.Rules[id]
		rule.ID = id
		result.Rules = append(result.Rules, rule)
	}
	return result, nil
}

// lines are the keys of the file, by path: "rules.L2_POD", "rules.L2_POD.penalty.kind".
type lines map[string]key

// key is a key of the file.
type key struct {
	line int
	null bool // Whether its value is null, such as "completeness:" without a number.
}

// keyLines returns the line of each key of the mapping, down to the fields of the penalties.
func keyLines(top *yaml.Node) lines {
	result := lines{}
	var walk func(node *yaml.Node, path string, depth int)
	walk = func(node *yaml.Node, path string, depth int) {
		if node.Kind != yaml.MappingNode || depth == 0 {
			return
		}
		for i := 0; i+1 < len(node.Content); i += 2 {
			k, v := node.Content[i], node.Content[i+1]
			p := k.Value
			if path != "" {
				p = path + "." + k.Value
			}
			result[p] = key{line: k.Line, null: v.Kind == yaml.ScalarNode && v.Tag == "!!null"}
			walk(v, p, depth-1)
		}
	}
	walk(top, "", 4)
	return result
}

// line returns the line of the key at the path, or of its nearest parent, or 0.
func (l lines) line(path string) int {
	for path != "" {
		if k, ok := l[path]; ok {
			return k.line
		}
		i := strings.LastIndex(path, ".")
		if i < 0 {
			break
		}
		path = path[:i]
	}
	return 0
}

// has reports whether the file has the key at the path, with a value other than null.
func (l lines) has(path string) bool {
	k, ok := l[path]
	return ok && !k.null
}

// ruleAt returns the ID of the rule whose entry holds the line, or "": the nearest rule or
// top-level key above the line.
func (l lines) ruleAt(line int) string {
	id, start := "", 0
	for path, k := range l {
		at := k.line
		if at > line || at <= start {
			continue
		}
		parts := strings.Split(path, ".")
		switch {
		case len(parts) == 1:
			id, start = "", at
		case len(parts) == 2 && parts[0] == "rules":
			id, start = parts[1], at
		}
	}
	return id
}

// ruleKeys returns the IDs of the rules, in the order of the file.
func ruleKeys(top *yaml.Node) []string {
	var rules *yaml.Node
	for i := 0; i+1 < len(top.Content); i += 2 {
		if top.Content[i].Value == "rules" {
			rules = top.Content[i+1]
		}
	}
	if rules == nil || rules.Kind != yaml.MappingNode {
		return nil
	}
	var ids []string
	for i := 0; i+1 < len(rules.Content); i += 2 {
		ids = append(ids, rules.Content[i].Value)
	}
	return ids
}

var (
	unknownFieldPattern = regexp.MustCompile(`^line (\d+): field (\S+) not found in type \S+$`)
	linePattern         = regexp.MustCompile(`^line (\d+): (.*)$`)
)

// decodeProblems turns the errors of the decoder into problems, with the rule of their line.
func decodeProblems(err *yaml.TypeError, lines lines) Problems {
	var problems Problems
	for _, message := range err.Errors {
		if m := unknownFieldPattern.FindStringSubmatch(message); m != nil {
			line, _ := strconv.Atoi(m[1]) // The pattern matches digits only.
			problems = append(problems, Problem{Line: line, Rule: lines.ruleAt(line), Message: fmt.Sprintf("unknown key %q", m[2])})
			continue
		}
		if m := linePattern.FindStringSubmatch(message); m != nil {
			line, _ := strconv.Atoi(m[1]) // The pattern matches digits only.
			problems = append(problems, Problem{Line: line, Rule: lines.ruleAt(line), Message: m[2]})
			continue
		}
		problems = append(problems, Problem{Message: message})
	}
	return problems
}
