# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`mc` (Mincong Classroom) is a Cobra-based CLI in Go that auto-grades student assignments for a
"Software Containerization and Orchestration" (Docker/Kubernetes) course. Each year ~25-30 students
form two-person teams; every team has its own Git repository cloned from the same template
([`mincong-classroom/containers`](https://github.com/mincong-classroom/containers)), so all student
repos share an identical file layout (e.g. `apps/spring-petclinic/Dockerfile`, `k8s/lab-2/pod-nginx.yaml`).
The tool exists to run team-specific commands and grading rules in bulk across those dozens of repos.

## Build & run

```sh
go mod tidy
go build -o dist/mc     # binary is gitignored under dist/
./dist/mc help
```

CI (`.github/workflows/mincong-classroom.yaml`, runs on every push) does: `go mod tidy`,
`golangci-lint`, then `go test ./... -v`. Tests exist only for `common/`, `cmd/team/`, `e2e/` and
the Pods of Lab 2 in `rules/` so far; add `_test.go` files alongside the package under test. The team unit tests use an
in-memory `github.Client` (`cmd/team/team_test.go`), so they never call GitHub.

The end-to-end tests (`e2e/`) build the real binary and run it with `HOME` pointing at a copy of
`e2e/testdata/mc` (a made-up registry and students — never real student data, this repo is
public) and a fake `gh` first in `PATH` (`e2e/testdata/bin/gh`, a shell script). The fake serves
`gh api <path>` from `e2e/testdata/github/<path>.json` (HTTP 404 when missing) and appends every
change (`gh repo create`, `gh api -X POST|PUT|DELETE`) to `$FAKE_GH_LOG` instead of making it, so
the tests assert the exact `gh` commands `provision` runs. Assert on `--json` output where
possible. The e2e package blank-imports `mc/cmd` and reads every fixture so that `go test`'s
cache is invalidated by a source or fixture change; add a fixture under `testdata/` and it is
covered automatically.

**Multi-line test data**: write the content of a file (YAML, JSON) or a multi-line expected output
as a Go *raw string literal* (between backquotes), laid out exactly like the real file, not as a
one-line string with `\n` and `\"` escapes:

```go
registry := `teams:
  - name: red
    members: []
`
```

Start the content right after the opening backquote, and put the closing one at the start of the
line after the last line, so that the string holds exactly the file, final newline included. Keep
the escapes for one-line strings, and for the answers typed on stdin in the tests (`"y\nn\n"`).

Key commands (all read the team registry, see "External data" below):

```sh
mc team ls [--json]           # teams, their GitHub status and validation problems, students not in a team
mc team red provision [--dry-run]     # confirms the members once, then creates repo + GitHub team, invites them
mc team red validate|status [--json]  # the actions on one team of the registry
mc rule ls [-l L1]            # the evaluation rules of the year (key rules of ~/.mc/settings.yaml), or --rules-file
mc rule check                 # one problem per line, with its line; exit 1 if any
mc rule render -o evaluation-rules.md  # the Markdown page of the rules
mc evidence -l L1 [-t red] [--late] [--template DIR]  # the evidence report of each team, from the rules of the year
mc info                       # the CLI and the registry: number of teams, URL of the evaluation sheet
mc grade                      # grade all teams, all labs (L1-L5)
mc grade -t red -t blue -l L3 # grade specific teams (-t, repeatable) for one lab (-l L3/3)
mc git clone|show             # bulk git ops over all team repos, e.g. `mc git show main:k8s/lab-2/pod-nginx.yaml`
mc git pull                   # git pull --ff-only; a rewritten history (force-push) is a warning for that team, never merged
mc git exec -- log -1 --oneline  # any git command in each team repo; goes on when one fails, exits 1 if any did
mc k8s create-namespaces      # kubectl create one namespace per team
```

## External data (critical — not in this repo)

Grading input lives **outside the repo** in a private, git-ignored `~/.mc/` directory that you must
assume exists at runtime. Nothing here works without it:

- `~/.mc/settings.yaml` — the settings (`common.Settings`), optional: `year`, and `rules`, the
  YAML of the evaluation rules with `{year}` in it (`common.RulesPath`, replaced by the flag
  `--rules-file`). Read once per run (`sync.OnceValues`), strictly: `cmd.Execute()` stops on an
  unknown key (`CheckSettings`). The year is `common.Year()`: the environment variable `MC_YEAR`,
  then the key `year`, then the current year.
- `~/.mc/teams-{year}.yaml` — the team registry (`TeamRegistry`). Unknown keys (e.g. an old
  `role`) are ignored when read.
  The global flag `--team-file` replaces it for any command (`common.TeamRegistryFile`, a leading
  `~/` is expanded). Since the `mc team <team>` subcommands are built before Cobra parses the
  flags, `cmd.Execute()` reads `--team-file` ahead from `os.Args` (`teamFileFromArgs`).
  The registry's optional `students:` list (`common.Student`, one `name: "LAST First"` each — last name in
  upper case, first name in Pascal case, parsed by `common.ParseName`; `SameName` ignores the case,
  the spaces and the comma of the former `"LAST, First"`) is
  informational: `mc team ls` lists the students not in a team, and a member not among them is a
  validation **warning**. There is no separate student file.
  `validateTeam()` returns **errors** (block `provision`: team name format/reserved/duplicate, a
  member's GitHub username missing or not found) and **warnings** (shown by `provision` above its
  one confirmation: more than 2 members, a member in several teams, without a name, or not among
  the students). For a member, only the GitHub user is enforced.
  The optional top-level `evaluation_sheet:` is the URL of the spreadsheet where the teams of the
  year are evaluated: `mc` never reads the spreadsheet, `mc info` only prints the URL, so that a
  tool can find it without opening the registry, which holds the names of the students.
- `~/.mc/assignments-L1.yaml` … `assignments-L4.yaml` — per-lab, per-team structured data
  (`common.TeamAssignmentL*`), loaded in `rules.NewGrader()`.

Gotcha: `NewGrader()` only reads the **L1–L4** assignment files. `assignmentsL5` is never populated,
so `GradeL5` always reports "team not found in assignments" and grades nothing until an
`assignments-L5.yaml` load is added. Most assignment structs are also empty placeholders today
(`TeamAssignmentL1/L2/L4/L5` have no fields); only L3 (`nginx_pod_name`) carries data.
Still, `GradeL3` skips the teams missing from `assignments-L1.yaml` (the file of Lab 1, not Lab 3),
and `GradeL4` / `GradeL5` those missing from their own file. `GradeL1` and `GradeL2` grade every
team of the registry.

## Architecture

Six packages: `cmd/` (CLI wiring), `common/` (domain types + team registry), `rubric/` (the YAML
of the evaluation rules), `evidence/` (the evidence report of a lab for a team), `rules/` (the
grading engine of 2025, deprecated), `github/` (the GitHub organization, through the `gh` CLI).

**Evaluation rules** (`rubric/`) — the YAML of a program has two maps: `labs` (title, report,
reference, deadline with its offset, intro) and `rules`, keyed by ID, each with its `lab`. `Parse`
decodes it twice: with `KnownFields(true)` into Go maps, so that a key with a typo is an error with
its line (`Problems`), and as a `yaml.Node`, for the order of the keys (a Go map has none) and
their lines (`lines`). `Check` returns the problems of the content, sorted by line; `Render` fills
the embedded `page.md.tmpl` with texts that Go wraps at 96 characters (`wrapper`): a wrapped line
never starts with a marker of Markdown. The tests use the made-up `rubric/testdata/`, never real
rules: update the golden page with `go test ./rubric -update`, then review its diff.

**Evidence** (`evidence/`) — `Gather(team, opts)` writes `docs/lab-{n}.evidence.md` for one team
and one lab: facts, never a score. The commit is the last push to main before the deadline
(`github.CLI.MainPushes`, the activity of the repository, not its events), `origin/main` with
`Late`, or `Rev`. The report goes to the local branch `teacher` (`Branch`) of the clone, checked
out in a worktree of `opts.Dir` (`~/.mc/{year}/teacher/<repo>`): the branch starts from the
commit, the next pass merges the new one (never a reset, never a push), and `commitReport`
commits a change only. `build` reads the tree of the commit with git, not the worktree: the section
of the exercise (`findSection`, `blocks`), without the lines of the statement, which are those of
every version of the report in the template (`LoadTemplate`, or the first commit of the team
without `--template`); the files of the rule and those the report links to (`fileReader`: a short
file is imported, a file of the template shows its smallest diff with one of its versions, a long
one is linked, so that the evaluator quotes its key lines); the files of the evaluator in
`docs/lab-{n}.evidence/` of the worktree (`{ID}.md`, `{lab}-{xyz}-*`); and the images of the rule
on Docker Hub (`DockerHub`, with the later patches of a tag). `markdown()` writes the report. The
test builds a template and a team repository with git in a temporary `HOME` with fixed dates, so
the commits and the golden report `evidence/testdata/lab-1.evidence.md` are the same at each run.

**Team management** (`cmd/team/`, `github/`) — replaces GitHub Classroom. Each team gets a private
repo `k8s-<name>` generated from the template and a secret GitHub team `<name>` with `push` on it.
`github.Client` holds the reads (repo/team exist, team role on the repo, membership state, user);
the changes are `github.Command` values (`CreateRepoCommand`, …) so `provision` can show the exact
`gh` command before running it, and the tests can record them. `mc team <team> provision`
(`provisioner.run(name, load)`) provisions **one team per run** and asks **one** `(y/N)`
question (`confirm()`, "no" by default): it prints the team, each member with their GitHub display
name (and "already a member"/"already invited" from `teamStatus()`), and the `validateTeam()`
warnings, then asks "Provision the team x?". The steps are internal details, never asked:
`stepsToRun()` returns those not done yet (idempotent), and `provision()` runs them one after the
other, stopping at the first failure. A team with `validateTeam()` errors is refused without a
question; a team already provisioned too. `--dry-run` asks the same questions, then prints each
step with its `$ gh …` command, dark yellow when stdout is a terminal and `NO_COLOR` is unset
(`provisioner.color`), and runs nothing. It ends with `Team "x" provisioned.` and
`github.RepoURL`/`TeamURL`. Before the question, `prepare()` gives members to a team that has none:
an unknown name or a registered team without members, name checked first (`teamNameErrors`),
members picked by number among the registry's students not in a team (`askMembers`:
`pickStudents`, `parsePicks`, or `typeMembers` when there is none to pick), GitHub usernames
checked on entry. The registry in memory changes at once, for the validation; the file only once
confirmed (`registryChange`): `common.SetTeamMembers()`, or `common.AddTeam()` for a new team, which
appends it to the registry file through the `yaml.Node` API (the comments survive; `editRegistry`
writes a temp file, then renames it). `LoadRegistry` rejects an unknown top-level key (only
`students`, `teams` and `evaluation_sheet`), so that a typo such as `student:` is not silently ignored. Not saved with
`--dry-run`. The
`cmd/team` unit tests run with a temporary `HOME` (`TestMain`), so they can never write the
teacher's `~/.mc`. `ls` fetches per team concurrently (`forEach`).
The command tree is `mc team ls` and `mc team <team> <action>` (`provision`, `validate`, `status`):
`AddTeamCommands(args)` adds one subcommand per registered team, called from `cmd.Execute()` before
Cobra resolves the args, so the registry is read at startup. For `mc team <team> provision` of a
team not in the registry yet, `teamToProvision(args)` reads the team ahead, wherever the flags are,
and it gets a subcommand too. `ls` and `provision` are reserved team names (`reservedNames`); the
former `mc team provision` returns an error that points to `mc team <team> provision`.

**Team model** (`common/types.go`) — a `Team` has a `Name`, `Members`, and an optional
`CustomRepoName`. There is no role anymore: every team is graded on the same rules, and an old
`role:` key in a registry is ignored. All team-derived paths/URLs come from methods on `Team`
(`GetRepoPath`, `GetKubeNamespace` → `team-<name>`, `GetRepoURL`).
Prefer these helpers over rebuilding paths inline: `mc git clone|pull|show|exec` and the grading rules
all find the team repos at `GetRepoPath()`, `$HOME/github/mincong-classroom/k8s-<name>`.

**Rule abstraction** (`common/types.go`) — every grading check implements
`Rule[T]{ Spec() RuleSpec; Run(team, opts T) RuleEvaluationResult }`. A `RuleSpec` combines
`LabId` + `Symbol` into an id like `L1_DKF` (see `RuleSpec.Id()`); `RuleEvaluationResult.Completeness`
is a float in `[0,1]` (a percentage). There are three flavors of rule:
- **Automated file/cluster checks** (`rules/k8s_pod.go`,
  `k8s_replicaset.go`, `k8s_deployment.go`, `k8s_service.go`, `k8s_namespace.go`):
  read files from the locally-cloned student repo and/or drive the cluster — `kubectl apply` a
  manifest, `kubectl port-forward` (`rules/k8s.go`), then HTTP-fetch and string-match the response.
  These require the student repos to be cloned locally and (for k8s rules) a live cluster + kubectl.
- **`ManualRule`** (`rules/manual.go`): returns 0% / "Manual grading is required" — a placeholder for
  checks graded by reading the work of the team: its report, its files and its Docker images. Every
  rule of Lab 1 (`rules/lab1.go`) and most L2/L3/L4/L5 rules are wired as these.
- Some automated rules also short-circuit to "Check the report manually" when they can't self-assess.

**Grader** (`rules/grader.go`) is the hub: one struct holding all assignment maps and every rule
instance, assembled in `NewGrader()`. `GradeL1(team)`…`GradeL5(team)` each run that lab's rules and
return `[]RuleEvaluationResult`. `cmd/grade.go` loops teams × selected labs and prints the report.

### Adding a grading rule

1. Implement `Spec()` and `Run()` on a new type in the appropriate `rules/*.go` file (put the shared
   manifest paths/ports/consts in `rules/k8s.go`).
2. Add a field for it on the `Grader` struct and construct it in `NewGrader()`.
3. Register it in `ListRuleSpecs()` (so `mc rule` prints it) **and** in the matching
   `GradeL<n>` method (so `mc grade` runs it).

Rule descriptions in `RuleSpec` are the source of truth for the human-readable rule catalog that the
README reproduces; update both together if you change wording.
