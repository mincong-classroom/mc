package evidence

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mincong-classroom/mc/github"
	"github.com/mincong-classroom/mc/rubric"
)

var update = flag.Bool("update", false, "update the golden files of testdata")

// rules are made-up rules of one lab.
const rules = `title: Evaluation Rules
labs:
  L1:
    title: Build an Image
    report: docs/lab-1.md
    reference: answers/lab-1.md
    deadline: "2030-03-04T08:30:00+01:00"
analysis_levels:
  none: Commands only.
evidence:
  kinds:
    command: A command.
    file: A file.
    image: A Docker image.
rules:
  L1_BLD:
    lab: L1
    name: Build Test
    exercise: "1"
    requirement: mandatory
    coefficient: 0.6
    description: The team writes a Dockerfile, then builds the image.
    analysis:
      level: none
    evidence:
      asked:
        - file: the Dockerfile
        - command: the command docker build
      extra: []
    files:
      team:
        - app/Dockerfile
        - app/index.html
  L1_TAG:
    lab: L1
    name: Publish Test
    exercise: "2"
    requirement: mandatory
    coefficient: 0.4
    description: The team publishes the image.
    analysis:
      level: none
    evidence:
      asked:
        - image: the image 1.0.0
      extra: []
    images:
      - sample/app-{team}:1.0.0
`

// statement is the report of the template, which the team fills.
const statement = "# Build an Image\n" +
	"\n" +
	"## Introduction\n" +
	"\n" +
	"The lab builds an image.\n" +
	"\n" +
	"## Exercise 1: Build\n" +
	"\n" +
	"Write the Dockerfile at `app/Dockerfile`, then build it.\n" +
	"\n" +
	"```sh\n" +
	"# TODO: enter the docker command here\n" +
	"```\n" +
	"\n" +
	"## Exercise 2: Publish\n" +
	"\n" +
	"Publish the image.\n" +
	"\n" +
	"```sh\n" +
	"# TODO: paste the publish command here\n" +
	"```\n"

// answer is the report of the team at the deadline.
const answer = "# Build an Image\n" +
	"\n" +
	"## Introduction\n" +
	"\n" +
	"The lab builds an image.\n" +
	"\n" +
	"## Exercise 1: Build\n" +
	"\n" +
	"Write the Dockerfile at `app/Dockerfile`, then build it.\n" +
	"\n" +
	"```sh\n" +
	"# TODO: enter the docker command here\n" +
	"docker build -t sample/app-red:1.0.0 app\n" +
	"```\n" +
	"\n" +
	"We built it on a Mac, see [the logs](../logs/build.txt).\n" +
	"\n" +
	"![The build](assets/build.png)\n" +
	"\n" +
	"## Exercise 2: Publish\n" +
	"\n" +
	"Publish the image.\n" +
	"\n" +
	"```sh\n" +
	"# TODO: paste the publish command here\n" +
	"```\n"

func TestGather(t *testing.T) {
	root := gitEnv(t)
	deadline := time.Date(2030, 3, 4, 7, 30, 0, 0, time.UTC)
	page := strings.Repeat("<p>The clinic.</p>\n", 50)
	templateFiles := map[string]string{
		"docs/lab-1.md":      statement,
		"app/Dockerfile":     "# TODO: write the Dockerfile\n",
		"app/index.html":     page + "<footer>Team</footer>\n" + page,
		"k8s/lab-2/pod.yaml": "# TODO\n",
		".gitignore":         "*.log\n",
	}

	template := filepath.Join(root, "template")
	commitFiles(t, template, "Initial commit", templateFiles)

	// The repository of the team: created from the template, then 1 commit before the deadline
	// and 1 after.
	work := filepath.Join(root, "work")
	commitFiles(t, work, "Initial commit", templateFiles)
	before := commitFiles(t, work, "Lab 1", map[string]string{
		"docs/lab-1.md":         answer,
		"app/Dockerfile":        "FROM eclipse-temurin:21\nCOPY target/app.jar /app.jar\nEXPOSE 8080\nCMD [\"java\", \"-jar\", \"/app.jar\"]\n",
		"app/index.html":        page + "<footer>Team Red</footer>\n" + page,
		"docs/assets/build.png": "\x89PNG\r\n\x1a\n",
		"logs/build.txt":        "#1 [internal] load build definition\n#2 DONE 0.1s\n",
		".DS_Store":             "\x00\x00\x00\x01Bud1",
		"k8s/lab-2/pod.yaml":    "apiVersion: v1\nkind: Pod\n",
	})
	after := commitFiles(t, work, "Lab 1, late", map[string]string{
		"docs/lab-1.md": strings.Replace(answer, "# TODO: paste the publish command here\n", "# TODO: paste the publish command here\ndocker push sample/app-red:1.0.0\n", 1),
	})
	clone := filepath.Join(root, "k8s-red")
	run(t, root, "clone", "--quiet", work, clone)

	r, err := rubric.Parse([]byte(rules))
	if err != nil {
		t.Fatalf("rubric.Parse(): %v", err)
	}
	tmpl, err := LoadTemplate(template)
	if err != nil {
		t.Fatalf("LoadTemplate(): %v", err)
	}
	team := Team{Name: "red", Repo: "k8s-red", Clone: clone}
	opts := Options{
		Rubric:   r,
		Lab:      "L1",
		Template: tmpl,
		Dir:      filepath.Join(root, "teacher"),
		Pushes: fakePushes{
			{Time: deadline.Add(-time.Hour), After: before, Type: "push"},
			{Time: deadline.Add(time.Hour), After: after, Type: "push"},
		},
		Registry: fakeRegistry{"sample/app-red": {
			{Name: "1.0.0", LastPushed: deadline.Add(-2 * time.Hour), Digest: "sha256:0f3e5d6c7b8a9f00", Images: []TagImage{{OS: "linux", Architecture: "amd64"}, {OS: "unknown", Architecture: "unknown"}}},
			{Name: "1.0.1", LastPushed: deadline.Add(2 * time.Hour), Digest: "sha256:1a2b3c4d5e6f7a8b", Images: []TagImage{{OS: "linux", Architecture: "amd64"}}},
			{Name: "2.0.0", LastPushed: deadline.Add(3 * time.Hour), Digest: "sha256:ffff"},
		}},
		Revision: "0a1b2c3d4e5f",
	}
	facts := filepath.Join(root, "teacher", "k8s-red", "docs", "lab-1.facts.md")

	// The first pass: the commit at the deadline.
	result, err := Gather(team, opts)
	if err != nil {
		t.Fatalf("Gather(): %v", err)
	}
	if result.Commit != before || !result.Committed || result.File != facts || len(result.Renamed) != 0 {
		t.Errorf("Gather() = %+v, want the commit %s committed to %s, and nothing renamed", result, before, facts)
	}
	assertGolden(t, facts, "lab-1.facts.md")

	// The evaluator adds its notes and its files, then runs it again.
	evaluatorDir := filepath.Join(root, "teacher", "k8s-red", "docs", "lab-1.evidence")
	writeFiles(t, evaluatorDir, map[string]string{
		"L1_BLD.md":      "The image 1.0.0 runs, and the home page answers on port 8080.\n",
		"l1-bld-run.log": "Started App in 3.2 seconds\n",
		"home.png":       "\x89PNG\r\n\x1a\n",
	})
	if result, err = Gather(team, opts); err != nil || !result.Committed {
		t.Fatalf("Gather() with the files of the evaluator = %+v, %v, want them committed", result, err)
	}
	if out := run(t, filepath.Join(root, "teacher", "k8s-red"), "ls-files", "docs/lab-1.evidence/l1-bld-run.log"); out == "" {
		t.Errorf("the log of the evaluator is not committed: the .gitignore of the team ignores *.log")
	}
	content := readFile(t, facts)
	for _, want := range []string{
		"The image 1.0.0 runs, and the home page answers on port 8080.",
		"#### [`l1-bld-run.log`](lab-1.evidence/l1-bld-run.log)\n\n```text\nStarted App in 3.2 seconds\n```",
		"## Other Files of the Evaluator\n\n#### [`home.png`](lab-1.evidence/home.png)\n\n![home.png](lab-1.evidence/home.png)",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("the facts lack %q:\n%s", want, content)
		}
	}

	// Nothing changed: nothing to commit.
	if result, err = Gather(team, opts); err != nil || result.Committed {
		t.Errorf("Gather() again = %+v, %v, want nothing committed", result, err)
	}

	// The evaluator writes its report from the facts, with their front matter: the next pass
	// skips the team, unless it is forced, or mc changed.
	report := filepath.Join(root, "teacher", "k8s-red", "docs", "lab-1.report.md")
	writeFiles(t, filepath.Dir(report), map[string]string{"lab-1.report.md": frontMatterOf(t, facts) + "# Lab 1, Team Red\n"})
	if want := "---\ncommit: " + before + "\nmc: 0a1b2c3d4e5f\nrules:\n"; !strings.HasPrefix(readFile(t, facts), want) {
		t.Errorf("the facts don't start with the front matter %q", want)
	}
	if result, err = Gather(team, opts); err != nil || !result.UpToDate || result.File != report {
		t.Errorf("Gather() with a report up to date = %+v, %v, want it up to date", result, err)
	}
	// An image pushed again, without a new commit of the team, makes a new pass.
	repushed := opts
	repushed.Registry = fakeRegistry{"sample/app-red": {
		{Name: "1.0.0", LastPushed: deadline.Add(4 * time.Hour), Digest: "sha256:9a8b7c6d5e4f3a2b", Images: []TagImage{{OS: "linux", Architecture: "amd64"}}},
	}}
	if result, err = Gather(team, repushed); err != nil || result.UpToDate || !result.Committed {
		t.Errorf("Gather() with an image pushed again = %+v, %v, want the facts gathered and committed", result, err)
	}
	if want := "  L1_TAG:\n    images: \"1.0.0 9a8b7c6d5e4f\"\n    screenshots: 0\n"; !strings.Contains(readFile(t, facts), want) {
		t.Errorf("the facts after the image pushed again lack %q:\n%s", want, frontMatterOf(t, facts))
	}
	if result, err = Gather(team, opts); err != nil || !result.UpToDate {
		t.Errorf("Gather() back to the images of the report = %+v, %v, want it up to date again", result, err)
	}
	forced := opts
	forced.Force = true
	if result, err = Gather(team, forced); err != nil || result.UpToDate {
		t.Errorf("Gather() forced = %+v, %v, want the facts gathered", result, err)
	}
	newMC := opts
	newMC.Revision = "9f8e7d6c5b4a"
	if result, err = Gather(team, newMC); err != nil || result.UpToDate || !result.Committed {
		t.Errorf("Gather() with another mc = %+v, %v, want the facts gathered and committed", result, err)
	}

	// A late pass merges origin/main into the branch.
	opts.Late = true
	if result, err = Gather(team, opts); err != nil || result.Commit != after || !result.Committed {
		t.Fatalf("Gather() late = %+v, %v, want the commit %s committed", result, err, after)
	}
	if !strings.Contains(readFile(t, facts), "docker push sample/app-red:1.0.0") {
		t.Errorf("the late report lacks the work pushed after the deadline")
	}
	if out := run(t, filepath.Join(root, "teacher", "k8s-red"), "merge-base", "--is-ancestor", after, "HEAD"); out != "" {
		t.Errorf("merge-base = %q", out)
	}

	// The branch teacher stays local.
	if out := run(t, clone, "ls-remote", "--heads", "origin", Branch); out != "" {
		t.Errorf("the remote has the branch %s: %q", Branch, out)
	}
}

func TestGatherRenamesTheFormerFiles(t *testing.T) {
	root := gitEnv(t)
	deadline := time.Date(2030, 3, 4, 7, 30, 0, 0, time.UTC)
	work := filepath.Join(root, "work")
	commitFiles(t, work, "Initial commit", map[string]string{"docs/lab-1.md": statement})
	commit := commitFiles(t, work, "Lab 1", map[string]string{"docs/lab-1.md": answer})
	clone := filepath.Join(root, "k8s-red")
	run(t, root, "clone", "--quiet", work, clone)
	r, err := rubric.Parse([]byte(rules))
	if err != nil {
		t.Fatalf("rubric.Parse(): %v", err)
	}
	team := Team{Name: "red", Repo: "k8s-red", Clone: clone}
	opts := Options{
		Rubric:   r,
		Lab:      "L1",
		Dir:      filepath.Join(root, "teacher"),
		Pushes:   fakePushes{{Time: deadline.Add(-time.Hour), After: commit, Type: "push"}},
		Registry: fakeRegistry{},
		Revision: "0a1b2c3d4e5f",
	}
	if _, err := Gather(team, opts); err != nil {
		t.Fatalf("Gather(): %v", err)
	}

	// Given the files of a former version of mc, committed to the branch teacher: the facts, and the
	// report that links to them.
	worktree := filepath.Join(root, "teacher", "k8s-red")
	front := frontMatterOf(t, filepath.Join(worktree, "docs", "lab-1.facts.md"))
	run(t, worktree, "mv", "docs/lab-1.facts.md", "docs/lab-1.evidence.md")
	writeFiles(t, worktree, map[string]string{
		"docs/lab-1.generated.md": front + "# Lab 1, Team Red\n\n" +
			"From the facts: [`lab-1.evidence.md`](lab-1.evidence.md), and the evidence of [`lab-1.evidence/`](lab-1.evidence/).\n",
	})
	run(t, worktree, "add", "--all")
	run(t, worktree, "commit", "--quiet", "-m", "[L1] Generated report")

	// When the next pass runs, with the same commits.
	result, err := Gather(team, opts)
	if err != nil {
		t.Fatalf("Gather() with the former files: %v", err)
	}

	// Then the files have their names of now, and the report is still up to date.
	report := filepath.Join(worktree, "docs", "lab-1.report.md")
	want := []Rename{
		{From: "docs/lab-1.evidence.md", To: "docs/lab-1.facts.md"},
		{From: "docs/lab-1.generated.md", To: "docs/lab-1.report.md"},
	}
	if !result.UpToDate || result.File != report || !slices.Equal(result.Renamed, want) {
		t.Errorf("Gather() with the former files = %+v, want the report up to date, and %+v renamed", result, want)
	}
	if got, want := readFile(t, report), "From the facts: [`lab-1.facts.md`](lab-1.facts.md), and the evidence of [`lab-1.evidence/`](lab-1.evidence/).\n"; !strings.HasSuffix(got, want) {
		t.Errorf("the report after the rename =\n%s\nwant it to end with\n%s", got, want)
	}
	if got, want := run(t, worktree, "log", "-1", "--format=%s"), "[L1] Rename docs/lab-1.evidence.md to lab-1.facts.md, and docs/lab-1.generated.md to lab-1.report.md"; got != want {
		t.Errorf("the last commit of the branch teacher = %q, want %q", got, want)
	}
	if got := run(t, worktree, "log", "--follow", "--format=%s", "--", "docs/lab-1.facts.md"); !strings.Contains(got, "[L1] Facts at") {
		t.Errorf("the history of the facts = %q, want the commits of their former name", got)
	}
	if got := run(t, worktree, "status", "--porcelain"); got != "" {
		t.Errorf("the worktree after the rename has changes: %q", got)
	}

	// A pass after the rename has nothing to rename.
	if result, err = Gather(team, opts); err != nil || !result.UpToDate || len(result.Renamed) != 0 {
		t.Errorf("Gather() after the rename = %+v, %v, want the report up to date, and nothing renamed", result, err)
	}
}

func TestGatherRenamesAnUntrackedReport(t *testing.T) {
	root := gitEnv(t)
	dir := filepath.Join(root, "k8s-red")
	commitFiles(t, dir, "Initial commit", map[string]string{"docs/lab-1.md": statement})
	run(t, dir, "checkout", "--quiet", "-b", Branch)
	writeFiles(t, dir, map[string]string{"docs/lab-1.generated.md": "# Lab 1, Team Red\n"})

	renamed, err := renameFormerFiles(dir, rubric.Lab{ID: "L1", Report: "docs/lab-1.md"})
	if want := []Rename{{From: "docs/lab-1.generated.md", To: "docs/lab-1.report.md"}}; err != nil || !slices.Equal(renamed, want) {
		t.Errorf("renameFormerFiles() = %+v, %v, want %+v", renamed, err, want)
	}
	if got, want := run(t, dir, "status", "--porcelain"), "?? docs/lab-1.report.md"; got != want {
		t.Errorf("git status after the rename = %q, want %q: the report stays untracked", got, want)
	}
}

func TestFindSection(t *testing.T) {
	lines := strings.Split("# Lab\n## Exercise 1: A\none\n### Notes of the team\ntwo\n## Exercise 10: B\n### 10.1 Part\nthree\n### 10.2 Part\nfour\n```\n## Exercise 3 in a code block\n```\n", "\n")
	for _, c := range []struct {
		exercise   string
		start, end int
		found      bool
	}{
		{"1", 2, 5, true},
		{"10", 6, 14, true},
		{"10.1", 7, 8, true},
		{"10.2", 9, 14, true},
		{"3", 0, 0, false},
		{"2", 0, 0, false},
	} {
		s, ok := findSection(lines, c.exercise)
		if ok != c.found || s.start != c.start || s.end != c.end {
			t.Errorf("findSection(%q) = lines %d to %d, %v, want %d to %d, %v", c.exercise, s.start, s.end, ok, c.start, c.end, c.found)
		}
	}
}

func TestRepoPath(t *testing.T) {
	for _, c := range []struct {
		target, want string
		ok           bool
	}{
		{"assets/a.png", "docs/assets/a.png", true},
		{"../k8s/lab-1/pod.yaml#L3", "k8s/lab-1/pod.yaml", true},
		{"/apps/Dockerfile", "apps/Dockerfile", true},
		{"my%20file.txt", "docs/my file.txt", true},
		{"https://example.com/a.png", "", false},
		{"#exercise-1", "", false},
		{"../../outside.md", "", false},
	} {
		got, ok := repoPath("docs/lab-1.md", c.target)
		if got != c.want || ok != c.ok {
			t.Errorf("repoPath(%q) = %q, %v, want %q, %v", c.target, got, ok, c.want, c.ok)
		}
	}
}

type fakePushes []github.Push

func (p fakePushes) MainPushes(string) ([]github.Push, error) {
	return p, nil
}

type fakeRegistry map[string][]Tag

func (r fakeRegistry) Tags(repository string) ([]Tag, bool, error) {
	tags, ok := r[repository]
	return tags, ok, nil
}

// gitEnv makes the commits of the test the same at each run, whatever the configuration of the
// machine, and returns a temporary directory.
func gitEnv(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved // On macOS, git prints /private/var for /var.
	}
	for key, value := range map[string]string{
		"HOME":                root,
		"GIT_CONFIG_GLOBAL":   filepath.Join(root, "gitconfig"),
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME":     "Teacher",
		"GIT_AUTHOR_EMAIL":    "teacher@example.com",
		"GIT_AUTHOR_DATE":     "2030-03-01T10:00:00Z",
		"GIT_COMMITTER_NAME":  "Teacher",
		"GIT_COMMITTER_EMAIL": "teacher@example.com",
		"GIT_COMMITTER_DATE":  "2030-03-01T10:00:00Z",
	} {
		t.Setenv(key, value)
	}
	return root
}

// commitFiles writes the files in the repository, created if needed, commits them, and returns
// the commit.
func commitFiles(t *testing.T, dir, message string, files map[string]string) string {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		run(t, "", "init", "--quiet", "-b", "main", dir)
	}
	writeFiles(t, dir, files)
	run(t, dir, "add", "--all")
	run(t, dir, "commit", "--quiet", "-m", message)
	return run(t, dir, "rev-parse", "HEAD")
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// frontMatterOf returns the front matter of a file, as a report copies it from the facts.
func frontMatterOf(t *testing.T, p string) string {
	t.Helper()
	content := readFile(t, p)
	i := strings.Index(content, "\n---\n\n")
	if !strings.HasPrefix(content, "---\n") || i < 0 {
		t.Fatalf("%s has no front matter:\n%s", p, content)
	}
	return content[:i+len("\n---\n\n")]
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	content, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func assertGolden(t *testing.T, p, name string) {
	t.Helper()
	got := readFile(t, p)
	golden := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v: run go test ./evidence -update", err)
	}
	if got != string(want) {
		t.Errorf("the report differs from %s: run go test ./evidence -update, then review the diff.\ngot:\n%s", golden, got)
	}
}

func TestReadMetadata(t *testing.T) {
	dir := t.TempDir()
	for _, c := range []struct {
		name    string
		content string // None when empty.
		want    metadata
		wantOK  bool
	}{
		{name: "no file"},
		{name: "no front matter", content: "# Lab 1\n"},
		{name: "one key only", content: "---\ncommit: c028aea\n---\n# Lab 1\n"},
		{
			name:    "both keys",
			content: "---\ncommit: c028aea\nmc: 0a1b2c3d4e5f\n---\n\n# Lab 1\n",
			want:    metadata{Commit: "c028aea", MC: "0a1b2c3d4e5f"},
			wantOK:  true,
		},
		{
			name:    "Windows line endings",
			content: "---\r\ncommit: c028aea\r\nmc: 0a1b2c3d4e5f\r\n---\r\n# Lab 1\r\n",
			want:    metadata{Commit: "c028aea", MC: "0a1b2c3d4e5f"},
			wantOK:  true,
		},
		{
			name: "the facts of the rules",
			content: metadata{Commit: "c028aea", MC: "0a1b2c3d4e5f", Rules: map[string]RuleFacts{
				"L1_TAG": {Images: "1.0.0 0f3e5d6c7b8a, 1.0.1 1a2b3c4d5e6f", Screenshots: 2},
				"L1_BLD": {Screenshots: 0},
			}}.frontMatter([]string{"L1_BLD", "L1_TAG", "L1_RUN"}) + "# Lab 1\n",
			want: metadata{Commit: "c028aea", MC: "0a1b2c3d4e5f", Rules: map[string]RuleFacts{
				"L1_TAG": {Images: "1.0.0 0f3e5d6c7b8a, 1.0.1 1a2b3c4d5e6f", Screenshots: 2},
				"L1_BLD": {Screenshots: 0},
			}},
			wantOK: true,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := filepath.Join(dir, strings.ReplaceAll(c.name, " ", "-")+".md")
			if c.content != "" {
				if err := os.WriteFile(p, []byte(c.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, ok, err := readMetadata(p)
			if err != nil || ok != c.wantOK || !reflect.DeepEqual(got, c.want) {
				t.Errorf("readMetadata() = %+v, %v, %v, want %+v, %v", got, ok, err, c.want, c.wantOK)
			}
		})
	}
}

func TestReadFacts(t *testing.T) {
	worktree := filepath.Join(t.TempDir(), "k8s-teal")
	lab := rubric.Lab{ID: "L2", Report: "docs/lab-2.md"}
	report := FactsPath(worktree, lab)
	if want := filepath.Join(worktree, "docs", "lab-2.facts.md"); report != want {
		t.Errorf("FactsPath() = %q, want %q", report, want)
	}

	if got, ok, err := ReadFacts(report); err != nil || ok {
		t.Errorf("ReadFacts() before mc eval gather-facts = %+v, %v, %v, want no facts", got, ok, err)
	}

	content := `---
commit: 0a1b2c3d4e5f60718293a4b5c6d7e8f901234567
mc: 949d7aa740ab
rules:
  L2_POD:
    images: "1.1.0 b80767bd1a2c"
    screenshots: 3
---

# Lab 2
`
	if err := os.MkdirAll(filepath.Dir(report), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(report, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	want := Facts{
		Commit: "0a1b2c3d4e5f60718293a4b5c6d7e8f901234567",
		MC:     "949d7aa740ab",
		Rules:  map[string]RuleFacts{"L2_POD": {Images: "1.1.0 b80767bd1a2c", Screenshots: 3}},
	}
	if got, ok, err := ReadFacts(report); err != nil || !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("ReadFacts() = %+v, %v, %v, want %+v, true", got, ok, err, want)
	}
}

func TestPlural(t *testing.T) {
	for _, c := range []struct {
		n    int
		word string
		want string
	}{
		{n: 1, word: "push", want: "1 push"},
		{n: 8, word: "push", want: "8 pushes"},
		{n: 2, word: "force push", want: "2 force pushes"},
		{n: 0, word: "line", want: "0 lines"},
	} {
		if got := plural(c.n, c.word); got != c.want {
			t.Errorf("plural(%d, %q) = %q, want %q", c.n, c.word, got, c.want)
		}
	}
}
