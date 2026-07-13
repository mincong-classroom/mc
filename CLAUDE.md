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
`golangci-lint`, then `go test ./... -v`. **There are currently no test files**, so `go test`
passes vacuously — add `_test.go` files alongside the package under test if you write tests.

Key commands (all read the team registry, see "External data" below):

```sh
mc team                       # list registered teams and members
mc rule                       # print every grading rule's spec/description
mc grade                      # grade all teams, all labs (L1-L5)
mc grade -t red -t blue -l L3 # grade specific teams (-t, repeatable) for one lab (-l L3/3)
mc git clone|pull|show        # bulk git ops over all team repos, e.g. `mc git show main:k8s/pod-nginx.yaml`
mc k8s create-namespaces      # kubectl create one namespace per team
```

## External data (critical — not in this repo)

Grading input lives **outside the repo** in a private, git-ignored `~/.mc/` directory that you must
assume exists at runtime. Nothing here works without it:

- `~/.mc/teams-2025.yaml` — the team registry (`TeamRegistry`). The year `2025` is hardcoded as the
  `year` const in `common/team.go`; bump it there for a new cohort.
- `~/.mc/assignments-L1.yaml` … `assignments-L4.yaml` — per-lab, per-team structured data
  (`common.TeamAssignmentL*`), loaded in `rules.NewGrader()`.

Gotcha: `NewGrader()` only reads the **L1–L4** assignment files. `assignmentsL5` is never populated,
so `GradeL5` always reports "team not found in assignments" and grades nothing until an
`assignments-L5.yaml` load is added. Most assignment structs are also empty placeholders today
(`TeamAssignmentL2/L4/L5` have no fields); only L1 (`mvn_command`) and L3 (`nginx_pod_name`) carry data.

## Architecture

Three packages: `cmd/` (CLI wiring), `common/` (domain types + team registry), `rules/` (grading engine).

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
