// Package e2e runs the mc binary end to end, against the team registry and the school list of
// testdata/mc, and a fake gh CLI (testdata/bin/gh) serving the GitHub responses of
// testdata/github. The fake gh records the changes mc asks for instead of making them.
package e2e

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"

	// Imported so that a change of the mc sources invalidates the cached results of these tests:
	// go test only tracks the packages imported by the test, not the binary it builds.
	_ "github.com/mincong-classroom/mc/cmd"
)

var mcPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "mc-e2e-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	mcPath = filepath.Join(dir, "mc")
	build := exec.Command("go", "build", "-o", mcPath, "github.com/mincong-classroom/mc")
	build.Stdout = os.Stderr
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to build mc: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

var readFixtures sync.Once

// env is a HOME whose ~/.mc holds a copy of testdata/mc.
type env struct {
	home  string
	ghLog string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	// go test only tracks the files opened by the test process, not by mc or the fake gh: read
	// them all once, so that a change of a fixture invalidates the cached results.
	readFixtures.Do(func() {
		err := filepath.WalkDir("testdata", func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				_, err = os.ReadFile(path)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	home := t.TempDir()
	mcDir := filepath.Join(home, ".mc")
	files, err := os.ReadDir("testdata/mc")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(mcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join("testdata/mc", file.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(mcDir, file.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return &env{home: home, ghLog: filepath.Join(t.TempDir(), "gh.log")}
}

type result struct {
	stdout   string
	stderr   string
	exitCode int
}

// run runs mc with the given arguments and standard input.
func (e *env) run(t *testing.T, stdin string, args ...string) result {
	t.Helper()
	testdata, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(mcPath, args...)
	cmd.Env = append(os.Environ(),
		"HOME="+e.home,
		"MC_YEAR=2026",
		"PATH="+filepath.Join(testdata, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FAKE_GH_DATA="+filepath.Join(testdata, "github"),
		"FAKE_GH_LOG="+e.ghLog,
	)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	r := result{}
	var exitErr *exec.ExitError
	if err := cmd.Run(); errors.As(err, &exitErr) {
		r.exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("mc %v: %v", args, err)
	}
	r.stdout = stdout.String()
	r.stderr = stderr.String()
	return r
}

// ghChanges returns the gh commands changing GitHub that mc ran, in order.
func (e *env) ghChanges(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(e.ghLog)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

// The JSON output of mc, as a user of --json reads it.

type teamJSON struct {
	Name              string       `json:"name"`
	Repo              string       `json:"repo"`
	Members           []memberJSON `json:"members"`
	Status            *statusJSON  `json:"status"`
	Problems          []string     `json:"problems"`
	StudentsNotInTeam []string     `json:"studentsNotInTeam"` // validate only
}

type memberJSON struct {
	Name       string `json:"name"`
	Github     string `json:"github"`
	GithubName string `json:"githubName"`
	State      string `json:"state"`
}

type statusJSON struct {
	RepoExists bool   `json:"repoExists"`
	TeamExists bool   `json:"teamExists"`
	Access     string `json:"access"`
	Error      string `json:"error"`
}

type lsJSON struct {
	Year              int        `json:"year"`
	Teams             []teamJSON `json:"teams"`
	StudentsNotInTeam []string   `json:"studentsNotInTeam"`
}

func decode[T any](t *testing.T, r result) T {
	t.Helper()
	var value T
	if err := json.Unmarshal([]byte(r.stdout), &value); err != nil {
		t.Fatalf("invalid JSON: %v\nstdout:\n%s\nstderr:\n%s", err, r.stdout, r.stderr)
	}
	return value
}

var (
	red = teamJSON{
		Name: "red",
		Repo: "k8s-red",
		Members: []memberJSON{
			{Name: "SMITH, John", Github: "jsmith", GithubName: "John Smith", State: "active"},
			{Name: "DOE, Jane", Github: "jdoe", State: "pending"},
		},
		Status: &statusJSON{RepoExists: true, TeamExists: true, Access: "write"},
	}
	blue = teamJSON{
		Name:    "blue",
		Repo:    "k8s-blue",
		Members: []memberJSON{{Name: "MARTIN, Alex", Github: "amartin", GithubName: "Alex Martin"}},
		Status:  &statusJSON{},
	}
	green = teamJSON{
		Name:    "green",
		Repo:    "k8s-green",
		Members: []memberJSON{},
		Status:  &statusJSON{RepoExists: true, TeamExists: true, Access: "admin"},
	}
	pink = teamJSON{
		Name:    "Pink-2",
		Repo:    "k8s-Pink-2",
		Members: []memberJSON{{Name: "NOBODY, Someone", Github: "ghost-user-404"}},
		Status:  &statusJSON{},
		Problems: []string{
			`invalid name "Pink-2": use lowercase letters only, such as a color`,
			"NOBODY, Someone is not on the school list",
			"the GitHub user @ghost-user-404 does not exist",
		},
	}
)

func TestTeamLsJSON(t *testing.T) {
	r := newEnv(t).run(t, "", "team", "ls", "--json")

	if r.exitCode != 0 {
		t.Fatalf("exit code %d, stderr:\n%s", r.exitCode, r.stderr)
	}
	got := decode[lsJSON](t, r)
	want := lsJSON{
		Year:              2026,
		Teams:             []teamJSON{red, blue, green, pink},
		StudentsNotInTeam: []string{"DURAND, Camille"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mc team ls --json =\n%+v\nwant\n%+v", got, want)
	}
}

func TestTeamLs(t *testing.T) {
	r := newEnv(t).run(t, "", "team", "ls")

	for _, line := range []string{
		"  - red: SMITH, John (@jsmith), DOE, Jane (@jdoe)",
		"    ✓ repo k8s-red · ✓ team red · ✓ access write · @jsmith active · @jdoe pending",
		"    ✗ repo k8s-blue · ✗ team blue · ✗ no access · @amartin not invited",
		"  - green: no members yet",
		"    ✓ repo k8s-green · ✓ team green · ⚠ access admin instead of write",
		"    ⚠ NOBODY, Someone is not on the school list",
		"1 of 4 students not in a team yet:",
		"  - DURAND, Camille",
	} {
		if !strings.Contains(r.stdout, line+"\n") {
			t.Errorf("mc team ls: missing line %q in:\n%s", line, r.stdout)
		}
	}
}

func TestTeamLsWithoutSchoolList(t *testing.T) {
	e := newEnv(t)
	if err := os.Remove(filepath.Join(e.home, ".mc", "students-2026.yaml")); err != nil {
		t.Fatal(err)
	}

	r := e.run(t, "", "team", "ls", "--json")

	got := decode[lsJSON](t, r)
	if got.StudentsNotInTeam != nil {
		t.Errorf("studentsNotInTeam = %v, want null", got.StudentsNotInTeam)
	}
	if problems := got.Teams[3].Problems; len(problems) != 2 {
		t.Errorf("Pink-2 problems = %q, want no school list problem", problems)
	}
	if !strings.Contains(r.stderr, "No school list at") {
		t.Errorf("stderr does not explain the missing school list:\n%s", r.stderr)
	}
}

func TestTeamLsWithoutRegistry(t *testing.T) {
	e := newEnv(t)
	if err := os.Remove(filepath.Join(e.home, ".mc", "teams-2026.yaml")); err != nil {
		t.Fatal(err)
	}

	r := e.run(t, "", "team", "ls")

	if r.exitCode != 1 || !strings.Contains(r.stderr, "failed to list teams") {
		t.Errorf("exit code %d, stderr:\n%s", r.exitCode, r.stderr)
	}
}

func TestTeamHelpListsTheTeams(t *testing.T) {
	r := newEnv(t).run(t, "", "team", "--help")

	for _, pattern := range []string{
		`(?m)^  ls +List the teams`,
		`(?m)^  provision +Provision a team`,
		`(?m)^  red +SMITH, John \(@jsmith\), DOE, Jane \(@jdoe\)$`,
		`(?m)^  green +no members yet$`,
	} {
		if !regexp.MustCompile(pattern).MatchString(r.stdout) {
			t.Errorf("mc team --help does not match %q:\n%s", pattern, r.stdout)
		}
	}
}

func TestTeamValidate(t *testing.T) {
	e := newEnv(t)

	r := e.run(t, "", "team", "red", "validate", "--json")
	if r.exitCode != 0 {
		t.Fatalf("mc team red validate: exit code %d, stderr:\n%s", r.exitCode, r.stderr)
	}
	want := teamJSON{
		Name: "red",
		Repo: "k8s-red",
		Members: []memberJSON{
			{Name: "SMITH, John", Github: "jsmith", GithubName: "John Smith"},
			{Name: "DOE, Jane", Github: "jdoe"},
		},
		StudentsNotInTeam: []string{"DURAND, Camille"},
	}
	if got := decode[teamJSON](t, r); !reflect.DeepEqual(got, want) {
		t.Errorf("mc team red validate --json =\n%+v\nwant\n%+v", got, want)
	}

	r = e.run(t, "", "team", "red", "validate")
	for _, line := range []string{
		`  - SMITH, John (@jsmith): "John Smith" on GitHub`,
		"  - DOE, Jane (@jdoe): no display name on GitHub",
		"  ✓ valid",
	} {
		if !strings.Contains(r.stdout, line+"\n") {
			t.Errorf("mc team red validate: missing line %q in:\n%s", line, r.stdout)
		}
	}
}

func TestTeamValidateInvalid(t *testing.T) {
	r := newEnv(t).run(t, "", "team", "Pink-2", "validate", "--json")

	if r.exitCode != 1 || !strings.Contains(r.stderr, "the team Pink-2 is not valid") {
		t.Errorf("exit code %d, stderr:\n%s", r.exitCode, r.stderr)
	}
	if got := decode[teamJSON](t, r).Problems; !reflect.DeepEqual(got, pink.Problems) {
		t.Errorf("problems = %q, want %q", got, pink.Problems)
	}
}

func TestTeamStatus(t *testing.T) {
	e := newEnv(t)

	r := e.run(t, "", "team", "red", "status")
	want := "✓ repo k8s-red · ✓ team red · ✓ access write · @jsmith active · @jdoe pending\n"
	if r.stdout != want {
		t.Errorf("mc team red status = %q, want %q", r.stdout, want)
	}

	r = e.run(t, "", "team", "blue", "status", "--json")
	wantBlue := blue
	wantBlue.Members = []memberJSON{{Name: "MARTIN, Alex", Github: "amartin"}}
	if got := decode[teamJSON](t, r); !reflect.DeepEqual(got, wantBlue) {
		t.Errorf("mc team blue status --json =\n%+v\nwant\n%+v", got, wantBlue)
	}
}

func TestUnknownTeamOrAction(t *testing.T) {
	e := newEnv(t)
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"team", "purple", "status"}, `unknown team or action "purple"`},
		{[]string{"team", "red", "provision"}, `unknown action "provision" for the team red`},
	}
	for _, tt := range tests {
		r := e.run(t, "", tt.args...)
		if r.exitCode != 1 || !strings.Contains(r.stderr, tt.want) {
			t.Errorf("mc %v: exit code %d, stderr %q, want %q", tt.args, r.exitCode, r.stderr, tt.want)
		}
	}
}

func TestTeamProvision(t *testing.T) {
	const (
		createBlueRepo = "gh repo create mincong-classroom/k8s-blue --private --template mincong-classroom/containers"
		createBlueTeam = "gh api -X POST orgs/mincong-classroom/teams -f name=blue -f privacy=secret"
		grantBlue      = "gh api -X PUT orgs/mincong-classroom/teams/blue/repos/mincong-classroom/k8s-blue -f permission=push"
		inviteAmartin  = "gh api -X PUT orgs/mincong-classroom/teams/blue/memberships/amartin -f role=member"
		grantGreen     = "gh api -X PUT orgs/mincong-classroom/teams/green/repos/mincong-classroom/k8s-green -f permission=push"
	)
	tests := []struct {
		name         string
		args         []string
		stdin        string
		wantExitCode int
		wantChanges  []string
		wantOutput   []string
	}{
		{
			name:        "all the steps of a new team",
			stdin:       "blue\na\n\n",
			wantChanges: []string{createBlueRepo, createBlueTeam, grantBlue, inviteAmartin},
			wantOutput:  []string{"Teams: red, blue, green, Pink-2", "== Team blue", "✓ done"},
		},
		{
			name:       "dry run",
			args:       []string{"--dry-run"},
			stdin:      "blue\na\n\n",
			wantOutput: []string{"(dry run) not run"},
		},
		{
			name:       "the steps already done are skipped",
			stdin:      "red\n\n",
			wantOutput: []string{"✓ the repository exists", "✓ already a member", "✓ already invited (pending)"},
		},
		{
			name:        "the admin access is replaced by push",
			stdin:       "green\ny\n\n",
			wantChanges: []string{grantGreen},
		},
		{
			name:        "several teams, all applies to one team only",
			stdin:       "blue\na\ngreen\ny\n\n",
			wantChanges: []string{createBlueRepo, createBlueTeam, grantBlue, inviteAmartin, grantGreen},
		},
		{
			name:        "no skips the rest of the team",
			stdin:       "blue\ny\nn\n\n",
			wantChanges: []string{createBlueRepo},
			wantOutput:  []string{"Skipped, with the remaining steps of the team"},
		},
		{
			name:       "an unknown team is asked again",
			stdin:      "purple\n\n",
			wantOutput: []string{`Unknown team "purple".`},
		},
		{
			name:         "an invalid team is not provisioned",
			stdin:        "Pink-2\n\n",
			wantExitCode: 1,
			wantOutput:   []string{"Not provisioned: fix the registry"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)

			r := e.run(t, tt.stdin, append([]string{"team", "provision"}, tt.args...)...)

			if r.exitCode != tt.wantExitCode {
				t.Errorf("exit code %d, want %d, stderr:\n%s", r.exitCode, tt.wantExitCode, r.stderr)
			}
			if got := e.ghChanges(t); !reflect.DeepEqual(got, tt.wantChanges) {
				t.Errorf("changes =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tt.wantChanges, "\n"))
			}
			for _, want := range tt.wantOutput {
				if !strings.Contains(r.stdout, want) {
					t.Errorf("output does not contain %q:\n%s", want, r.stdout)
				}
			}
		})
	}
}
