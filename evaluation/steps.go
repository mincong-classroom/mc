package evaluation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mincong-classroom/mc/rubric"
)

// Steps is the record of the steps of mc eval run for a team in a lab, in
// lab-{n}/.steps/{team}.json: the digests of the inputs of each step that an agent did, so that
// the next run skips the step while they stay the same. A step that fails records nothing, so it
// runs again.
type Steps struct {
	Report *StepRecord `json:"report,omitempty"` // The report, written from the facts.
	Grade  *StepRecord `json:"grade,omitempty"`  // The evaluation, one YAML file per team.
}

// StepRecord is a step that succeeded for a team: the digests of its inputs, and when it was done.
type StepRecord struct {
	Inputs Inputs    `json:"inputs"`
	Done   time.Time `json:"done"`
}

// Inputs are the digests of the inputs of a step, in SHA-256.
type Inputs struct {
	// Facts is the front matter of the facts, without the commit of mc: the commit evaluated and the
	// facts of each rule. A new version of mc that gathers the same facts, or a push after the
	// deadline, runs no step again.
	Facts string `json:"facts"`
	// Report is the report, for the grades.
	Report string `json:"report,omitempty"`
	// Rules are the rules of the lab, for the grades, without their coefficients.
	Rules string `json:"rules,omitempty"`
	// Files are the files of the inputs of the agent in the settings, such as its skill, by path.
	Files map[string]string `json:"files,omitempty"`
}

// stepsDir is the directory of the records, in the directory of the evaluations of a lab: a name
// that starts with a dot, so that Teams doesn't take it for a team.
const stepsDir = ".steps"

// StepsPath returns the record of the steps of the team, in the directory of the evaluations of a
// lab.
func StepsPath(labDir, team string) string {
	return filepath.Join(labDir, stepsDir, team+".json")
}

// ReadSteps reads the record of the steps of the team, which is empty when the file doesn't exist.
func ReadSteps(labDir, team string) (Steps, error) {
	var s Steps
	file := StepsPath(labDir, team)
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return Steps{}, fmt.Errorf("%s: %w: delete it to run every step again", file, err)
	}
	return s, nil
}

// WriteSteps writes the record of the steps of the team. It writes a temporary file first, then
// renames it, so that a run that stops never leaves half a record.
func WriteSteps(labDir, team string, s Steps) error {
	file := StepsPath(labDir, team)
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

// Changes returns what changed between the inputs of a former run and those of now, one item
// each, such as "the facts changed" or ".claude/skills/report-lab/SKILL.md added".
func Changes(former, now Inputs) []string {
	var changes []string
	for _, input := range []struct{ name, former, now string }{
		{"the facts", former.Facts, now.Facts},
		{"the report", former.Report, now.Report},
		{"the rules of the lab", former.Rules, now.Rules},
	} {
		if input.former != input.now {
			changes = append(changes, input.name+" changed")
		}
	}
	files := map[string]string{}
	for file := range former.Files {
		files[file] = ""
	}
	for file := range now.Files {
		files[file] = ""
	}
	for _, file := range sortedKeys(files) {
		before, ok := former.Files[file]
		after, found := now.Files[file]
		switch {
		case !ok:
			changes = append(changes, file+" added")
		case !found:
			changes = append(changes, file+" removed")
		case before != after:
			changes = append(changes, file+" changed")
		}
	}
	return changes
}

// Digest returns the SHA-256 of a value in JSON, whose maps have their keys sorted.
func Digest(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return digest(data), nil
}

// DigestFile returns the SHA-256 of a file, or "" when it doesn't exist.
func DigestFile(file string) (string, error) {
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return digest(data), nil
}

// DigestFiles returns the SHA-256 of each file of the inputs, relative to root unless absolute: a
// file, or every file of a directory. The key of a file is its path from the input, such as
// .claude/skills/report-lab/SKILL.md for the input .claude/skills/report-lab. The hidden files,
// such as .DS_Store, are left out. An input that doesn't exist is an error, so that a typo doesn't
// go unnoticed.
func DigestFiles(root string, inputs []string) (map[string]string, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	files := map[string]string{}
	for _, input := range inputs {
		dir := input
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(root, dir)
		}
		if _, err := os.Stat(dir); err != nil {
			return nil, fmt.Errorf("the input %s: %w", input, err)
		}
		err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if p != dir && strings.HasPrefix(d.Name(), ".") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(dir, p)
			if err != nil {
				return err
			}
			sum, err := DigestFile(p)
			if err != nil {
				return err
			}
			files[path.Join(filepath.ToSlash(input), filepath.ToSlash(rel))] = sum
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

// DigestRules returns the SHA-256 of what the rules say of a lab: the lab, its rules, and what
// applies to every rule, such as the penalties of every rule. It leaves out the coefficients,
// which the teacher tunes after the grades and which change no completeness, and the other labs.
func DigestRules(r *rubric.Rubric, lab string) (string, error) {
	l, ok := r.Lab(lab)
	if !ok {
		return "", fmt.Errorf("no lab %s in the rules", lab)
	}
	rules := r.LabRules(lab)
	for i := range rules {
		rules[i].Coefficient = 0
		rules[i].CoefficientReason = ""
	}
	return Digest(struct {
		Lab            rubric.Lab
		Rules          []rubric.Rule
		Scoring        rubric.Scoring
		Evidence       rubric.Evidence
		AnalysisLevels []rubric.Entry
		Observations   rubric.Observations
	}{l, rules, r.Scoring, r.Evidence, r.AnalysisLevels, r.Observations})
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// sortedKeys returns the keys of the map, sorted.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
