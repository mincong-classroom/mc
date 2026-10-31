package evidence

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
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
	}
	reportPath := filepath.Join(root, "teacher", "k8s-red", "docs", "lab-1.evidence.md")

	// The first pass: the commit at the deadline.
	result, err := Gather(team, opts)
	if err != nil {
		t.Fatalf("Gather(): %v", err)
	}
	if result.Commit != before || !result.Committed || result.Report != reportPath {
		t.Errorf("Gather() = %+v, want the commit %s committed to %s", result, before, reportPath)
	}
	assertGolden(t, reportPath, "lab-1.evidence.md")

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
	content := readFile(t, reportPath)
	for _, want := range []string{
		"The image 1.0.0 runs, and the home page answers on port 8080.",
		"#### [`l1-bld-run.log`](lab-1.evidence/l1-bld-run.log)\n\n```text\nStarted App in 3.2 seconds\n```",
		"## Other Files of the Evaluator\n\n#### [`home.png`](lab-1.evidence/home.png)\n\n![home.png](lab-1.evidence/home.png)",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("the report lacks %q:\n%s", want, content)
		}
	}

	// Nothing changed: nothing to commit.
	if result, err = Gather(team, opts); err != nil || result.Committed {
		t.Errorf("Gather() again = %+v, %v, want nothing committed", result, err)
	}

	// A late pass merges origin/main into the branch.
	opts.Late = true
	if result, err = Gather(team, opts); err != nil || result.Commit != after || !result.Committed {
		t.Fatalf("Gather() late = %+v, %v, want the commit %s committed", result, err, after)
	}
	if !strings.Contains(readFile(t, reportPath), "docker push sample/app-red:1.0.0") {
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
