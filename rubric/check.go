package rubric

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// CheckOptions are the options of Check.
type CheckOptions struct {
	// Root is the directory of the repository of the rules, against which the reference of each
	// lab and the files of the teacher are checked. When empty, they are not checked.
	Root string
	// MaxWidth is the maximum number of characters of a line of the file, 0 for no limit.
	MaxWidth int
}

var (
	labIDPattern    = regexp.MustCompile(`^L\d+$`)
	ruleIDPattern   = regexp.MustCompile(`^L\d+_[A-Z]{3}$`)
	exercisePattern = regexp.MustCompile(`^\d+(\.\d+){0,2}$`)
	imagePattern    = regexp.MustCompile(`^[a-z0-9{}._/-]+:[A-Za-z0-9._-]+$`)
)

// Check returns the problems of the rules, in the order of their lines.
func (r *Rubric) Check(opts CheckOptions) []Problem {
	c := checker{rubric: r, opts: opts}
	c.checkWidth()
	c.checkLabs()
	c.checkScoring()
	c.checkRules()
	sort.SliceStable(c.problems, func(i, j int) bool { return c.problems[i].Line < c.problems[j].Line })
	return c.problems
}

type checker struct {
	rubric   *Rubric
	opts     CheckOptions
	problems []Problem
}

func (c *checker) add(path, id, format string, args ...any) {
	c.problems = append(c.problems, Problem{
		Line:    c.rubric.lines.line(path),
		Rule:    id,
		Message: fmt.Sprintf(format, args...),
	})
}

func (c *checker) checkWidth() {
	if c.opts.MaxWidth <= 0 {
		return
	}
	scanner := bufio.NewScanner(bytes.NewReader(c.rubric.source))
	for n := 1; scanner.Scan(); n++ {
		if width := utf8.RuneCountInString(scanner.Text()); width > c.opts.MaxWidth {
			c.problems = append(c.problems, Problem{
				Line:    n,
				Message: fmt.Sprintf("the line has %d characters, more than %d", width, c.opts.MaxWidth),
			})
		}
	}
}

func (c *checker) checkLabs() {
	if len(c.rubric.Labs) == 0 {
		c.add("labs", "", "no lab")
	}
	for _, lab := range c.rubric.Labs {
		at := "labs." + lab.ID
		if !labIDPattern.MatchString(lab.ID) {
			c.add(at, lab.ID, "the ID of a lab is L and its number, such as L1")
		}
		if lab.Title == "" {
			c.add(at, lab.ID, "no title")
		}
		if _, err := lab.DeadlineTime(); err != nil {
			c.add(at+".deadline", lab.ID, "the deadline %q is not a time with its offset to UTC, such as 2026-10-23T08:30:00+02:00", lab.Deadline)
		}
		if lab.Report == "" {
			c.add(at, lab.ID, "no report")
		} else {
			c.checkTeamPath(at+".report", lab.ID, lab.Report)
		}
		if lab.Reference == "" {
			c.add(at, lab.ID, "no reference")
		} else {
			c.checkTeacherPath(at+".reference", lab.ID, lab.Reference)
		}
		if len(c.rubric.LabRules(lab.ID)) == 0 {
			c.add(at, lab.ID, "no rule has the lab %s", lab.ID)
		}
	}
}

func (c *checker) checkScoring() {
	late := c.rubric.Scoring.LatePenalty
	if late.Min < 0 || late.Max > 1 || late.Min > late.Max {
		c.add("scoring.late_penalty", "", "the late penalty goes from min to max, between 0 and 1: got %v to %v", late.Min, late.Max)
	}
	for _, penalty := range c.rubric.Scoring.Penalties {
		c.checkPenalty("scoring.penalties."+penalty.Name, "", penalty)
	}
}

func (c *checker) checkPenalty(at, id string, penalty Penalty) {
	if penalty.Remove <= 0 || penalty.Remove > 1 {
		c.add(at, id, "a penalty removes more than 0 and at most 1 of the points: got %v", penalty.Remove)
	}
	if penalty.When == "" {
		c.add(at, id, "a penalty says when it applies")
	}
}

func (c *checker) checkRules() {
	levels := keySet(c.rubric.AnalysisLevels)
	kinds := keySet(c.rubric.Evidence.Kinds)
	labIndex := map[string]int{}
	for i, lab := range c.rubric.Labs {
		labIndex[lab.ID] = i
	}
	ruleLab := map[string]string{}
	for _, rule := range c.rubric.Rules {
		ruleLab[rule.ID] = rule.Lab
	}

	sums := map[string]float64{}
	lastLab, lastExercise := -1, ""
	for _, rule := range c.rubric.Rules {
		at := "rules." + rule.ID
		if !ruleIDPattern.MatchString(rule.ID) {
			c.add(at, rule.ID, "the ID of a rule is its lab, _ and 3 capital letters, such as L1_DKF")
		}
		index, ok := labIndex[rule.Lab]
		switch {
		case rule.Lab == "":
			c.add(at, rule.ID, "no lab")
		case !ok:
			c.add(at+".lab", rule.ID, "the lab %q is not a key of labs", rule.Lab)
		case !strings.HasPrefix(rule.ID, rule.Lab+"_"):
			c.add(at+".lab", rule.ID, "the ID does not start with its lab %s", rule.Lab)
		}
		if ok {
			// The rules are grouped by lab, in the order of the labs, then of the exercises.
			if index < lastLab {
				c.add(at, rule.ID, "the rules of %s come before those of %s", rule.Lab, c.rubric.Labs[lastLab].ID)
			} else if index == lastLab && compareExercises(rule.Exercise, lastExercise) < 0 {
				c.add(at+".exercise", rule.ID, "the exercise %s comes after %s, in the order of the file", rule.Exercise, lastExercise)
			}
			lastLab, lastExercise = index, rule.Exercise
		}
		if rule.Name == "" {
			c.add(at, rule.ID, "no name")
		}
		if !exercisePattern.MatchString(rule.Exercise) {
			c.add(at+".exercise", rule.ID, "the exercise %q is not a number such as 4, 4.1 or 4.1.1", rule.Exercise)
		}
		switch rule.Requirement {
		case RequirementMandatory:
			sums[rule.Lab] += rule.Coefficient
		case RequirementOptional:
		default:
			c.add(at+".requirement", rule.ID, "the requirement %q is not %s or %s", rule.Requirement, RequirementMandatory, RequirementOptional)
		}
		if steps := rule.Coefficient * 20; rule.Coefficient <= 0 || math.Abs(steps-math.Round(steps)) > 1e-9 {
			c.add(at+".coefficient", rule.ID, "the coefficient %v does not go by steps of 0.05", rule.Coefficient)
		}
		if rule.Description == "" {
			c.add(at, rule.ID, "no description")
		}
		for _, penalty := range rule.Penalties {
			c.checkPenalty(at+".penalties", rule.ID, penalty)
		}
		if rule.Analysis.Level == "" {
			c.add(at, rule.ID, "no analysis level")
		} else if !levels[rule.Analysis.Level] {
			c.add(at+".analysis", rule.ID, "the analysis level %q is not a key of analysis_levels", rule.Analysis.Level)
		}
		for _, item := range slices.Concat(rule.Evidence.Asked, rule.Evidence.Extra) {
			if !kinds[item.Kind] {
				c.problems = append(c.problems, Problem{
					Line:    item.line,
					Rule:    rule.ID,
					Message: fmt.Sprintf("the kind of evidence %q is not a key of evidence.kinds", item.Kind),
				})
			}
		}
		for _, p := range rule.Files.Team {
			c.checkTeamPath(at+".files", rule.ID, p)
		}
		for _, p := range rule.Files.Teacher {
			c.checkTeacherPath(at+".files", rule.ID, p)
		}
		for _, image := range rule.Images {
			if !imagePattern.MatchString(image) {
				c.add(at+".images", rule.ID, "the image %q is not a name and a tag, such as namespace/app-{team}:1.0.0", image)
			}
		}
		for _, dependency := range rule.DependsOn {
			lab, ok := ruleLab[dependency]
			switch {
			case !ok:
				c.add(at+".depends_on", rule.ID, "depends on %s, which is not a rule", dependency)
			case labIndex[lab] > labIndex[rule.Lab]:
				c.add(at+".depends_on", rule.ID, "depends on %s, of a later lab", dependency)
			}
		}
	}
	c.checkCycles()

	for _, lab := range c.rubric.Labs {
		if sum, ok := sums[lab.ID]; ok && math.Abs(sum-1) > 1e-9 {
			c.add("labs."+lab.ID, lab.ID, "the coefficients of the mandatory rules add up to %v, not 1", roundHundredths(sum))
		}
	}
}

// checkCycles reports the rules that depend on themselves, through their dependencies.
func (c *checker) checkCycles() {
	dependsOn := map[string][]string{}
	for _, rule := range c.rubric.Rules {
		dependsOn[rule.ID] = rule.DependsOn
	}
	const (
		visiting = 1
		done     = 2
	)
	state := map[string]int{}
	var visit func(id string, chain []string)
	visit = func(id string, chain []string) {
		switch state[id] {
		case visiting:
			start := slices.Index(chain, id)
			c.add("rules."+id+".depends_on", id, "a cycle of dependencies: %s", strings.Join(append(chain[start:], id), " -> "))
			return
		case done:
			return
		}
		state[id] = visiting
		for _, next := range dependsOn[id] {
			if _, ok := dependsOn[next]; ok {
				visit(next, append(chain, id))
			}
		}
		state[id] = done
	}
	for _, rule := range c.rubric.Rules {
		visit(rule.ID, nil)
	}
}

// checkTeamPath checks a path of the repository of a team: relative, and inside it.
func (c *checker) checkTeamPath(at, id, p string) {
	if path.IsAbs(p) || p != path.Clean(p) || p == ".." || strings.HasPrefix(p, "../") {
		c.add(at, id, "the path %q is not relative to the root of the repository of the team", p)
	}
}

// checkTeacherPath checks a path of the repository of the rules: it exists.
func (c *checker) checkTeacherPath(at, id, p string) {
	if path.IsAbs(p) || strings.HasPrefix(path.Clean(p), "../") {
		c.add(at, id, "the path %q is not relative to the root of the repository", p)
		return
	}
	if c.opts.Root == "" {
		return
	}
	if _, err := os.Stat(filepath.Join(c.opts.Root, filepath.FromSlash(p))); err != nil {
		c.add(at, id, "the file %q does not exist", p)
	}
}

func keySet(entries []Entry) map[string]bool {
	set := make(map[string]bool, len(entries))
	for _, entry := range entries {
		set[entry.Key] = true
	}
	return set
}

// compareExercises compares 2 exercises such as "4" and "4.1", part by part, as numbers. An
// exercise that is not a number compares equal, the check of its format reports it.
func compareExercises(a, b string) int {
	if a == "" || b == "" {
		return 0
	}
	x, y := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(x) && i < len(y); i++ {
		m, errM := strconv.Atoi(x[i])
		n, errN := strconv.Atoi(y[i])
		if errM != nil || errN != nil {
			return 0
		}
		if m != n {
			return m - n
		}
	}
	return len(x) - len(y)
}
