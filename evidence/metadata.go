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

// metadata tells what a report was made from, in its front matter. The report of the evidence
// carries it, and the report generated from the evidence copies it: a generated report whose
// metadata are those of a new pass is up to date.
type metadata struct {
	Commit string `yaml:"commit"` // The commit of the team evaluated, in full.
	MC     string `yaml:"mc"`     // The commit of mc that gathered the evidence.
}

// frontMatter returns the metadata as the front matter of a Markdown file.
func (m metadata) frontMatter() string {
	return fmt.Sprintf("---\ncommit: %s\nmc: %s\n---\n\n", m.Commit, m.MC)
}

// readMetadata reads the metadata in the front matter of a report. It reports false when the file
// doesn't exist, or when it has no front matter with both keys.
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

// Facts are what the front matter of a report of the evidence tells of its pass.
type Facts struct {
	Commit string // The commit of the team evaluated, in full.
	MC     string // The commit of mc that gathered the evidence.
}

// ReportPath returns the report of the evidence of a lab in the worktree of a team, such as
// ~/.mc/2026/teacher/k8s-red/docs/lab-1.evidence.md for the report docs/lab-1.md of the lab.
func ReportPath(worktree string, lab rubric.Lab) string {
	report, _ := evidencePaths(lab.Report)
	return filepath.Join(worktree, filepath.FromSlash(report))
}

// ReadFacts reads the facts in the front matter of a report of the evidence. It reports false
// when the report doesn't exist, or has no front matter with both commits: mc evidence hasn't
// gathered the evidence yet.
func ReadFacts(report string) (Facts, bool, error) {
	m, ok, err := readMetadata(report)
	return Facts(m), ok, err // The same fields: a conversion, which the tags don't prevent.
}

// generatedPath returns the report that an evaluator generates from the evidence, next to the
// report of the team: docs/lab-1.generated.md for docs/lab-1.md.
func generatedPath(report string) string {
	return strings.TrimSuffix(report, path.Ext(report)) + ".generated.md"
}
