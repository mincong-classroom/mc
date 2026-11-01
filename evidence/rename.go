package evidence

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mincong-classroom/mc/rubric"
)

// Rename is a file of a former name that a pass renamed in the worktree of a team.
type Rename struct {
	From string // The former path, relative to the worktree, such as docs/lab-1.evidence.md.
	To   string // The path of now, such as docs/lab-1.facts.md.
}

// formerNames returns the files of the former names next to the report of the team, with their
// names of now: the facts were docs/lab-1.evidence.md, and the report of the evaluator
// docs/lab-1.generated.md, for docs/lab-1.md. The facts come first.
func formerNames(report string) []Rename {
	base := strings.TrimSuffix(report, path.Ext(report))
	facts, _ := factsPaths(report)
	return []Rename{
		{From: base + ".evidence.md", To: facts},
		{From: base + ".generated.md", To: reportPath(report)},
	}
}

// renameFormerFiles renames the files of the former names in the worktree of the team, when it
// exists, and commits the renames to the branch teacher. A file tracked by git is renamed with
// git mv, so that it keeps its history. The report of the evaluator links to the facts, so its
// links follow them. A file whose name of now exists already stays as it is.
func renameFormerFiles(dir string, lab rubric.Lab) ([]Rename, error) {
	wt := git{dir: dir}
	former := formerNames(lab.Report)
	var renamed []Rename
	for _, r := range former {
		from, to := filepath.Join(dir, filepath.FromSlash(r.From)), filepath.Join(dir, filepath.FromSlash(r.To))
		fromFound, err := exists(from)
		if err != nil {
			return nil, err
		}
		toFound, err := exists(to)
		if err != nil {
			return nil, err
		}
		if !fromFound || toFound {
			continue
		}
		if len(renamed) == 0 {
			if err := checkBranch(wt); err != nil {
				return nil, err
			}
		}
		if wt.ok("ls-files", "--error-unmatch", "--", r.From) {
			if _, err := wt.run("mv", "--", r.From, r.To); err != nil {
				return nil, err
			}
		} else if err := os.Rename(from, to); err != nil {
			return nil, err
		}
		renamed = append(renamed, r)
	}
	if len(renamed) == 0 {
		return nil, nil
	}

	if facts := former[0]; renamed[0] == facts {
		if err := relinkFacts(wt, facts, former[1].To); err != nil {
			return nil, err
		}
	}
	if wt.ok("diff", "--cached", "--quiet") {
		return renamed, nil // Untracked files only: nothing to commit.
	}
	moves := make([]string, len(renamed))
	for i, r := range renamed {
		moves[i] = r.From + " to " + path.Base(r.To)
	}
	message := fmt.Sprintf("[%s] Rename %s", lab.ID, strings.Join(moves, ", and "))
	if _, err := wt.run("-c", "commit.gpgsign=false", "commit", "--quiet", "--no-verify", "-m", message); err != nil {
		return nil, err
	}
	return renamed, nil
}

// relinkFacts replaces the former name of the facts with the one of now in the report of the
// evaluator, if it exists, such as the link [lab-1.evidence.md](lab-1.evidence.md).
func relinkFacts(wt git, facts Rename, report string) error {
	file := filepath.Join(wt.dir, filepath.FromSlash(report))
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	content := strings.ReplaceAll(string(data), path.Base(facts.From), path.Base(facts.To))
	if content == string(data) {
		return nil
	}
	info, err := os.Stat(file)
	if err != nil {
		return err
	}
	if err := os.WriteFile(file, []byte(content), info.Mode().Perm()); err != nil {
		return err
	}
	if wt.ok("ls-files", "--error-unmatch", "--", report) {
		_, err = wt.run("add", "--", report)
	}
	return err
}

// exists reports whether the file exists.
func exists(file string) (bool, error) {
	_, err := os.Stat(file)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}
