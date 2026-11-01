package evidence

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/mincong-classroom/mc/rubric"
)

// metadata tells what a file was made from, in its front matter. The facts carry it, and the
// report written from the facts copies it: a report whose metadata are those of a new pass is up to
// date.
type metadata struct {
	Commit string `yaml:"commit"` // The commit of the team evaluated, in full.
	MC     string `yaml:"mc"`     // The commit of mc that gathered the facts.
	// Rules are the facts of each rule that the evaluation sheet shows, by the ID of the rule.
	Rules map[string]RuleFacts `yaml:"rules,omitempty"`
}

// RuleFacts are the facts of a rule that the evaluation sheet shows, in the columns TA Docker
// Images and TA Screenshots.
type RuleFacts struct {
	// Images are the tags of the images of the rule on Docker Hub, the tag of the rule and its later
	// patches, each with the first 12 characters of its digest: "1.0.0 b80767bd1a2c, 1.0.1
	// 0f3e5d6c7b8a". It is "none" when Docker Hub has none of them, and empty when the rule has no
	// image, or when Docker Hub couldn't be read.
	Images string `yaml:"images,omitempty"`
	// Screenshots is the number of screenshots of the team in the section of the exercise of the
	// rule, among the files of its repository.
	Screenshots int `yaml:"screenshots"`
}

// noImages is the images of a rule whose tags Docker Hub doesn't have.
const noImages = "none"

// frontMatter returns the metadata as the front matter of a Markdown file, with the rules in the
// order given.
func (m metadata) frontMatter(order []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\ncommit: %s\nmc: %s\n", m.Commit, m.MC)
	if len(m.Rules) > 0 {
		b.WriteString("rules:\n")
	}
	for _, id := range order {
		rf, ok := m.Rules[id]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "  %s:\n", id)
		if rf.Images != "" {
			fmt.Fprintf(&b, "    images: %s\n", strconv.Quote(rf.Images))
		}
		fmt.Fprintf(&b, "    screenshots: %d\n", rf.Screenshots)
	}
	b.WriteString("---\n\n")
	return b.String()
}

// upToDate reports whether a report with these metadata is up to date: the same commits as now,
// and the same images on Docker Hub, by rule. An image that Docker Hub couldn't give now, an
// empty one, doesn't count.
func (m metadata) upToDate(commit, revision string, images map[string]string) bool {
	if m.Commit != commit || m.MC != revision {
		return false
	}
	for id, now := range images {
		if now != "" && m.Rules[id].Images != now {
			return false
		}
	}
	return true
}

// readMetadata reads the metadata in the front matter of the facts or of a report. It reports false
// when the file doesn't exist, or when it has no front matter with both keys.
func readMetadata(p string) (metadata, bool, error) {
	var m metadata
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return m, false, nil
	}
	if err != nil {
		return m, false, err
	}
	rest, ok := strings.CutPrefix(strings.ReplaceAll(string(data), "\r\n", "\n"), "---\n")
	if !ok {
		return m, false, nil
	}
	front, _, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		return m, false, nil
	}
	if err := yaml.Unmarshal([]byte(front), &m); err != nil {
		return metadata{}, false, fmt.Errorf("the front matter of %s: %w", p, err)
	}
	if m.Commit == "" || m.MC == "" {
		return metadata{}, false, nil
	}
	return m, true, nil
}

// Facts are what the front matter of the facts tells of their pass.
type Facts struct {
	Commit string               // The commit of the team evaluated, in full.
	MC     string               // The commit of mc that gathered the facts.
	Rules  map[string]RuleFacts // The facts of each rule for the evaluation sheet, by its ID.
}

// FactsPath returns the facts of a lab in the worktree of a team, such as
// ~/.mc/2026/teacher/k8s-red/docs/lab-1.facts.md for the report docs/lab-1.md of the lab.
func FactsPath(worktree string, lab rubric.Lab) string {
	facts, _ := factsPaths(lab.Report)
	return filepath.Join(worktree, filepath.FromSlash(facts))
}

// ReadFacts reads the front matter of the facts. It reports false when the file doesn't exist, or
// has no front matter with both commits: mc eval gather-facts hasn't gathered the facts yet.
func ReadFacts(file string) (Facts, bool, error) {
	m, ok, err := readMetadata(file)
	return Facts(m), ok, err // The same fields: a conversion, which the tags don't prevent.
}

// ReportPath returns the report that the evaluator writes from the facts of a lab, in the worktree
// of a team, such as ~/.mc/2026/teacher/k8s-red/docs/lab-1.report.md. Its front matter is the one
// of the facts, so ReadFacts reads it too.
func ReportPath(worktree string, lab rubric.Lab) string {
	return filepath.Join(worktree, filepath.FromSlash(reportPath(lab.Report)))
}

// reportPath returns the report that an evaluator writes from the facts, next to the report of the
// team: docs/lab-1.report.md for docs/lab-1.md.
func reportPath(report string) string {
	return strings.TrimSuffix(report, path.Ext(report)) + ".report.md"
}
