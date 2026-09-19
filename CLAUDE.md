# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`mc` (Mincong Classroom) is a Cobra-based CLI in Go that auto-grades student assignments for a
"Software Containerization and Orchestration" (Docker/Kubernetes) course. Each year ~25-30 students
form two-person teams; every team has its own Git repository cloned from the same template
([`mincong-classroom/containers`](https://github.com/mincong-classroom/containers)), so all student
repos share an identical file layout (e.g. `apps/spring-petclinic/Dockerfile`, `k8s/pod-nginx.yaml`).
The tool exists to run team-specific commands and grading rules in bulk across those dozens of repos.

## Build & run

```sh
go mod tidy
go build -o dist/mc     # binary is gitignored under dist/
./dist/mc help
```

CI (`.github/workflows/mincong-classroom.yaml`, runs on every push) does: `go mod tidy`,
`golangci-lint`, then `go test ./... -v`. Tests exist only for `common/`, `cmd/team/` and `e2e/`
so far; add `_test.go` files alongside the package under test. The team unit tests use an
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

Key commands (all read the team registry, see "External data" below):

```sh
mc team ls [--json]           # teams, their GitHub status and validation problems, students not in a team
mc team provision [--dry-run] # interactive: asks for the team, creates repo + GitHub team, invites the members
mc team red validate|status [--json]  # the actions on one team of the registry
mc rule                       # print every grading rule's spec/description
mc grade                      # grade all teams, all labs (L1-L5)
mc grade -t red -t blue -l L3 # grade specific teams (-t, repeatable) for one lab (-l L3/3)
mc git clone|pull|show        # bulk git ops over all team repos, e.g. `mc git show main:k8s/pod-nginx.yaml`
mc k8s create-namespaces      # kubectl create one namespace per team
```

## External data (critical — not in this repo)

Grading input lives **outside the repo** in a private, git-ignored `~/.mc/` directory that you must
assume exists at runtime. Nothing here works without it:

- `~/.mc/teams-{year}.yaml` — the team registry (`TeamRegistry`). The year is `common.Year()`:
  the `defaultYear` const in `common/team.go` (bump it for a new cohort), overridden by the
  environment variable `MC_YEAR`. Unknown keys (e.g. an old `role`) are ignored when read.
  The global flag `--team-file` replaces it for any command (`common.TeamRegistryFile`, a leading
  `~/` is expanded). Since the `mc team <team>` subcommands are built before Cobra parses the
  flags, `cmd.Execute()` reads `--team-file` ahead from `os.Args` (`teamFileFromArgs`).
  The registry's optional `students:` list (`common.Student`, one `name: "LAST, First"` each) is
  informational: `mc team ls` lists the students not in a team, and a member not among them is a
  validation **warning**. There is no separate student file.
  `validateTeam()` returns **errors** (block `provision`: team name format/reserved/duplicate, a
  member's GitHub username missing or not found) and **warnings** (`provision` asks "Provision it
  anyway?" before the steps: more than 2 members, a member in several teams, without a name, or
  not among the students). For a member, only the GitHub user is enforced.
- `~/.mc/assignments-L1.yaml` … `assignments-L4.yaml` — per-lab, per-team structured data
  (`common.TeamAssignmentL*`), loaded in `rules.NewGrader()`.

Gotcha: `NewGrader()` only reads the **L1–L4** assignment files. `assignmentsL5` is never populated,
so `GradeL5` always reports "team not found in assignments" and grades nothing until an
`assignments-L5.yaml` load is added. Most assignment structs are also empty placeholders today
(`TeamAssignmentL2/L4/L5` have no fields); only L1 (`mvn_command`) and L3 (`nginx_pod_name`) carry data.

## Architecture

Four packages: `cmd/` (CLI wiring), `common/` (domain types + team registry), `rules/` (grading
engine), `github/` (the GitHub organization, through the `gh` CLI).

**Team management** (`cmd/team/`, `github/`) — replaces GitHub Classroom. Each team gets a private
repo `k8s-<name>` generated from the template and a secret GitHub team `<name>` with `push` on it.
`github.Client` holds the reads (repo/team exist, team role on the repo, membership state, user);
the changes are `github.Command` values (`CreateRepoCommand`, …) so `provision` can show the exact
`gh` command before running it, and the tests can record them. `provision` computes the steps
from `teamStatus()`, skips those already done (idempotent), refuses a team with
`validateTeam()` errors, asks to confirm its warnings, and asks for each step, one at a time: every prompt is a
`(y/N)` question (`confirm()`), "no" by default, with no "all" nor "quit" answer — a "no" skips
the rest of the team, ctrl+c stops the command. `--dry-run` keeps the prompts but runs nothing. `provisioner.run()` provisions **one team
per run**: it asks for the team by name (an empty answer asks again; ctrl+c, or the end of the
input in the tests, stops it), then prints `Team "x" provisioned.` with `github.RepoURL`/`TeamURL`.
The `$ gh …` lines are dark yellow when stdout is a terminal and `NO_COLOR` is unset
(`provisioner.color`). A registered team without members goes to `completeTeam()` (members saved
with `common.SetTeamMembers()`); an unknown name
goes to `registerTeam()`: name checked (`teamNameErrors`), members picked by number among the
registry's students not in a team (`askMembers`: `pickStudents`, `parsePicks`, or `typeMembers`
when there is none to pick), GitHub usernames checked on entry, then `common.AddTeam()` appends it to the registry file through the `yaml.Node` API (the
comments survive; `editRegistry` writes a temp file, then renames it). `LoadRegistry` rejects an
unknown top-level key (only `students` and `teams`), so that a typo such as `student:` is not
silently ignored. Not saved with `--dry-run`. The
`cmd/team` unit tests run with a temporary `HOME` (`TestMain`), so they can never write the
teacher's `~/.mc`. `ls` fetches per team concurrently (`forEach`).
The command tree is `mc team <action>` (`ls`, `provision`) and `mc team <team> <action>`
(`validate`, `status`): `AddTeamCommands()` adds one subcommand per registered team, called from
`cmd.Execute()` before Cobra resolves the args, so the registry is read at startup. `ls` and
`provision` are reserved team names (`reservedNames`).

**Team model** (`common/types.go`) — a `Team` has a `Name`, `Members`, a `Role`
(`"frontend"` | `"customer"` | `"veterinarian"`, which selects the L3 Docker image rule), and an
optional `CustomRepoName`. All team-derived paths/URLs come from methods on `Team`
(`GetRepoPath`, `GetKubeNamespace` → `team-<name>`, `GetContainerRepoForWeekendServer`, `GetRepoURL`).
Prefer these helpers over rebuilding paths inline. Note an existing inconsistency: `GetRepoPath()`
uses `$HOME/github/mincong-classroom/k8s-<name>`, while `cmd/git/clone.go` and `show.go` hardcode
`/Users/mincong/github/mincong-classroom/<name>` — if you touch repo-path logic, reconcile these.

**Rule abstraction** (`common/types.go`) — every grading check implements
`Rule[T]{ Spec() RuleSpec; Run(team, opts T) RuleEvaluationResult }`. A `RuleSpec` combines
`LabId` + `Symbol` into an id like `L1_DKF` (see `RuleSpec.Id()`); `RuleEvaluationResult.Completeness`
is a float in `[0,1]` (a percentage). There are three flavors of rule:
- **Automated file/cluster checks** (`rules/docker.go` `DockerfileRule`, `rules/k8s_pod.go`,
  `k8s_replicaset.go`, `k8s_deployment.go`, `k8s_service.go`, `k8s_namespace.go`, `registry.go`):
  read files from the locally-cloned student repo and/or drive the cluster — `kubectl apply` a
  manifest, `kubectl port-forward` (`rules/k8s.go`), then HTTP-fetch and string-match the response.
  These require the student repos to be cloned locally and (for k8s rules) a live cluster + kubectl.
- **`ManualRule`** (`rules/manual.go`): returns 0% / "Manual grading is required" — a placeholder for
  checks the teacher does by hand from the team's report. Most L2/L3/L4/L5 rules are wired as these.
- Some automated rules also short-circuit to "Check the report manually" when they can't self-assess.

**Grader** (`rules/grader.go`) is the hub: one struct holding all assignment maps and every rule
instance, assembled in `NewGrader()`. `GradeL1(team)`…`GradeL5(team)` each run that lab's rules and
return `[]RuleEvaluationResult`. `cmd/grade.go` loops teams × selected labs and prints the report.

### Adding a grading rule

1. Implement `Spec()` and `Run()` on a new type in the appropriate `rules/*.go` file (put the shared
   manifest paths/ports/consts in `rules/k8s.go`).
2. Add a field for it on the `Grader` struct and construct it in `NewGrader()`.
3. Register it in `ListRuleRepresentations()` (so `mc rule` prints it) **and** in the matching
   `GradeL<n>` method (so `mc grade` runs it). Some rules are effectively dead today: `RegistryRule`
   (`registry.go`) and `SqlInitRule` (`sql.go`) are defined but never added to the `Grader`, and
   `MavenJarRule` (`maven.go`) is constructed and listed by `mc rule` but its `.Run` call is
   commented out in `GradeL1`, so it is never actually graded.

Rule descriptions in `RuleSpec` are the source of truth for the human-readable rule catalog that the
README reproduces; update both together if you change wording.
