package evidence

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
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
}

// frontMatter returns the metadata as the front matter of a Markdown file.
func (m metadata) frontMatter() string {
	return fmt.Sprintf("---\ncommit: %s\nmc: %s\n---\n\n", m.Commit, m.MC)
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
	Commit string // The commit of the team evaluated, in full.
	MC     string // The commit of mc that gathered the facts.
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

// reportPath returns the report that an evaluator writes from the facts, next to the report of the
// team: docs/lab-1.report.md for docs/lab-1.md.
func reportPath(report string) string {
	return strings.TrimSuffix(report, path.Ext(report)) + ".report.md"
}
