package evidence

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// git runs git in a directory.
type git struct {
	dir string
}

// run returns the standard output of the git command.
func (g git) run(args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", g.dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("git -C %s %s: %s", g.dir, strings.Join(args, " "), message)
	}
	return stdout.Bytes(), nil
}

// text returns the standard output of the git command, without its final newline.
func (g git) text(args ...string) (string, error) {
	out, err := g.run(args...)
	return strings.TrimSpace(string(out)), err
}

// ok reports whether the git command succeeds.
func (g git) ok(args ...string) bool {
	_, err := g.run(args...)
	return err == nil
}

// entry is a file of a tree.
type entry struct {
	path string
	blob string
	size int64
}

// tree returns the files of the commit, by path.
func (g git) tree(rev string) (map[string]entry, error) {
	out, err := g.run("ls-tree", "-r", "-l", "-z", rev)
	if err != nil {
		return nil, err
	}
	files := map[string]entry{}
	for _, record := range strings.Split(string(out), "\x00") {
		// <mode> SP <type> SP <object> SP <size> TAB <path>
		meta, path, ok := strings.Cut(record, "\t")
		if !ok {
			continue
		}
		fields := strings.Fields(meta)
		if len(fields) != 4 || fields[1] != "blob" {
			continue
		}
		size, _ := strconv.ParseInt(fields[3], 10, 64) // "-" for a submodule, which is not a blob.
		files[path] = entry{path: path, blob: fields[2], size: size}
	}
	return files, nil
}

// blob returns the content of a blob.
func (g git) blob(hash string) ([]byte, error) {
	return g.run("cat-file", "blob", hash)
}

// Template is the repository from which the repositories of the teams were created: every
// version of its files, to tell what a team wrote from what the template gave it.
type Template struct {
	git   git
	blobs map[string][]string // The blobs of each path, in every commit of the template.
}

// LoadTemplate reads the history of the clone of the template, at origin/main if it has one, so
// that a clone left on another branch still gives every version.
func LoadTemplate(dir string) (*Template, error) {
	g := git{dir: dir}
	ref := "origin/main"
	if !g.ok("rev-parse", "--verify", "--quiet", ref+"^{commit}") {
		ref = "HEAD"
	}
	out, err := g.run("log", "--root", "--raw", "--no-abbrev", "--no-renames", "--format=", "-z", ref)
	if err != nil {
		return nil, err
	}
	t := &Template{git: g, blobs: map[string][]string{}}
	records := strings.Split(string(out), "\x00")
	for i := 0; i+1 < len(records); i++ {
		// :<old mode> <new mode> <old blob> <new blob> <status>, then the path.
		meta := strings.TrimSpace(records[i])
		if !strings.HasPrefix(meta, ":") {
			continue
		}
		fields := strings.Fields(meta)
		path := records[i+1]
		i++
		if len(fields) < 5 || strings.Trim(fields[3], "0") == "" {
			continue // A deleted file.
		}
		if blob := fields[3]; !slices.Contains(t.blobs[path], blob) {
			t.blobs[path] = append(t.blobs[path], blob)
		}
	}
	return t, nil
}

// fromTemplate reports whether the file is one of the template, unchanged.
func (t *Template) fromTemplate(path, blob string) bool {
	return t != nil && slices.Contains(t.blobs[path], blob)
}

// versions returns the contents of every version of the path in the template.
func (t *Template) versions(path string) ([][]byte, error) {
	if t == nil {
		return nil, nil
	}
	var contents [][]byte
	for _, blob := range t.blobs[path] {
		content, err := t.git.blob(blob)
		if err != nil {
			return nil, err
		}
		contents = append(contents, content)
	}
	return contents, nil
}

// statementLines returns the lines of every version of the file in the template, trimmed: a line
// of a report among them is the statement's, not the team's.
func (t *Template) statementLines(path string) (map[string]bool, error) {
	contents, err := t.versions(path)
	if err != nil || len(contents) == 0 {
		return nil, err
	}
	lines := map[string]bool{}
	for _, content := range contents {
		for _, line := range strings.Split(string(content), "\n") {
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				lines[trimmed] = true
			}
		}
	}
	return lines, nil
}

// diffWithTemplate returns the smallest diff between a version of the path in the template and
// the content, in the unified format without its headers, and the number of lines it changes.
// It returns false when the template has no version of the path.
func (t *Template) diffWithTemplate(path string, content []byte) (string, int, bool, error) {
	versions, err := t.versions(path)
	if err != nil || len(versions) == 0 {
		return "", 0, false, err
	}
	best, bestChanged := "", -1
	for _, version := range versions {
		diff, changed, err := unifiedDiff(version, content)
		if err != nil {
			return "", 0, false, err
		}
		if bestChanged < 0 || changed < bestChanged {
			best, bestChanged = diff, changed
		}
	}
	return best, bestChanged, true, nil
}

// unifiedDiff returns the diff of 2 contents, without its headers, and the number of lines added
// or removed.
func unifiedDiff(old, new []byte) (string, int, error) {
	dir, err := os.MkdirTemp("", "mc-evidence-")
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = os.RemoveAll(dir) }() // A temporary directory: nothing to do if it stays.
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	if err := os.WriteFile(a, old, 0o600); err != nil {
		return "", 0, err
	}
	if err := os.WriteFile(b, new, 0o600); err != nil {
		return "", 0, err
	}
	cmd := exec.Command("git", "diff", "--no-index", "--no-color", "--unified=2", a, b)
	out, err := cmd.Output()
	var exit *exec.ExitError
	// git diff --no-index exits with 1 when the files differ.
	if err != nil && (!errors.As(err, &exit) || exit.ExitCode() != 1) {
		return "", 0, fmt.Errorf("git diff --no-index: %w", err)
	}
	var lines []string
	changed := 0
	inHunk := false
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if strings.HasPrefix(line, "@@") {
			inHunk = true
		}
		if !inHunk || line == "" {
			continue
		}
		if line[0] == '+' || line[0] == '-' {
			changed++
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"), changed, nil
}

// templateFromCommit returns the files of a commit as the template, such as the first commit of
// the repository of a team, which the template created.
func templateFromCommit(g git, rev string) (*Template, error) {
	files, err := g.tree(rev)
	if err != nil {
		return nil, err
	}
	t := &Template{git: g, blobs: map[string][]string{}}
	for p, e := range files {
		t.blobs[p] = []string{e.blob}
	}
	return t, nil
}
