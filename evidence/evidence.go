// Package evidence gathers the facts of each rule of a lab for a team, in a file for the teacher,
// such as docs/lab-1.facts.md. It holds facts, not a score, of 3 kinds: what the team wrote in its
// report, the files of its repository, and the evidence that the evaluator made, such as
// screenshots and logs.
//
// The facts go to the local branch "teacher" of the clone of the team, checked out in a worktree:
// the branch starts from the commit evaluated, and the next pass merges the new commit into it. It
// is never pushed.
//
// The front matter of the facts holds the commit evaluated and the commit of mc. An evaluator that
// writes a report from the facts, such as docs/lab-1.report.md, copies it: while both commits stay
// the same, the report is up to date, and a new pass skips the team.
package evidence

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/mincong-classroom/mc/github"
	"github.com/mincong-classroom/mc/rubric"
)

// Branch is the local branch of the clone of a team that holds the facts and the reports.
const Branch = "teacher"

// Team is a team whose facts are gathered.
type Team struct {
	Name  string // The name of the team, which replaces {team} in the images of a rule.
	Repo  string // The name of its repository, such as k8s-red.
	Clone string // The directory of its local clone.
}

// Pushes gives the pushes to main of a repository of the organization, oldest first.
type Pushes interface {
	MainPushes(repo string) ([]github.Push, error)
}

// Registry gives the tags of a repository of Docker images.
type Registry interface {
	Tags(repository string) ([]Tag, bool, error)
}

// Options are the options of Gather.
type Options struct {
	Rubric *rubric.Rubric
	Lab    string // The ID of the lab, such as L1.
	// Template tells the files and the lines of the template from those of the team. When nil,
	// the first commit of the repository of the team is the template.
	Template *Template
	Dir      string // The directory of the worktrees of the branch teacher, one per team.
	Late     bool   // Evaluate origin/main, for a late pass, instead of the head of main at the deadline.
	Rev      string // Evaluate this commit instead.
	Pushes   Pushes // When nil, the commit at the deadline is found from the dates of the commits.
	Registry Registry
	// Revision is the commit of mc that runs, written in the front matter of the facts. When
	// empty, the report is never up to date.
	Revision string
	// Force gathers the facts again even when the report is up to date.
	Force bool
}

// Result is what Gather did for a team.
type Result struct {
	Commit    string // The commit evaluated.
	How       string // How the commit was chosen, e.g. "the head of main at the deadline".
	File      string // The file of the facts in the worktree, or the one of the report when up to date.
	Committed bool   // Whether the facts changed, and were committed to the branch teacher.
	// UpToDate tells that the report is at the commit evaluated, from the same commit of mc:
	// nothing was gathered, and File is the report.
	UpToDate bool
	// Renamed are the files of the former names that this pass renamed in the worktree, such as
	// docs/lab-1.evidence.md to docs/lab-1.facts.md.
	Renamed []Rename
}

// Gather writes the facts of the lab for the team, and commits them to the local branch teacher of
// its clone.
func Gather(team Team, opts Options) (Result, error) {
	lab, ok := opts.Rubric.Lab(opts.Lab)
	if !ok {
		return Result{}, fmt.Errorf("no lab %s in the rules", opts.Lab)
	}
	deadline, err := lab.DeadlineTime()
	if err != nil {
		return Result{}, fmt.Errorf("the deadline of %s: %w", lab.ID, err)
	}
	clone := git{dir: team.Clone}
	if _, err := clone.run("fetch", "--quiet", "origin"); err != nil {
		return Result{}, err
	}
	commit, err := resolveCommit(clone, team, opts, deadline)
	if err != nil {
		return Result{}, err
	}
	// Before the check below, which reads the report under its name of now.
	renamed, err := renameFormerFiles(filepath.Join(opts.Dir, team.Repo), lab)
	if err != nil {
		return Result{}, err
	}
	if !opts.Force && opts.Revision != "" {
		report := filepath.Join(opts.Dir, team.Repo, filepath.FromSlash(reportPath(lab.Report)))
		m, ok, err := readMetadata(report)
		if err != nil {
			return Result{}, err
		}
		if ok && m == (metadata{Commit: commit.hash, MC: opts.Revision}) {
			return Result{Commit: commit.hash, How: commit.how, File: report, UpToDate: true, Renamed: renamed}, nil
		}
	}
	worktree, err := prepareWorktree(clone, filepath.Join(opts.Dir, team.Repo), commit.hash)
	if err != nil {
		return Result{}, err
	}

	template := opts.Template
	if template == nil {
		root, err := clone.text("rev-list", "--max-parents=0", commit.hash)
		if err != nil {
			return Result{}, err
		}
		roots := strings.Fields(root)
		if template, err = templateFromCommit(clone, roots[len(roots)-1]); err != nil {
			return Result{}, err
		}
	}
	r, err := build(buildInput{
		rubric:   opts.Rubric,
		lab:      lab,
		deadline: deadline,
		team:     team,
		commit:   commit,
		clone:    clone,
		worktree: worktree.dir,
		template: template,
		registry: opts.Registry,
		revision: opts.Revision,
	})
	if err != nil {
		return Result{}, err
	}

	facts := filepath.Join(worktree.dir, filepath.FromSlash(r.factsPath))
	if err := os.MkdirAll(filepath.Dir(facts), 0o755); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(facts, []byte(r.markdown()), 0o644); err != nil {
		return Result{}, err
	}
	committed, err := commitFacts(worktree, r, lab)
	if err != nil {
		return Result{}, err
	}
	return Result{Commit: commit.hash, How: commit.how, File: facts, Committed: committed, Renamed: renamed}, nil
}

// commitInfo is the commit evaluated, and the pushes around it.
type commitInfo struct {
	hash     string
	how      string
	pushedAt time.Time     // Zero when unknown.
	none     bool          // No push to main before the deadline: the commit is the template.
	later    []github.Push // The pushes to main after the deadline.
	forced   []github.Push // The pushes that rewrote the history of main.
	late     bool          // The commit is origin/main, after the deadline.
	activity bool          // Whether the pushes come from the activity of the repository.
}

func resolveCommit(clone git, team Team, opts Options, deadline time.Time) (commitInfo, error) {
	var c commitInfo
	var pushes []github.Push
	if opts.Pushes != nil {
		var err error
		if pushes, err = opts.Pushes.MainPushes(team.Repo); err != nil {
			return c, err
		}
		c.activity = true
	}
	for _, push := range pushes {
		if push.Type == "force_push" {
			c.forced = append(c.forced, push)
		}
		if push.Time.After(deadline) {
			c.later = append(c.later, push)
		}
	}

	switch {
	case opts.Rev != "":
		c.how = "the commit asked for"
		c.hash = opts.Rev
	case opts.Late:
		c.how = "origin/main, in a late pass"
		c.hash = "origin/main"
		c.late = true
	case c.activity:
		c.how = "the head of main at the deadline"
		for _, push := range pushes {
			if !push.Time.After(deadline) {
				c.hash, c.pushedAt = push.After, push.Time
			}
		}
	default:
		c.how = "the head of main at the deadline, from the dates of the commits"
		hash, err := clone.text("rev-list", "-1", "--before="+deadline.Format(time.RFC3339), "origin/main")
		if err != nil {
			return c, err
		}
		c.hash = hash
	}
	if c.hash == "" {
		root, err := clone.text("rev-list", "--max-parents=0", "origin/main")
		if err != nil {
			return c, err
		}
		roots := strings.Fields(root)
		c.hash = roots[len(roots)-1]
		c.none = true
		c.how = "the first commit: no push to main before the deadline"
	}
	hash, err := clone.text("rev-parse", "--verify", "--quiet", c.hash+"^{commit}")
	if err != nil {
		return c, fmt.Errorf("the commit %s is not in the clone %s: was the history rewritten? %w", c.hash, clone.dir, err)
	}
	c.hash = hash
	return c, nil
}

// prepareWorktree checks out the branch teacher in the directory, and merges the commit into it.
func prepareWorktree(clone git, dir, hash string) (git, error) {
	wt := git{dir: dir}
	_, err := os.Stat(dir)
	switch {
	case err == nil:
		if err := checkBranch(wt); err != nil {
			return wt, err
		}
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return wt, err
		}
		args := []string{"worktree", "add", "--quiet", dir, Branch}
		if !clone.ok("rev-parse", "--verify", "--quiet", "refs/heads/"+Branch) {
			args = []string{"worktree", "add", "--quiet", "-b", Branch, dir, hash}
		}
		if _, err := clone.run(args...); err != nil {
			return wt, err
		}
	default:
		return wt, err
	}
	if wt.ok("merge-base", "--is-ancestor", hash, "HEAD") {
		return wt, nil
	}
	message := fmt.Sprintf("Merge %s, the commit evaluated", short(hash))
	if _, err := wt.run("-c", "commit.gpgsign=false", "merge", "--quiet", "--no-edit", "-m", message, hash); err != nil {
		_, _ = wt.run("merge", "--abort") // The merge failed already: keep its error.
		return wt, err
	}
	return wt, nil
}

// checkBranch returns an error unless the worktree has the branch teacher checked out.
func checkBranch(wt git) error {
	branch, err := wt.text("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || branch != Branch {
		return fmt.Errorf("%s is not a worktree of the branch %s: %q", wt.dir, Branch, branch)
	}
	return nil
}

// commitFacts commits the facts and the files of the evaluator, if they changed.
func commitFacts(wt git, r *labFacts, lab rubric.Lab) (bool, error) {
	paths := []string{r.factsPath}
	if _, err := os.Stat(filepath.Join(wt.dir, filepath.FromSlash(r.evidenceDir))); err == nil {
		paths = append(paths, r.evidenceDir)
	}
	// --force: these paths are the evaluator's, even when the .gitignore of the team ignores some of
	// their files, such as *.log.
	if _, err := wt.run(append([]string{"add", "--all", "--force", "--"}, paths...)...); err != nil {
		return false, err
	}
	if wt.ok("diff", "--cached", "--quiet") {
		return false, nil
	}
	message := fmt.Sprintf("[%s] Facts at %s", lab.ID, short(r.commit.hash))
	if _, err := wt.run("-c", "commit.gpgsign=false", "commit", "--quiet", "--no-verify", "-m", message); err != nil {
		return false, err
	}
	return true, nil
}

// factsPaths returns the facts and the directory of the evaluator, next to the report of the team:
// docs/lab-1.facts.md and docs/lab-1.evidence/ for docs/lab-1.md.
func factsPaths(report string) (string, string) {
	base := strings.TrimSuffix(report, path.Ext(report))
	return base + ".facts.md", base + ".evidence"
}

func short(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}
