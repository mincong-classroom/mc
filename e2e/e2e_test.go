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
