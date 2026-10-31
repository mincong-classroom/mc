// Package rubric reads the evaluation rules of a program from a YAML file: the labs of the
// module, what applies to every rule, and the rules keyed by their ID. It checks the file, and
// renders it as a Markdown page.
//
// The file has two maps, for two tasks: "labs" plans the module, one entry per lab session with
// its deadline; "rules" evaluates the labs, one entry per rule, whose field "lab" is a key of
// "labs". The order of the file is kept: the page lists the labs and the rules as the file does.
package rubric

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Rubric is the content of a file of evaluation rules.
type Rubric struct {
	Title          string
	Intro          string
	Labs           []Lab
	Scoring        Scoring
	Evidence       Evidence
	AnalysisLevels []Entry
	Observations   Observations
	Rules          []Rule

	source []byte // the file, for the checks of its lines
	lines  lines  // the lines of the keys, for the problems
}

// Lab is a lab session of the module.
type Lab struct {
	ID        string `yaml:"-"`
	Title     string `yaml:"title"`
	Report    string `yaml:"report"`    // The report in the repository of the team.
	Reference string `yaml:"reference"` // The reference answer, in the repository of the rules.
	Deadline  string `yaml:"deadline"`  // A time with its offset to UTC, e.g. 2026-10-23T08:30:00+02:00.
	Intro     string `yaml:"intro"`
}

// DeadlineTime returns the deadline of the lab.
func (l Lab) DeadlineTime() (time.Time, error) {
	return time.Parse(time.RFC3339, l.Deadline)
}

// Number returns the number of the lab, e.g. 1 for L1.
func (l Lab) Number() string {
	return strings.TrimPrefix(l.ID, "L")
}

// Scoring says how the points are computed: the late penalty, and the penalties of every rule.
type Scoring struct {
	Text        string      `yaml:"text"`
	LatePenalty LatePenalty `yaml:"late_penalty"`
	Penalties   []Penalty   `yaml:"-"`
}

// LatePenalty is the share of the points removed from work pushed after the deadline.
type LatePenalty struct {
	Min float64 `yaml:"min"`
	Max float64 `yaml:"max"`
	How string  `yaml:"how"`
}

// Penalty is a share of the points of a rule, removed when its condition holds.
type Penalty struct {
	Name   string  `yaml:"-"` // The key of a penalty of every rule, empty for the penalty of one rule.
	Remove float64 `yaml:"remove"`
	When   string  `yaml:"when"`
	How    string  `yaml:"how"`
}

// Evidence describes the kinds of evidence, and who produces it.
type Evidence struct {
	Kinds   []Entry
	Origin  string
	Origins []Entry
}

// Entry is a key and its text, in the order of the file.
type Entry struct {
	Key  string
	Text string
}

// Observations are what the evaluator notes outside the score.
type Observations struct {
	Environment Environment `yaml:"environment"`
	Hygiene     string      `yaml:"hygiene"`
}

// Environment is the machine and the cluster of a team.
type Environment struct {
	About   string `yaml:"about"`
	OS      string `yaml:"os"`
	Arch    string `yaml:"arch"`
	Cluster string `yaml:"cluster"`
}

// Rule is an evaluation rule of a lab.
type Rule struct {
	ID                string       `yaml:"-"`
	Lab               string       `yaml:"lab"`
	Name              string       `yaml:"name"`
	Exercise          string       `yaml:"exercise"`    // "4" is Exercise 4, "4.1" its part 4.1.
	Requirement       string       `yaml:"requirement"` // mandatory, or optional for a bonus.
	DependsOn         []string     `yaml:"depends_on"`
	Coefficient       float64      `yaml:"coefficient"`
	CoefficientReason string       `yaml:"coefficient_reason"`
	Description       string       `yaml:"description"`
	Notes             string       `yaml:"notes"`
	Penalties         []Penalty    `yaml:"penalties"`
	Analysis          Analysis     `yaml:"analysis"`
	Evidence          RuleEvidence `yaml:"evidence"`
	Files             Files        `yaml:"files"`
	// Images are the Docker images of the team that the rule evaluates, e.g.
	// "mincongclassroom/spring-petclinic-{team}:1.0.0", where {team} is the name of the team.
	Images []string `yaml:"images"`
}

// Optional reports whether the rule is a bonus, on top of the points of the lab.
func (r Rule) Optional() bool {
	return r.Requirement == RequirementOptional
}

// Points returns the points of the rule out of the 20 of its lab.
func (r Rule) Points() float64 {
	return roundHundredths(r.Coefficient * 20)
}

// The values of Rule.Requirement.
const (
	RequirementMandatory = "mandatory"
	RequirementOptional  = "optional"
)

// Analysis is the explanation that a rule asks for.
type Analysis struct {
	Level string `yaml:"level"` // A key of the analysis levels.
	About string `yaml:"about"`
}

// RuleEvidence is the evidence of a rule: asked by the statement, or extra.
type RuleEvidence struct {
	Asked []EvidenceItem `yaml:"asked"`
	Extra []EvidenceItem `yaml:"extra"`
}

// EvidenceItem is a piece of evidence of a kind, such as "command", and what it shows. In the
// file, it is a mapping of one key: "- command: the command docker build".
type EvidenceItem struct {
	Kind string
	What string
	line int
}

// UnmarshalYAML reads a mapping of one key.
func (e *EvidenceItem) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode || len(node.Content) != 2 {
		return fmt.Errorf("line %d: a piece of evidence is one kind and its text, e.g. \"- file: the manifest\"", node.Line)
	}
	e.Kind = node.Content[0].Value
	e.line = node.Line
	return node.Content[1].Decode(&e.What)
}

// Files are the files of a rule.
type Files struct {
	Team    []string `yaml:"team"`    // In the repository of the team, relative to its root.
	Teacher []string `yaml:"teacher"` // In the repository of the rules, relative to its root.
}

// file is the layout of the YAML, decoded with the known fields only. The mappings keyed by an ID
// or a name are Go maps here: their order comes from the yaml.Node of the file.
type file struct {
	Title          string            `yaml:"title"`
	Intro          string            `yaml:"intro"`
	Labs           map[string]Lab    `yaml:"labs"`
	Scoring        scoringFile       `yaml:"scoring"`
	Evidence       evidenceFile      `yaml:"evidence"`
	AnalysisLevels map[string]string `yaml:"analysis_levels"`
	Observations   Observations      `yaml:"observations"`
	Rules          map[string]Rule   `yaml:"rules"`
}

type scoringFile struct {
	Text        string             `yaml:"text"`
	LatePenalty LatePenalty        `yaml:"late_penalty"`
	Penalties   map[string]Penalty `yaml:"penalties"`
}

type evidenceFile struct {
	Kinds   map[string]string `yaml:"kinds"`
	Origin  string            `yaml:"origin"`
	Origins map[string]string `yaml:"origins"`
}

// Problem is an error of the file.
type Problem struct {
	Line    int    // The line of the file, 0 if unknown.
	Rule    string // The ID of the rule, or of the lab, if any.
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

// Load reads the file of rules. Its error is Problems when the file is valid YAML with unknown
// keys, or a key whose value has the wrong type.
func Load(path string) (*Rubric, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Parse reads the content of a file of rules.
func Parse(data []byte) (*Rubric, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("the file is not a mapping of keys, such as title, labs and rules")
	}
	lines := keyLines(doc.Content[0])

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

	top := doc.Content[0]
	r := &Rubric{
		Title:        f.Title,
		Intro:        f.Intro,
		Observations: f.Observations,
		Scoring: Scoring{
			Text:        f.Scoring.Text,
			LatePenalty: f.Scoring.LatePenalty,
		},
		Evidence: Evidence{Origin: f.Evidence.Origin},
		source:   data,
		lines:    lines,
	}
	for _, id := range keys(top, "labs") {
		lab := f.Labs[id]
		lab.ID = id
		r.Labs = append(r.Labs, lab)
	}
	for _, name := range keys(top, "scoring", "penalties") {
		penalty := f.Scoring.Penalties[name]
		penalty.Name = name
		r.Scoring.Penalties = append(r.Scoring.Penalties, penalty)
	}
	r.Evidence.Kinds = entries(f.Evidence.Kinds, keys(top, "evidence", "kinds"))
	r.Evidence.Origins = entries(f.Evidence.Origins, keys(top, "evidence", "origins"))
	r.AnalysisLevels = entries(f.AnalysisLevels, keys(top, "analysis_levels"))
	for _, id := range keys(top, "rules") {
		rule := f.Rules[id]
		rule.ID = id
		r.Rules = append(r.Rules, rule)
	}
	return r, nil
}

// Lab returns the lab of the ID, e.g. L1, or false if the file has no such lab.
func (r *Rubric) Lab(id string) (Lab, bool) {
	for _, lab := range r.Labs {
		if lab.ID == id {
			return lab, true
		}
	}
	return Lab{}, false
}

// LabRules returns the rules of the lab, in the order of the file.
func (r *Rubric) LabRules(lab string) []Rule {
	var rules []Rule
	for _, rule := range r.Rules {
		if rule.Lab == lab {
			rules = append(rules, rule)
		}
	}
	return rules
}

// LabID returns the ID of a lab given as typed on the command line: "L1", "l1" or "1".
func LabID(arg string) string {
	id := strings.ToUpper(strings.TrimSpace(arg))
	if id != "" && !strings.HasPrefix(id, "L") {
		id = "L" + id
	}
	return id
}

// lines are the lines of the keys of the file, by path: "rules.L1_DKF", "rules.L1_DKF.name".
type lines map[string]int

// keyLines returns the line of each key of the mapping, down to the fields of the labs and the
// rules.
func keyLines(top *yaml.Node) lines {
	result := lines{}
	var walk func(node *yaml.Node, path string, depth int)
	walk = func(node *yaml.Node, path string, depth int) {
		if node.Kind != yaml.MappingNode || depth == 0 {
			return
		}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			p := key.Value
			if path != "" {
				p = path + "." + key.Value
			}
			result[p] = key.Line
			walk(node.Content[i+1], p, depth-1)
		}
	}
	walk(top, "", 4)
	return result
}

// keys returns the keys of the mapping at the path of the document, in the order of the file.
func keys(top *yaml.Node, path ...string) []string {
	node := top
	for _, name := range path {
		node = value(node, name)
		if node == nil {
			return nil
		}
	}
	if node.Kind != yaml.MappingNode {
		return nil
	}
	var result []string
	for i := 0; i+1 < len(node.Content); i += 2 {
		result = append(result, node.Content[i].Value)
	}
	return result
}

// value returns the value of the key in the mapping, or nil.
func value(mapping *yaml.Node, key string) *yaml.Node {
	if mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func entries(texts map[string]string, order []string) []Entry {
	var result []Entry
	for _, key := range order {
		result = append(result, Entry{Key: key, Text: texts[key]})
	}
	return result
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

// ruleAt returns the ID of the rule or of the lab whose entry holds the line, or "": the nearest
// entry or top-level key above the line.
func (l lines) ruleAt(line int) string {
	id, start := "", 0
	for path, at := range l {
		if at > line || at <= start {
			continue
		}
		parts := strings.Split(path, ".")
		switch {
		case len(parts) == 1:
			id, start = "", at
		case len(parts) == 2 && (parts[0] == "rules" || parts[0] == "labs"):
			id, start = parts[1], at
		}
	}
	return id
}

// line returns the line of the key at the path, or of its nearest parent.
func (l lines) line(path string) int {
	for path != "" {
		if at, ok := l[path]; ok {
			return at
		}
		i := strings.LastIndex(path, ".")
		if i < 0 {
			break
		}
		path = path[:i]
	}
	return 0
}

func roundHundredths(x float64) float64 {
	return math.Round(x*100) / 100
}
