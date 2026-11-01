// Package e2e runs the mc binary end to end, against the team registry of testdata/mc (made-up
// names), and a fake gh CLI (testdata/bin/gh) serving the GitHub responses of
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
	vars  []string // More environment variables, which replace those of run, e.g. "MC_YEAR=".
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
	cmd.Env = append(cmd.Env, e.vars...) // The last value of a variable wins.
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
	Name     string       `json:"name"`
	Repo     string       `json:"repo"`
	Members  []memberJSON `json:"members"`
	Status   *statusJSON  `json:"status"`
	Errors   []string     `json:"errors"`
	Warnings []string     `json:"warnings"`
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
			{Name: "SMITH John", Github: "jsmith", GithubName: "John Smith", State: "active"},
			{Name: "DOE Jane", Github: "jdoe", State: "pending"},
		},
		Status: &statusJSON{RepoExists: true, TeamExists: true, Access: "write"},
	}
	blue = teamJSON{
		Name:    "blue",
		Repo:    "k8s-blue",
		Members: []memberJSON{{Name: "MARTIN Alex", Github: "amartin", GithubName: "Alex Martin"}},
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
		Members: []memberJSON{{Name: "NOBODY Someone", Github: "ghost-user-404"}},
		Status:  &statusJSON{},
		Errors: []string{
			`invalid name "Pink-2": use lowercase letters only, such as a color`,
			"the GitHub user @ghost-user-404 does not exist",
		},
		Warnings: []string{"NOBODY Someone is not among the students of the registry"},
	}
)

func TestTeamLsJSON(t *testing.T) {
	r := newEnv(t).run(t, "", "team", "ls", "--json")

	if r.exitCode != 0 {
		t.Fatalf("exit code %d, stderr:\n%s", r.exitCode, r.stderr)
	}
	prefix := `{
    "year": 2026,
    "teams": [
        {
`
	if !strings.HasPrefix(r.stdout, prefix) {
		t.Errorf("mc team ls --json is not indented with 4 spaces:\n%s", r.stdout)
	}
	got := decode[lsJSON](t, r)
	want := lsJSON{
		Year:              2026,
		Teams:             []teamJSON{red, blue, green, pink},
		StudentsNotInTeam: []string{"DURAND Camille"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mc team ls --json =\n%+v\nwant\n%+v", got, want)
	}
}

func TestTeamLs(t *testing.T) {
	r := newEnv(t).run(t, "", "team", "ls")

	for _, line := range []string{
		"  - red: SMITH John (@jsmith), DOE Jane (@jdoe)",
		"    ✓ repo k8s-red · ✓ team red · ✓ access write · @jsmith active · @jdoe pending",
		"    ✗ repo k8s-blue · ✗ team blue · ✗ no access · @amartin not invited",
		"  - green: no members yet",
		"    ✓ repo k8s-green · ✓ team green · ⚠ access admin instead of write",
		"    ✗ the GitHub user @ghost-user-404 does not exist",
		"    ⚠ NOBODY Someone is not among the students of the registry",
		"1 of 4 students not in a team yet:",
		"  - DURAND Camille",
	} {
		if !strings.Contains(r.stdout, line+"\n") {
			t.Errorf("mc team ls: missing line %q in:\n%s", line, r.stdout)
		}
	}
}

func TestTeamLsWithoutStudents(t *testing.T) {
	e := newEnv(t)
	teamFile := filepath.Join(t.TempDir(), "teams.yaml")
	registry := `teams:
  - name: green
    members: []
`
	if err := os.WriteFile(teamFile, []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}

	r := e.run(t, "", "team", "ls", "--json", "--team-file", teamFile)

	want := lsJSON{Year: 2026, Teams: []teamJSON{green}}
	if got := decode[lsJSON](t, r); !reflect.DeepEqual(got, want) {
		t.Errorf("mc team ls --json without students =\n%+v\nwant\n%+v", got, want)
	}

	r = e.run(t, "", "team", "ls", "--team-file", teamFile)
	if r.exitCode != 0 || strings.Contains(r.stdout, "not in a team") || r.stderr != "" {
		t.Errorf("mc team ls without students: exit code %d, stdout:\n%s\nstderr:\n%s", r.exitCode, r.stdout, r.stderr)
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
		`(?m)^  red +SMITH John \(@jsmith\), DOE Jane \(@jdoe\)$`,
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
			{Name: "SMITH John", Github: "jsmith", GithubName: "John Smith"},
			{Name: "DOE Jane", Github: "jdoe"},
		},
	}
	if got := decode[teamJSON](t, r); !reflect.DeepEqual(got, want) {
		t.Errorf("mc team red validate --json =\n%+v\nwant\n%+v", got, want)
	}

	r = e.run(t, "", "team", "red", "validate")
	for _, line := range []string{
		`  - SMITH John (@jsmith): "John Smith" on GitHub`,
		"  - DOE Jane (@jdoe): no display name on GitHub",
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
	if got := decode[teamJSON](t, r); !reflect.DeepEqual(got.Errors, pink.Errors) || !reflect.DeepEqual(got.Warnings, pink.Warnings) {
		t.Errorf("errors = %q, warnings = %q, want %q and %q", got.Errors, got.Warnings, pink.Errors, pink.Warnings)
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
	wantBlue.Members = []memberJSON{{Name: "MARTIN Alex", Github: "amartin"}}
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
		{[]string{"team", "provision"}, `"mc team provision" is now "mc team <team> provision"`},
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
		team         string
		args         []string
		stdin        string
		wantExitCode int
		wantChanges  []string
		wantOutput   []string
	}{
		{
			name:        "one confirmation, then all the steps",
			team:        "blue",
			stdin:       "y\n",
			wantChanges: []string{createBlueRepo, createBlueTeam, grantBlue, inviteAmartin},
			wantOutput: []string{
				`
== Team blue: 1 member
  - MARTIN Alex (@amartin): "Alex Martin" on GitHub
Provision the team blue? (y/N): ✓ Create the private repository mincong-classroom/k8s-blue from the template mincong-classroom/containers
`,
				`
Team "blue" provisioned.
- repo: https://github.com/mincong-classroom/k8s-blue
- team: https://github.com/orgs/mincong-classroom/teams/blue
`,
			},
		},
		{
			name:       "dry run",
			team:       "blue",
			args:       []string{"--dry-run"},
			stdin:      "y\n",
			wantOutput: []string{"(dry run) Create the private repository", `Dry run: team "blue" not provisioned, nothing changed on GitHub.`},
		},
		{
			name:       "no by default",
			team:       "blue",
			stdin:      "\n",
			wantOutput: []string{"Provision the team blue? (y/N): \nTeam \"blue\" not provisioned.\n"},
		},
		{
			name: "a team already provisioned",
			team: "red",
			wantOutput: []string{
				`  - SMITH John (@jsmith): "John Smith" on GitHub, already a member`,
				"  - DOE Jane (@jdoe): no display name on GitHub, already invited",
				`Team "red" is already provisioned.`,
			},
		},
		{
			name:        "the admin access is replaced by push",
			team:        "green",
			stdin:       "\ny\n", // No members picked yet
			wantChanges: []string{grantGreen},
		},
		{
			name:         "an invalid new team name",
			team:         "Purple",
			wantExitCode: 1,
			wantOutput:   []string{`✗ invalid name "Purple"`},
		},
		{
			name:         "an invalid team is not provisioned",
			team:         "Pink-2",
			wantExitCode: 1,
			wantOutput:   []string{`✗ invalid name "Pink-2"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)

			r := e.run(t, tt.stdin, append([]string{"team", tt.team, "provision"}, tt.args...)...)

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

func TestTeamFile(t *testing.T) {
	e := newEnv(t)
	teamFile, err := filepath.Abs("testdata/test-teams-2026.yaml")
	if err != nil {
		t.Fatal(err)
	}

	r := e.run(t, "", "team", "ls", "--json", "--team-file", teamFile)
	want := lsJSON{
		Year: 2026,
		Teams: []teamJSON{{
			Name:     "yellow",
			Repo:     "k8s-yellow",
			Members:  []memberJSON{{Name: "NEWCOMER Sam", Github: "snewcomer", GithubName: "Sam Newcomer"}},
			Status:   &statusJSON{},
			Warnings: []string{"NEWCOMER Sam is not among the students of the registry"},
		}},
		StudentsNotInTeam: []string{"DURAND Camille"},
	}
	if got := decode[lsJSON](t, r); !reflect.DeepEqual(got, want) {
		t.Errorf("mc team ls --json --team-file =\n%+v\nwant\n%+v", got, want)
	}

	// The team subcommands come from the test registry, wherever the flag is.
	r = e.run(t, "", "--team-file="+teamFile, "team", "yellow", "status")
	if want := "✗ repo k8s-yellow · ✗ team yellow · ✗ no access · @snewcomer not invited\n"; r.stdout != want {
		t.Errorf("mc team yellow status = %q, want %q", r.stdout, want)
	}
	r = e.run(t, "", "team", "red", "status", "--team-file", teamFile)
	if r.exitCode != 1 || !strings.Contains(r.stderr, `unknown team or action "red"`) {
		t.Errorf("mc team red status with the test registry: exit code %d, stderr:\n%s", r.exitCode, r.stderr)
	}

	// NEWCOMER is not among the students: a warning, the team is provisioned once confirmed.
	r = e.run(t, "n\n", "team", "yellow", "provision", "--team-file", teamFile)
	if got := e.ghChanges(t); got != nil {
		t.Errorf("changes after declining = %q, want none", got)
	}
	if !strings.Contains(r.stdout, "⚠ NEWCOMER Sam is not among the students of the registry\nProvision the team yellow?") {
		t.Errorf("mc team yellow provision does not show the warning before the confirmation:\n%s", r.stdout)
	}
	r = e.run(t, "y\n", "team", "yellow", "provision", "--team-file", teamFile)
	wantChanges := []string{
		"gh repo create mincong-classroom/k8s-yellow --private --template mincong-classroom/containers",
		"gh api -X POST orgs/mincong-classroom/teams -f name=yellow -f privacy=secret",
		"gh api -X PUT orgs/mincong-classroom/teams/yellow/repos/mincong-classroom/k8s-yellow -f permission=push",
		"gh api -X PUT orgs/mincong-classroom/teams/yellow/memberships/snewcomer -f role=member",
	}
	if got := e.ghChanges(t); !reflect.DeepEqual(got, wantChanges) {
		t.Errorf("changes =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(wantChanges, "\n"))
	}

	r = e.run(t, "", "team", "ls", "--team-file", "/nonexistent/teams.yaml")
	if r.exitCode != 1 || !strings.Contains(r.stderr, "failed to list teams") {
		t.Errorf("mc team ls with a missing team file: exit code %d, stderr:\n%s", r.exitCode, r.stderr)
	}
}

func TestTeamProvisionRegistersANewTeam(t *testing.T) {
	e := newEnv(t)
	registryFile := filepath.Join(e.home, ".mc", "teams-2026.yaml")

	// DURAND is the only student not in a team yet: number 1.
	r := e.run(t, "1\ncdurand\ny\n", "team", "purple", "provision")

	if r.exitCode != 0 {
		t.Fatalf("exit code %d, stderr:\n%s", r.exitCode, r.stderr)
	}
	for _, want := range []string{
		"The team purple is not in the registry yet.\n",
		`Students not in a team yet:
   1. DURAND Camille
`,
		`  - DURAND Camille (@cdurand): "Camille Durand" on GitHub
Provision the team purple? (y/N): ✓ the team purple saved to ` + registryFile,
		"\nTeam \"purple\" provisioned.\n",
	} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("output does not contain %q:\n%s", want, r.stdout)
		}
	}
	wantChanges := []string{
		"gh repo create mincong-classroom/k8s-purple --private --template mincong-classroom/containers",
		"gh api -X POST orgs/mincong-classroom/teams -f name=purple -f privacy=secret",
		"gh api -X PUT orgs/mincong-classroom/teams/purple/repos/mincong-classroom/k8s-purple -f permission=push",
		"gh api -X PUT orgs/mincong-classroom/teams/purple/memberships/cdurand -f role=member",
	}
	if got := e.ghChanges(t); !reflect.DeepEqual(got, wantChanges) {
		t.Errorf("changes =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(wantChanges, "\n"))
	}

	data, err := os.ReadFile(registryFile)
	if err != nil {
		t.Fatal(err)
	}
	wantEnd := `  - name: purple
    members:
      - name: "DURAND Camille"
        github: cdurand
`
	if !strings.HasSuffix(string(data), wantEnd) || !strings.Contains(string(data), "# Created before the course, no members yet.") {
		t.Errorf("registry does not end with the new team, or lost its comments:\n%s", data)
	}

	// The new team is a team like the others.
	ls := decode[lsJSON](t, e.run(t, "", "team", "ls", "--json"))
	if names := len(ls.Teams); names != 5 || ls.Teams[4].Name != "purple" || len(ls.StudentsNotInTeam) != 0 {
		t.Errorf("mc team ls --json after the registration = %+v", ls)
	}
	if r := e.run(t, "", "team", "purple", "status"); r.exitCode != 0 {
		t.Errorf("mc team purple status: exit code %d, stderr:\n%s", r.exitCode, r.stderr)
	}
}

func TestTeamProvisionDryRunDoesNotRegister(t *testing.T) {
	e := newEnv(t)
	registryFile := filepath.Join(e.home, ".mc", "teams-2026.yaml")
	before, err := os.ReadFile(registryFile)
	if err != nil {
		t.Fatal(err)
	}

	r := e.run(t, "1\ncdurand\ny\n", "team", "purple", "provision", "--dry-run")

	if !strings.Contains(r.stdout, "(dry run) the team purple not saved to the registry") {
		t.Errorf("output:\n%s", r.stdout)
	}
	if after, _ := os.ReadFile(registryFile); !bytes.Equal(after, before) {
		t.Errorf("the registry changed in a dry run:\n%s", after)
	}
	if got := e.ghChanges(t); got != nil {
		t.Errorf("changes in a dry run = %q", got)
	}
}

func TestTeamProvisionDeclinedDoesNotRegister(t *testing.T) {
	e := newEnv(t)
	registryFile := filepath.Join(e.home, ".mc", "teams-2026.yaml")
	before, err := os.ReadFile(registryFile)
	if err != nil {
		t.Fatal(err)
	}

	r := e.run(t, "1\ncdurand\nn\n", "team", "purple", "provision")

	if r.exitCode != 0 || !strings.Contains(r.stdout, "\nTeam \"purple\" not provisioned.\n") {
		t.Errorf("exit code %d, stdout:\n%s\nstderr:\n%s", r.exitCode, r.stdout, r.stderr)
	}
	if after, _ := os.ReadFile(registryFile); !bytes.Equal(after, before) {
		t.Errorf("the registry changed although the team was declined:\n%s", after)
	}
	if got := e.ghChanges(t); got != nil {
		t.Errorf("changes after declining = %q", got)
	}
}

func TestTeamProvisionAddsTheMembersOfARegisteredTeam(t *testing.T) {
	e := newEnv(t)
	registryFile := filepath.Join(e.home, ".mc", "teams-2026.yaml")

	// green is registered without members; DURAND is the only student not in a team yet.
	r := e.run(t, "1\ncdurand\ny\n", "team", "green", "provision")

	if r.exitCode != 0 || !strings.Contains(r.stdout, "The team green has no members yet.\n") {
		t.Fatalf("exit code %d, stdout:\n%s\nstderr:\n%s", r.exitCode, r.stdout, r.stderr)
	}
	wantChanges := []string{
		"gh api -X PUT orgs/mincong-classroom/teams/green/repos/mincong-classroom/k8s-green -f permission=push",
		"gh api -X PUT orgs/mincong-classroom/teams/green/memberships/cdurand -f role=member",
	}
	if got := e.ghChanges(t); !reflect.DeepEqual(got, wantChanges) {
		t.Errorf("changes =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(wantChanges, "\n"))
	}
	data, err := os.ReadFile(registryFile)
	if err != nil {
		t.Fatal(err)
	}
	want := `  # Created before the course, no members yet.
  - name: green
    members:
      - name: "DURAND Camille"
        github: cdurand
`
	if !strings.Contains(string(data), want) {
		t.Errorf("registry does not have the members of green:\n%s", data)
	}
}

func TestTeamRegistryWithAnUnknownKey(t *testing.T) {
	e := newEnv(t)
	teamFile := filepath.Join(t.TempDir(), "teams.yaml")
	registry := `student:
  - name: "DURAND Camille"
teams: []
`
	if err := os.WriteFile(teamFile, []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}

	r := e.run(t, "", "team", "ls", "--team-file", teamFile)

	if r.exitCode != 1 || !strings.Contains(r.stderr, `unknown key "student"`) {
		t.Errorf("exit code %d, stderr:\n%s", r.exitCode, r.stderr)
	}
}

func TestInfoPrintsTheEvaluationSheet(t *testing.T) {
	e := newEnv(t)
	r := e.run(t, "", "info")
	if !strings.Contains(r.stdout, "Registered Teams: 4\nEvaluation Sheet: none\n") {
		t.Errorf("mc info without an evaluation sheet, stdout:\n%s", r.stdout)
	}

	teamFile := filepath.Join(t.TempDir(), "teams.yaml")
	registry := `evaluation_sheet: https://docs.google.com/spreadsheets/d/0123456789/edit
teams:
  - name: red
    members: []
`
	if err := os.WriteFile(teamFile, []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}

	r = e.run(t, "", "info", "--team-file", teamFile)
	if want := "Evaluation Sheet: https://docs.google.com/spreadsheets/d/0123456789/edit\n"; r.exitCode != 0 || !strings.Contains(r.stdout, want) {
		t.Errorf("mc info: exit code %d, stdout:\n%s\nstderr:\n%s", r.exitCode, r.stdout, r.stderr)
	}
	// The other commands accept the key too.
	r = e.run(t, "", "team", "ls", "--team-file", teamFile)
	if r.exitCode != 0 {
		t.Errorf("mc team ls: exit code %d, stderr:\n%s", r.exitCode, r.stderr)
	}
}

// repoPath returns where mc finds the repository of the team: under HOME.
func (e *env) repoPath(team string) string {
	return filepath.Join(e.home, "github", "mincong-classroom", "k8s-"+team)
}

// git runs git in the directory dir, with the HOME of e and an identity for the commits.
func (e *env) git(t *testing.T, dir string, args ...string) {
	t.Helper()
	args = append([]string{"-C", dir, "-c", "user.name=mc", "-c", "user.email=mc@example.org", "-c", "commit.gpgsign=false"}, args...)
	git := exec.Command("git", args...)
	git.Env = append(os.Environ(), "HOME="+e.home)
	if out, err := git.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// initRepo creates a repository in dir, on the branch main, whose first commit adds a README.md
// with the content readme.
func (e *env) initRepo(t *testing.T, dir, readme string) {
	t.Helper()
	e.git(t, e.home, "init", "--quiet", "--initial-branch=main", dir)
	writeREADME(t, dir, readme)
	e.git(t, dir, "add", "README.md")
	e.git(t, dir, "commit", "--quiet", "-m", "Init")
}

// initTeamRepo creates the repository of the team where mc git clone clones it, under HOME, with
// one commit "Init" of a README.
func (e *env) initTeamRepo(t *testing.T, team string) {
	t.Helper()
	e.initRepo(t, e.repoPath(team), "The repository of the team "+team+"\n")
}

func writeREADME(t *testing.T, repo, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readREADME(t *testing.T, repo string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestGitShowReadsTheRepoUnderHome(t *testing.T) {
	e := newEnv(t)
	e.initTeamRepo(t, "red")

	r := e.run(t, "", "git", "show", "HEAD:README.md")

	if !strings.Contains(r.stdout, "The repository of the team red\n") {
		t.Errorf("mc git show does not show the README of k8s-red under HOME:\n%s", r.stdout)
	}
}

// cloneRedRepo creates the repository of the team red, with a README.md "v1", and the teacher's
// clone of it, under HOME. It returns the paths of both.
func (e *env) cloneRedRepo(t *testing.T) (team, clone string) {
	t.Helper()
	team = filepath.Join(t.TempDir(), "k8s-red")
	clone = e.repoPath("red")
	e.initRepo(t, team, "v1\n")
	e.git(t, e.home, "clone", "--quiet", team, clone)
	return team, clone
}

func TestGitPullFastForwards(t *testing.T) {
	e := newEnv(t)
	team, clone := e.cloneRedRepo(t)
	writeREADME(t, team, "v2\n")
	e.git(t, team, "commit", "--quiet", "-am", "Lab 1")

	r := e.run(t, "", "git", "pull")

	if got := readREADME(t, clone); got != "v2\n" {
		t.Errorf("README.md of the clone after mc git pull = %q, want %q\nstdout:\n%s\nstderr:\n%s", got, "v2\n", r.stdout, r.stderr)
	}
	if strings.Contains(r.stdout, "history rewritten") || strings.Contains(r.stderr, "team red") {
		t.Errorf("mc git pull reports a problem for red:\nstdout:\n%s\nstderr:\n%s", r.stdout, r.stderr)
	}
}

func TestGitPullWarnsOfARewrittenHistory(t *testing.T) {
	e := newEnv(t)
	team, clone := e.cloneRedRepo(t)
	// The team rewrites the commit that the teacher already has, as a force-push would.
	writeREADME(t, team, "v2\n")
	e.git(t, team, "commit", "--quiet", "--amend", "-am", "Init, rewritten")

	r := e.run(t, "", "git", "pull")

	if got := readREADME(t, clone); got != "v1\n" {
		t.Errorf("README.md of the clone after mc git pull = %q, want %q: the rewritten history is merged", got, "v1\n")
	}
	warning := "[team: red] ⚠ history rewritten: force-push?"
	if !strings.Contains(r.stdout, warning) {
		t.Errorf("mc git pull: missing the warning %q in:\n%s", warning, r.stdout)
	}
	if strings.Contains(r.stderr, "team red") {
		t.Errorf("mc git pull reports a rewritten history as a failure:\n%s", r.stderr)
	}
	// The next teams are pulled: they have no clone here, so they fail.
	if !strings.Contains(r.stderr, "Failed to pull latest changes for team blue") {
		t.Errorf("mc git pull stops after the warning:\nstdout:\n%s\nstderr:\n%s", r.stdout, r.stderr)
	}
}

func TestGitExec(t *testing.T) {
	e := newEnv(t)
	e.initTeamRepo(t, "red")
	e.initTeamRepo(t, "green")

	r := e.run(t, "", "git", "exec", "--", "log", "--format=%s")

	// The repositories of blue and Pink-2 are not cloned: git fails for them, but goes on with the
	// next teams.
	output := regexp.MustCompile(`(?s)\[team: red\][^\n]* log --format=%s[^\n]*\nInit\n.*\[team: blue\].*\[team: green\][^\n]*\nInit\n.*\[team: Pink-2\]`)
	if !output.MatchString(r.stdout) {
		t.Errorf("mc git exec does not run git for each team, in order:\n%s", r.stdout)
	}
	wantErr := "git failed for 2 of 4 teams: blue (exit status 128), Pink-2 (exit status 128)"
	if r.exitCode != 1 || !strings.Contains(r.stderr, wantErr) {
		t.Errorf("mc git exec with 2 teams not cloned: exit code %d, stderr:\n%s\nwant exit code 1 and %q", r.exitCode, r.stderr, wantErr)
	}
}

func TestGitExecSucceedsForEveryTeam(t *testing.T) {
	e := newEnv(t)
	e.initTeamRepo(t, "green")
	teamFile := filepath.Join(t.TempDir(), "teams.yaml")
	registry := `teams:
  - name: green
    members: []
`
	if err := os.WriteFile(teamFile, []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}

	r := e.run(t, "", "git", "exec", "--team-file", teamFile, "--", "show", "HEAD:README.md")

	if r.exitCode != 0 || r.stderr != "" || !strings.Contains(r.stdout, "[team: green]") || !strings.Contains(r.stdout, "The repository of the team green\n") {
		t.Errorf("mc git exec: exit code %d, stdout:\n%s\nstderr:\n%s", r.exitCode, r.stdout, r.stderr)
	}
}

func TestGitExecWithoutArguments(t *testing.T) {
	r := newEnv(t).run(t, "", "git", "exec")

	if r.exitCode != 1 || !strings.Contains(r.stderr, "requires at least 1 arg") {
		t.Errorf("mc git exec without arguments: exit code %d, stderr:\n%s", r.exitCode, r.stderr)
	}
}

// writeFile writes a file in the HOME of the env, creating its directory.
func (e *env) writeFile(t *testing.T, name, content string) {
	t.Helper()
	path := filepath.Join(e.home, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// testRules are the made-up rules of the tests of the package rubric.
func testRules(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../rubric/testdata/evaluation-rules.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestRuleLsReadsTheRulesOfTheYearOfTheSettings(t *testing.T) {
	e := newEnv(t)
	e.writeFile(t, ".mc/settings.yaml", "year: 2030\nrules: ~/course/{year}/evaluation-rules.yaml\n")
	e.writeFile(t, "course/2030/evaluation-rules.yaml", testRules(t))
	e.vars = []string{"MC_YEAR="} // No year from the environment: the one of the settings.

	r := e.run(t, "", "rule", "ls", "-l", "L1")
	want := "RULE    EXERCISE  COEFFICIENT  NAME\nL1_BLD  1         0.6          Build Test\nL1_TAG  2         0.4          Publish Test\n"
	if r.exitCode != 0 || r.stdout != want {
		t.Errorf("mc rule ls -l L1 = %d, %q, stderr %q, want 0, %q", r.exitCode, r.stdout, r.stderr, want)
	}
}

func TestRuleLsReadsTheRulesFile(t *testing.T) {
	e := newEnv(t)
	e.writeFile(t, "rules.yaml", testRules(t))

	r := e.run(t, "", "rule", "ls", "--rules-file", filepath.Join(e.home, "rules.yaml"), "-l", "2")
	if r.exitCode != 0 || !strings.Contains(r.stdout, "L2_LOG  2.1       0.1          Logs Test, bonus") {
		t.Errorf("mc rule ls --rules-file = %d, %q, stderr %q, want the rules of L2", r.exitCode, r.stdout, r.stderr)
	}
}

func TestRuleLsWithoutRules(t *testing.T) {
	e := newEnv(t)

	r := e.run(t, "", "rule", "ls")
	if r.exitCode != 1 || !strings.Contains(r.stderr, "no evaluation rules: set the key rules in "+filepath.Join(e.home, ".mc", "settings.yaml")) {
		t.Errorf("mc rule ls without settings = %d, stderr %q, want 1 and the key to set", r.exitCode, r.stderr)
	}
}

func TestBrokenSettingsStopEveryCommand(t *testing.T) {
	e := newEnv(t)
	e.writeFile(t, ".mc/settings.yaml", "yaer: 2030\n")

	r := e.run(t, "", "info")
	if r.exitCode != 1 || !strings.Contains(r.stderr, "field yaer not found") || r.stdout != "" {
		t.Errorf("mc info with a typo in the settings = %d, %q, stderr %q, want 1 and the typo", r.exitCode, r.stdout, r.stderr)
	}
}

// evaluationOfTeal is a valid evaluation of the made-up Lab 2 of the rules of the tests.
const evaluationOfTeal = `team: teal
commit: 0a1b2c3
time_spent: Session only, done at 11:21
action_items: Run it yourself
rules:
  L2_POD:
    completeness: 0.8
    done_well: The Pod runs the image of Lab 1.
    to_improve: The report doesn't show the logs of the Pod.
    to_check: ""
    analysis: none
    writing_style: own words
    penalty: null
`

// writeEvaluations writes the evaluations of Lab 2 of the teams, each one a copy of the one of
// teal, the facts of mc evidence for the teams with a commit, and the rules and a registry of
// teal and coral. It returns the arguments of mc for the rules and the registry.
func (e *env) writeEvaluations(t *testing.T, teams []string, commits map[string]string) []string {
	t.Helper()
	for _, team := range teams {
		e.writeFile(t, ".mc/2026/evaluations/lab-2/"+team+".yaml", strings.Replace(evaluationOfTeal, "team: teal", "team: "+team, 1))
	}
	for team, commit := range commits {
		e.writeFile(t, ".mc/2026/teacher/k8s-"+team+"/docs/lab-2.evidence.md", "---\ncommit: "+commit+"\nmc: 949d7aa740ab\n---\n\n# Lab 2\n")
	}
	e.writeFile(t, "rules.yaml", testRules(t))
	registry := `teams:
  - name: teal
    members: []
  - name: coral
    members: []
`
	e.writeFile(t, "teams.yaml", registry)
	return []string{"--rules-file", filepath.Join(e.home, "rules.yaml"), "--team-file", filepath.Join(e.home, "teams.yaml")}
}

func TestEvalLs(t *testing.T) {
	e := newEnv(t)
	flags := e.writeEvaluations(t, []string{"teal"}, nil)
	// coral: a rule to check, a penalty and the optional rule; olive: a key that doesn't exist.
	e.writeFile(t, ".mc/2026/evaluations/lab-2/coral.yaml", `team: coral
commit: 0a1b2c3d4e5f60718293a4b5c6d7e8f901234567
action_items: Talk with them
rules:
  L2_POD:
    completeness: 1
    to_check: The logs come from another Pod.
    penalty: {kind: secret, remove: 0.3, why: The report shows a token.}
  L2_LOG:
    completeness: 0.5
`)
	e.writeFile(t, ".mc/2026/evaluations/lab-2/olive.yaml", "team: olive\nrules:\n  L2_POD:\n    to_chek: \"\"\n")

	r := e.run(t, "", append([]string{"eval", "ls"}, flags...)...)

	dir := filepath.Join(e.home, ".mc", "2026", "evaluations")
	want := "LAB  TEAM   COMMIT   RULES   TO CHECK  PENALTIES  ACTION ITEMS\n" +
		"L2   coral  0a1b2c3  1/1 +1  1         1          Talk with them\n" +
		"L2   olive  -        -       -         -          -\n" +
		"L2   teal   0a1b2c3  1/1     0         0          Run it yourself\n"
	wantStderr := filepath.Join(dir, "lab-2", "olive.yaml") + ": 1 problem, see mc eval run -l L2 -t olive\n"
	if r.exitCode != 0 || r.stdout != want || r.stderr != wantStderr {
		t.Errorf("mc eval ls = %d, stdout\n%s\nstderr %q, want 0, stdout\n%s\nstderr %q", r.exitCode, r.stdout, r.stderr, want, wantStderr)
	}

	// A lab without evaluations.
	r = e.run(t, "", append([]string{"eval", "ls", "-l", "1"}, flags...)...)
	if want := "No evaluation in " + filepath.Join(dir, "lab-1") + "\n"; r.exitCode != 0 || r.stdout != want {
		t.Errorf("mc eval ls -l 1 = %d, %q, stderr %q, want 0, %q", r.exitCode, r.stdout, r.stderr, want)
	}
}

func TestEvalRun(t *testing.T) {
	e := newEnv(t)
	flags := e.writeEvaluations(t, []string{"teal", "coral"}, map[string]string{
		"teal":  "0a1b2c3d4e5f60718293a4b5c6d7e8f901234567",
		"coral": "0a1b2c3ffffffffffffffffffffffffffffffff",
	})

	r := e.run(t, "", append([]string{"eval", "run", "-l", "L2", "--check-only"}, flags...)...)

	want := "2 files of L2 checked in " + filepath.Join(e.home, ".mc", "2026", "evaluations", "lab-2") + ": no problem\n"
	if r.exitCode != 0 || r.stdout != want || r.stderr != "" {
		t.Errorf("mc eval run -l L2 = %d, %q, stderr %q, want 0, %q", r.exitCode, r.stdout, r.stderr, want)
	}
}

func TestEvalRunStartsTheAgent(t *testing.T) {
	e := newEnv(t)
	flags := e.writeEvaluations(t, []string{"teal"}, map[string]string{"teal": "0a1b2c3d4e5f60718293a4b5c6d7e8f901234567"})
	// The agent writes its arguments in its directory, the one of the rules, outside of Git here.
	e.writeFile(t, ".mc/settings.yaml", "agents:\n  eval_run: echo \"{lab} {teams} $MC_EVAL_AGENT\" >> agent.log\n")
	log := filepath.Join(e.home, "agent.log")

	r := e.run(t, "", append([]string{"eval", "run", "-l", "2"}, flags...)...)
	if r.exitCode != 0 || !strings.Contains(r.stdout, "1 file of L2 checked") {
		t.Errorf("mc eval run -l 2 = %d, %q, stderr %q, want 0 and the checks", r.exitCode, r.stdout, r.stderr)
	}
	r = e.run(t, "", append([]string{"eval", "run", "-l", "2", "-t", "teal", "-t", "coral"}, flags...)...)
	if r.exitCode != 1 || !strings.Contains(r.stderr, "coral.yaml: no such file") {
		t.Errorf("mc eval run -t teal -t coral = %d, stderr %q, want 1 and the missing file of coral", r.exitCode, r.stderr)
	}
	if data, err := os.ReadFile(log); err != nil || string(data) != "L2 all 1\nL2 teal coral 1\n" {
		t.Errorf("the arguments of the agent = %q, %v, want L2 all, then L2 teal coral", data, err)
	}
}

func TestEvalRunWithoutAgent(t *testing.T) {
	e := newEnv(t)
	flags := e.writeEvaluations(t, []string{"teal"}, nil)

	r := e.run(t, "", append([]string{"eval", "run", "-l", "2"}, flags...)...)
	if r.exitCode != 1 || !strings.Contains(r.stderr, "no agent to evaluate the lab: set agents.eval_run in "+filepath.Join(e.home, ".mc", "settings.yaml")) {
		t.Errorf("mc eval run without an agent = %d, stderr %q, want 1 and the key to set", r.exitCode, r.stderr)
	}

	// From an agent: it checks its files, and never starts another agent.
	e.writeFile(t, ".mc/settings.yaml", "agents:\n  eval_run: touch started\n")
	e.vars = []string{"MC_EVAL_AGENT=1"}
	r = e.run(t, "", append([]string{"eval", "run", "-l", "2"}, flags...)...)
	if r.exitCode != 1 || !strings.Contains(r.stderr, "can't start another one") {
		t.Errorf("mc eval run from an agent = %d, stderr %q, want 1", r.exitCode, r.stderr)
	}
	if _, err := os.Stat(filepath.Join(e.home, "started")); err == nil {
		t.Error("mc eval run from an agent started another agent")
	}

	// A failed agent: no check of files that it may have left half written.
	e.vars = nil
	e.writeFile(t, ".mc/settings.yaml", "agents:\n  eval_run: exit 3\n")
	r = e.run(t, "", append([]string{"eval", "run", "-l", "2"}, flags...)...)
	if r.exitCode != 1 || !strings.Contains(r.stderr, "the agent failed, so its files are not checked: exit status 3") || r.stdout != "" {
		t.Errorf("mc eval run with a failed agent = %d, %q, stderr %q, want 1 and no check", r.exitCode, r.stdout, r.stderr)
	}
}

func TestEvalRunPrintsTheProblems(t *testing.T) {
	e := newEnv(t)
	flags := e.writeEvaluations(t, []string{"teal", "coral", "olive"}, map[string]string{
		"teal":  "0a1b2c3d4e5f60718293a4b5c6d7e8f901234567",
		"coral": "9f8e7d6c5b4a39281706f5e4d3c2b1a098765432",
	})
	dir := filepath.Join(e.home, ".mc", "2026", "evaluations", "lab-2")
	coral := filepath.Join(dir, "coral.yaml")
	data, err := os.ReadFile(coral)
	if err != nil {
		t.Fatal(err)
	}
	e.writeFile(t, ".mc/2026/evaluations/lab-2/coral.yaml", strings.Replace(string(data), "completeness: 0.8", "completeness: 0.85", 1))

	r := e.run(t, "", append([]string{"eval", "run", "-l", "2", "--check-only"}, flags...)...)

	// One problem per line, sorted by file, then by line. olive has no facts, and no team.
	want := coral + ":2: the commit 0a1b2c3 is not the one of the facts of mc evidence, 9f8e7d6: evaluate that commit, or gather the facts again\n" +
		coral + ":7: L2_POD: the completeness 0.85 is not from 0 to 1 by steps of 0.1\n" +
		filepath.Join(dir, "olive.yaml") + `: the team "olive" is not in the registry ` + filepath.Join(e.home, "teams.yaml") + "\n" +
		filepath.Join(dir, "olive.yaml") + ":2: no facts of mc evidence for the team olive in L2: run mc evidence -l L2 -t olive first\n" +
		"Error: 4 problems in 2 of 3 files\n"
	if r.exitCode != 1 || r.stdout != "" || r.stderr != want {
		t.Errorf("mc eval run -l 2 = %d, %q, stderr\n%s\nwant 1 and the stderr\n%s", r.exitCode, r.stdout, r.stderr, want)
	}

	// One team only: the problems of the others don't count.
	r = e.run(t, "", append([]string{"eval", "run", "-l", "2", "-t", "teal", "--check-only"}, flags...)...)
	if want := "1 file of L2 checked in " + dir + ": no problem\n"; r.exitCode != 0 || r.stdout != want {
		t.Errorf("mc eval run -l 2 -t teal = %d, %q, stderr %q, want 0, %q", r.exitCode, r.stdout, r.stderr, want)
	}
}

func TestEvalRunWithoutEvaluations(t *testing.T) {
	e := newEnv(t)
	flags := e.writeEvaluations(t, nil, nil)
	dir := filepath.Join(t.TempDir(), "evaluations")

	r := e.run(t, "", append([]string{"eval", "run", "-l", "L2", "--dir", dir, "--check-only"}, flags...)...)

	want := "no evaluation of L2 in " + filepath.Join(dir, "lab-2") + ": one YAML file per team, such as " + filepath.Join(dir, "lab-2", "teal.yaml")
	if r.exitCode != 1 || !strings.Contains(r.stderr, want) {
		t.Errorf("mc eval run --dir of no file = %d, stderr %q, want 1 and %q", r.exitCode, r.stderr, want)
	}

	// A team asked for, without its file.
	r = e.run(t, "", append([]string{"eval", "run", "-l", "L2", "--dir", dir, "-t", "teal", "--check-only"}, flags...)...)
	want = filepath.Join(dir, "lab-2", "teal.yaml") + ": no such file: the evaluation of the team teal\n"
	if r.exitCode != 1 || !strings.HasPrefix(r.stderr, want) {
		t.Errorf("mc eval run -t teal without its file = %d, stderr %q, want 1 and %q", r.exitCode, r.stderr, want)
	}
}

// tabRead is the tab Evaluation as the script of mc eval sync reads it: a row of teal in Lab 1,
// and one in Lab 2 without a completeness.
const tabRead = `{
  "tab": "Evaluation",
  "keys": ["Team", "Lab", "Rule"],
  "columns": {"A": "Team", "D": "Lab", "E": "Rule", "F": "Rule Score", "G": "Completeness", "H": "Score", "I": "Comment",
    "J": "Troubleshooting Instructions", "K": "TA To Check", "L": "TA Commit", "M": "TA Docker Images", "N": "TA Screenshots",
    "O": "TA Analysis", "P": "TA Writing Style", "Q": "TA Time Spent", "R": "TA Lab Flag", "S": "TA Penalty",
    "T": "TA Action Items", "U": "TA Comment"},
  "rows": [
    {"_row": 2, "Team": "teal", "Lab": "L1", "Rule": "L1_BLD", "Completeness": "1"},
    {"_row": 3, "Team": "teal", "Lab": "L2", "Rule": "L2_POD", "Rule Score": "10", "TA Analysis": "none"}
  ]
}
`

func TestEvalSync(t *testing.T) {
	e := newEnv(t)
	flags := e.writeEvaluations(t, []string{"teal", "coral"}, map[string]string{
		"teal":  "0a1b2c3d4e5f60718293a4b5c6d7e8f901234567",
		"coral": "0a1b2c3ffffffffffffffffffffffffffffffff",
	})
	dir := filepath.Join(e.home, ".mc", "2026", "sheet")
	sync := append([]string{"eval", "sync", "-l", "L2"}, flags...)

	// Without a read of the tab: the script that reads it, then a stop.
	r := e.run(t, "", sync...)
	script := filepath.Join(dir, "evaluation.read.js")
	if r.exitCode != 1 || !strings.Contains(r.stdout, "No read of the tab Evaluation since the last export") ||
		!strings.Contains(r.stdout, "\n  "+script+"\n") || !strings.Contains(r.stderr, "read the tab first") {
		t.Errorf("mc eval sync without a read = %d, stdout %q, stderr %q, want 1 and the script", r.exitCode, r.stdout, r.stderr)
	}
	data, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	if s := string(data); !strings.Contains(s, `"I": "Comment"`) || !strings.Contains(s, `"U": "TA Comment"`) || strings.Contains(s, `"B"`) || strings.Contains(s, `"C"`) {
		t.Errorf("the script reads other columns than A and D to U:\n%s", s)
	}

	// With a read: teal L2_POD is a row of the tab, and coral L2_POD a new row after the last one.
	e.writeFile(t, ".mc/2026/sheet/evaluation.read.json", tabRead)
	r = e.run(t, "", sync...)
	pending := filepath.Join(dir, "evaluation.json")
	want := "Synced 2 evaluations of L2 with the tab Evaluation, as read in " + filepath.Join(dir, "evaluation.read.json") + ".\n\n" +
		"CELL           ROW             COLUMN            CHANGE\n" +
		"Evaluation!G3  teal L2 L2_POD  Completeness      nothing -> 0.8\n" +
		"Evaluation!U3  teal L2 L2_POD  TA Comment        nothing -> \"Done well: The Pod runs the image of Lab...\"\n" +
		"Evaluation!L3  teal L2 L2_POD  TA Commit         nothing -> \"0a1b2c3\"\n" +
		"Evaluation!P3  teal L2 L2_POD  TA Writing Style  nothing -> \"own words\"\n" +
		"Evaluation!Q3  teal L2 L2_POD  TA Time Spent     nothing -> \"Session only, done at 11:21\"\n" +
		"Evaluation!T3  teal L2 L2_POD  TA Action Items   nothing -> \"Run it yourself\"\n\n" +
		"New 1 row: coral, row 4.\n\n" +
		"2 rows wait in " + pending + ". Next, in the page of the spreadsheet:\n\n" +
		"  mc sheet blocks evaluation  list the blocks to paste\n" +
		"  mc sheet copy evaluation 1  copy a block, then paste it at its range, block after block\n" +
		"  fill down B3:C4             the columns B and C of the new rows: type the range in the Name Box, then Cmd+D\n" +
		"  mc sheet pasted evaluation  record the blocks pasted, such as 1 2 3, or every block\n" +
		"  mc sheet verify evaluation  after a new read of the tab, compare it with what was written\n"
	if r.exitCode != 0 || r.stdout != want || r.stderr != "" {
		t.Errorf("mc eval sync = %d, stdout\n%s\nstderr %q, want 0, stdout\n%s", r.exitCode, r.stdout, r.stderr, want)
	}

	// The blocks write the keys of the new row, and those of the row of the tab are left alone.
	r = e.run(t, "", "sheet", "blocks", "evaluation")
	if r.exitCode != 0 || !strings.Contains(r.stdout, "Evaluation!A4 ") || !strings.Contains(r.stdout, "Evaluation!D4:F4 ") || strings.Contains(r.stdout, "Evaluation!A3") {
		t.Errorf("mc sheet blocks evaluation = %d, stdout\n%s\nstderr %q, want the keys of the row 4 only", r.exitCode, r.stdout, r.stderr)
	}

	// Again for coral only: its row waits still, replaced, at the same number.
	r = e.run(t, "", append(sync, "-t", "coral")...)
	if r.exitCode != 0 || !strings.Contains(r.stdout, "New 1 row: coral, row 4.\n\nReplaced 1 row of L2 that waited.\n\n2 rows wait in ") {
		t.Errorf("mc eval sync -t coral = %d, stdout\n%s\nstderr %q, want the row 4 again", r.exitCode, r.stdout, r.stderr)
	}
}
