# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`mc` (Mincong Classroom) is a Cobra-based CLI in Go that evaluates student assignments for a
"Software Containerization and Orchestration" (Docker/Kubernetes) course. Each year ~25-30 students
form two-person teams; every team has its own Git repository cloned from the same template
([`mincong-classroom/containers`](https://github.com/mincong-classroom/containers)), so all student
repos share an identical file layout (e.g. `apps/spring-petclinic/Dockerfile`, `k8s/lab-2/pod-nginx.yaml`).
The tool exists to run team-specific commands and gather evidence in bulk across those dozens of repos.

## Build & run

```sh
go mod tidy
go build -o dist/mc     # binary is gitignored under dist/
./dist/mc help
```

CI (`.github/workflows/mincong-classroom.yaml`, runs on every push) does: `go mod tidy`,
`golangci-lint`, then `go test ./... -v`. Tests exist only for `common/`, `cmd/team/`, `e2e/`, `evaluation/` and `sheet/`
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
mc evidence -l L1 [-t red] [--late] [--force] [--template DIR]  # the evidence report of each team, from the rules of the year
mc eval ls [-l L2] [--dir DIR]  # the evaluations, one line per team and lab: commit, rules evaluated, to check, penalties
mc eval run -l L2 [-t teal] [--check-only] [--dir DIR]  # the agent of agents.eval_run (sh, in the repo of the rules), then the checks of its files: one problem per line, exit 1 if any
mc eval sync -l L2 [-t teal] [--sheet-dir DIR]  # the checks, then the cells of the files in ~/.mc/{year}/sheet/evaluation.json for mc sheet; without a read of the tab since the last export, writes evaluation.read.js and exits 1
mc annotate find IMAGE TEXT [--all]  # the box of each line of a screenshot with the text, with tesseract
mc annotate draw IMAGE OUT BOX... [--evaluator] [--padding N]  # a copy with numbered boxes x,y,w,h,green|red,label
mc sheet blocks|copy|pasted|verify|snippet TAB  # the data of a tab of the evaluation sheet, in ~/.mc/{year}/sheet/TAB*.json
mc info                       # the CLI, its commit and the registry: number of teams, URL of the evaluation sheet
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
- `~/.mc/{year}/evaluations/lab-{n}/{team}.yaml` — the evaluation of a lab for a team, written
  by the evaluator (an AI agent) and read by `mc eval ls`, `run` and `sync` (`evaluation.Load`). The files of
  `mc evidence` are next to it, in `~/.mc/{year}/teacher/`.

## Architecture

Eight packages: `cmd/` (CLI wiring), `common/` (domain types + team registry), `rubric/` (the YAML
of the evaluation rules), `evidence/` (the evidence report of a lab for a team), `evaluation/`
(the evaluation of a lab for a team, by the evaluator), `annotate/` (the
numbered boxes on a screenshot), `sheet/` (the data of a tab of the evaluation sheet), `github/` (the GitHub organization, through the `gh` CLI). `CONTRIBUTING.md` draws them and their imports: update its
diagram when a package is added, or imports another. `cmd/`, `common/`, `e2e/`, `evaluation/`,
`evidence/` and `sheet/` have a `README.md`, with a diagram: update it with its package. The readers
of every `README.md`, the root one included, are the users of `mc`: for now, the teacher. Write
them for these readers.

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
on Docker Hub (`DockerHub`, with the later patches of a tag). `markdown()` writes the report,
with a front matter (`metadata`): the commit evaluated and the commit of mc (`opts.Revision`, from
`common.Revision`, which reads the build info: the pseudo-version of `go run ...@{commit}`, or
`vcs.revision` of a local build, `-dirty` with changes). The report that the evaluator generates
from it, `docs/lab-{n}.generated.md`, copies the front matter: when it equals the one of now,
`Gather` returns `UpToDate` and gathers nothing, unless `opts.Force`, which the command sets for
`--force` and for a revision that isn't stable (`common.StableRevision`). The
test builds a template and a team repository with git in a temporary `HOME` with fixed dates, so
the commits and the golden report `evidence/testdata/lab-1.evidence.md` are the same at each run.

**Evaluation** (`evaluation/`) — the evaluator writes one YAML per team and lab, so that several
agents work at once: `LabDir(dir, lab)` is `{dir}/lab-{n}`, `Path(labDir, team)` its file
`{team}.yaml`, and `Teams(labDir)` the teams with a file. `Load(path)` decodes it as `rubric.Parse`
does: strictly with `KnownFields(true)` into Go maps (an unknown key, a key twice or a wrong type is
`Problems`, with its line), and as a `yaml.Node` for the order of the rules and the lines of the
keys. A syntax error keeps the line in the message of yaml.v3, which is 0-based for some parser
errors. `File.Check(opts)` returns the problems of the content, sorted by line: the team of the
name of the file, the commit (a prefix of `opts.Commit`, the facts of `mc evidence`; empty means no
facts), the action items, each mandatory rule of the lab, the completeness by steps of 0.1,
`done_well` and `to_improve`, the analysis level, the writing style, and the penalty: a key of
`scoring.penalties` with its `remove`, `rule` (`PenaltyRule`) with a `remove` of the rule's own
penalties, or `late` (`PenaltyLate`) within `scoring.late_penalty`. It imports `rubric` and `sheet` only:
`cmd/eval.go` finds the worktree of the team by its repository in the registry, and reads the facts
with `evidence.ReadFacts(evidence.ReportPath(worktree, lab))`. It checks the content of the rules
only (`loadRules` with no root): the files of the teacher don't change an evaluation. The tests use
the made-up rules of `rubric/testdata/`.

`Sync(opts)` (`sync.go`) computes the rows of the tab Evaluation, a `sheet.File`, from the files
of a lab, the tab read, the last export and what waits. Its layout is `sheetColumns` (A, D to U;
never B or C, the students), the one of `SheetFile()`, which `cmd/eval_sync.go` passes to
`sheet.Snippet` for `evaluation.read.js`; a read with another header is an error. A row is found by
Team, Lab and Rule; a missing one is `_new`, numbered after the last row of the tab or of what
waits, with the formulas F, H and R (B and C are filled down by hand: `FillFrom`, `LastNewRow`).
The completeness is written when the read cell is empty, or when it is the exported value (mc's
own) and the new one is higher, with the TA Comment; otherwise it is kept, with a sentence in TA To
Check. Another cell is written when it differs, with a `_check` when the tab holds a value that mc
didn't write (`sheet.SameValue` against the export). A formula is in the tab when the export has
the same text and the cell is not empty. The rows that wait for the lab and the teams are replaced,
reusing the numbers of the new ones; a row that waits for another lab must still match the tab.
`cmd/eval_sync.go` requires `evaluation.read.json` newer than `evaluation.exported.json`.

**Annotate** (`annotate/`) — `Read(ctx, img)` runs `tesseract --psm 11` on the image enlarged twice
(the filter Mitchell, as ImageMagick does), then groups its words in lines (`groupLines`): by row,
split at a large gap. `Line.Find` matches a text without the case, the spaces and the kind of
dash. `Draw(img, marks, opts)` draws the boxes in pure Go, with the font Go Bold of
`golang.org/x/image`: no ImageMagick. The tests parse a made-up TSV of tesseract; `TestRead` runs
tesseract only when it is installed, so the CI skips it.

**Sheet** (`sheet/`) — the data of a tab of the evaluation spreadsheet, in JSON files of
`~/.mc/{year}/sheet/` (`cmd/sheet.go`: `TAB.json` waits to be written, `TAB.read.json` is the tab as
read, `TAB.exported.json` the last export). `mc` never calls the API of Google Sheets: the teacher
or a browser agent pastes the blocks, and reads the tab with the script of `mc sheet snippet`. A
`File` holds one object per row, its attributes the columns by header (`Columns` gives the
letters), found by its `Keys`; `_row` is its number in the tab, `_check` the cells to look at
before writing them, and `_new` a row that the tab doesn't have yet, whose keys `Blocks` writes too
(`RecordPasted` clears it once its keys are pasted). A row holds the cells to write only, never an empty text, which would clear
a cell. `UnmarshalJSON` decodes the rows by hand (an unknown `_` attribute is an error);
`MarshalJSON` orders the attributes like the columns, and `Save` calls it directly, since
`json.Marshal` would escape `<`, `>` and `&` again. `Blocks` splits the cells to write into
rectangles where every cell is written (`rectangles`: the runs of rows of each column, merged
across adjacent columns), the `_check` ones last; `pasteText` adds the apostrophe that keeps a
text a text (a number such as `2e88112`, a date, `-0.2`). `RecordPasted` moves the cells of the
blocks pasted to the export; `Verify` compares the tab read with it, cell by cell at the same
letter, never a formula. The script (`snippet.js`, its `CONFIG` replaced by `Snippet`) queries gviz
from the page, numbers the rows from 2 (gviz keeps the empty rows above the last one), and copies
the rows to the clipboard. The tests use made-up teams only.

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
(`GetRepoPath`, `GetRepoURL`).
Prefer these helpers over rebuilding paths inline: `mc git clone|pull|show|exec` and `mc evidence`
all find the team repos at `GetRepoPath()`, `$HOME/github/mincong-classroom/k8s-<name>`.

