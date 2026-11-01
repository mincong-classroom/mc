# mc

The CLI tool for Mincong Classroom (mc). It's mainly for auto-grading assignments 🤖

## Problems

### Managing Git Repositories

There are about 25-30 students in a classroom each year. If they group into teams of two members, there are tens of repositories. It is hard to manage without a specific tool. In particular, all the Git repositories have the same structure because they are cloned from the same template [`mincong-classroom/containers`](https://github.com/mincong-classroom/containers). This is challenging for updating the repository, printing the content of a specific file, etc.

### Running Team Specific Commands

It's a bit difficult to run all the team specific commands manually. I could do that with a for-loop but inside that for-loop, there are some variables that need to be computed, all of which are team-specific. For example, the Git repo name, the path of a specific file, ...

## Installation

Install Golang:

```sh
brew install go

go version
# go version go1.24.1 darwin/arm64
```

Then build the CLI:

```sh
go mod tidy
go build -o dist/mc
```

## Usage

```sh
mc help
```

```
Mincong Classroom (mc) is a command line interface for grading student
assignments in the Kubernetes course.

Usage:
  mc [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  git         Perform Git operations in bulk
  grade       Grade assignments
  help        Help about any command
  info        Display CLI information
  k8s         Perform Kubernetes operations
  rule        List, check and render the evaluation rules of the year
  team        Manage the teams

Flags:
  -h, --help               help for mc
      --team-file string   Team registry to use instead of ~/.mc/teams-{year}.yaml, e.g. a test registry to try the commands

Use "mc [command] --help" for more information about a command.
```

## Settings

The settings of `mc` on this machine are in `~/.mc/settings.yaml`. The file and each of its keys
are optional:

```yaml
year: 2026                                                  # the year of the cohort
rules: ~/github/my-org/course/{year}/evaluation-rules.yaml  # the evaluation rules of each year
agents:
  eval_run: claude -p "/evaluate-lab {lab} {teams}"         # the AI agent of mc eval run
```

The year selects the files of the cohort, such as the team registry `~/.mc/teams-{year}.yaml`. It
is the environment variable `MC_YEAR` if set, e.g. `MC_YEAR=2025 mc grade`, then the key `year`,
then the current year. `rules` is the YAML of the evaluation rules, where `{year}` is the year:
`mc rule`, `mc evidence` and `mc eval` read it. `agents` are the command lines of the AI agents
that `mc` starts, see [Evaluation](#evaluation). An unknown key is an error, so that a typo doesn't
go unnoticed.

## Team

The teams are registered in a private YAML file, the team registry `~/.mc/teams-{year}.yaml`.
A team has a name, lowercase letters only (such as a color), and at most two members. A team
can have no members yet: it is created before the course, and its members are added later.

A name is written `"LAST First"`: the last name in upper case, then the first name in Pascal case,
each of one or more words, e.g. `"DE LA FONTAINE Jean-Pierre"`. A member whose name does not follow
this format is a warning; the names are compared ignoring the case and the extra spaces.

The optional `students` list holds the students of the year, known before the course. It is
informational: `mc team ls` lists the students who are not in a team yet, and a member who is not
among them is a warning, shown by `provision` before its confirmation. For a member, only the
GitHub user must be valid.

The optional `evaluation_sheet` is the URL of the spreadsheet where the teams of the year are
evaluated. `mc` does not read the spreadsheet: `mc info` prints its URL, so that it can be found
without opening the registry, which holds the names of the students.

```yaml
evaluation_sheet: https://docs.google.com/spreadsheets/d/{id}/edit
students:
  - name: "SMITH John"        # "LAST First"
  - name: "DOE Jane"
teams:
  - name: red
    members:
      - name: "SMITH John"    # as in the students
        github: jsmith        # GitHub username
  - name: orange
    members: []               # not taken yet
```

The year of the registry is the one of the settings: see "Settings".

The global flag `--team-file` replaces the team registry for any command, e.g. to rehearse with a
test registry before the course. `provision` still acts on the real organization: add `--dry-run`.

```sh
mc team ls --team-file ~/.mc/test-teams-2026.yaml
mc team yellow provision --dry-run --team-file ~/.mc/test-teams-2026.yaml
```

Each team gets a private repository `k8s-{team}` in the GitHub organization, generated from the
template repository `mincong-classroom/containers`, and a secret GitHub team `{team}` with push
access to it. With the organization's base permission set to `none`, the members of a team only
see their own repository. The GitHub calls go through the `gh` CLI, logged in with the scopes
`repo` and `admin:org`:

```sh
gh auth refresh -s admin:org
```

The commands are `mc team ls`, and `mc team <team> <action>` for the actions on one team of the
registry: each team is a subcommand. `mc team --help` lists the teams with their members, and
`mc team red --help` the actions on the team `red`. A team that is not in the registry yet has one
action, `provision`, which registers it. The names `ls` and `provision` are reserved.

| Command | What it does |
|---|---|
| `mc team ls [--json]` | Lists the teams with their members, their status on GitHub and their validation problems, then the students not in a team yet |
| `mc team red provision [--dry-run]` | Shows the members of the team with their GitHub accounts, and once confirmed, creates its repository and its GitHub team, grants the team push access, and adds the members, which invites them to the organization. The team can be new: it is registered |
| `mc team red validate [--json]` | Errors, which prevent the provisioning: an invalid or reserved name, a name used by another team, a member without a GitHub username or whose GitHub user does not exist. Warnings, confirmed when provisioning: more than 2 members, a member in several teams or not among the students. Prints the display name of each GitHub account, for the students to confirm it |
| `mc team red status [--json]` | Shows whether the repository and the GitHub team exist, the access of the team to the repository, and whether each member is `active` or `pending` (invitation not accepted yet) |

`provision` asks one question: it shows the team, each member with the display name of their
GitHub account, and the warnings of the validation, then asks to confirm with `y` ("no" is the
default). What it takes to provision the team is not asked: once confirmed, it runs the steps that
are not done yet, one after the other, so a team can be provisioned again. A team with errors is not
provisioned, and nothing is asked. ctrl+c stops the command. With `--dry-run`, the same questions
are asked, and the steps are described with the `gh` command each one would run (in dark yellow in
a terminal, unless `NO_COLOR` is set), but nothing changes.

The members come from the registry, or are asked first:

- a team that is not in the registry yet gets its members, and is registered once confirmed;
- a registered team without members, such as a team created before the course, gets them too.

The members are picked by number among the students of the registry who are not in a team yet, or
typed when there is none to pick, each with their GitHub username (checked right away, and its
display name shown). The registry is updated once confirmed, keeping its comments, except with
`--dry-run`.

```
$ mc team purple provision
The team purple is not in the registry yet.
Students not in a team yet:
   1. DOE Jane
   2. MARTIN Alex
Members, by number, e.g. "1 2" (empty for none yet): 1 2
GitHub username of DOE Jane: jdoe
  @jdoe: "Jane Doe" on GitHub
GitHub username of MARTIN Alex: amartin
  @amartin: "Alex Martin" on GitHub

== Team purple: 2 members
  - DOE Jane (@jdoe): "Jane Doe" on GitHub
  - MARTIN Alex (@amartin): "Alex Martin" on GitHub
Provision the team purple? (y/N): y
✓ the team purple saved to ~/.mc/teams-2026.yaml
✓ Create the private repository mincong-classroom/k8s-purple from the template mincong-classroom/containers
✓ Create the secret GitHub team purple
✓ Grant the GitHub team purple push access to k8s-purple
✓ Add @jdoe to the GitHub team purple, which invites them to the organization
✓ Add @amartin to the GitHub team purple, which invites them to the organization

Team "purple" provisioned.
- repo: https://github.com/mincong-classroom/k8s-purple
- team: https://github.com/orgs/mincong-classroom/teams/purple
```

## Git

The commands `mc git` run git in bulk, in the repository of each team of the registry, cloned
under `~/github/mincong-classroom/k8s-{team}`.

| Command | What it does |
|---|---|
| `mc git clone` | Clones the repository of each team |
| `mc git pull` | Pulls the latest changes of each team, by fast-forward only: see below |
| `mc git show <rev>:<path>` | Shows a file of each team, e.g. `mc git show main:k8s/lab-2/pod-nginx.yaml` |
| `mc git exec -- <git arguments>` | Runs any git command for each team, e.g. `mc git exec -- log -1 --oneline` |

`exec` passes the arguments after `--` to git as they are, and prints the name of the team before
its output. When git fails for a team, it goes on with the next ones, then fails at the end,
listing these teams.

`mc git pull` only fast-forwards (`git pull --ff-only`): the history of a team is never merged
into the local one. When a team rewrote the history of its branch, e.g. with a force-push, the
local branch is no longer an ancestor of the remote one: the repository of that team is not
pulled, a warning says so, and the other teams are pulled.

```
[team: red] ⚠ history rewritten: force-push? The local branch is no longer an ancestor of the remote one: not pulled
```

The local clone keeps the history it had: compare it with the new one
(`git log main...origin/main`) before resetting it (`git reset --hard origin/main`).

## Rule

The evaluation rules of a program are in a YAML file with two maps: `labs`, the lab sessions of
the module with their deadline, and `rules`, one entry per rule keyed by its ID, such as `L1_DKF`.
`mc` lists the rules of the year, checks them, and renders them as a Markdown page:

```sh
mc rule ls -l L1                       # the rules of Lab 1
mc rule check                          # one problem per line, exit 1 if any
mc rule render -o evaluation-rules.md  # the page, wrapped at 96 characters
```

The file of the year comes from the key `rules` of the settings, `~/.mc/settings.yaml`, where
`{year}` is the year (see "Settings"). The flag `--rules-file` replaces it, e.g. with a test file:
`mc rule ls --rules-file rubric/testdata/evaluation-rules.yaml`.

The check covers the keys, the IDs of the labs and of the rules, the exercises, the coefficients
(by steps of 0.05, and those of the mandatory rules of a lab add up to 1), the penalties, the
levels of analysis, the kinds of evidence, the dependencies, the deadlines, the paths, and the
width of the lines: 120 characters at most. A problem gives the line of the file:

```
evaluation-rules.yaml:142: L1_DKF: unknown key "penalty"
```

The made-up file of the tests shows the format,
[`rubric/testdata/evaluation-rules.yaml`](rubric/testdata/evaluation-rules.yaml), and its page,
[`rubric/testdata/evaluation-rules.md`](rubric/testdata/evaluation-rules.md).

## Evidence

`mc evidence` gathers the evidence of each rule of a lab, for each team, in a report for the
teacher. The report holds facts, not a score, of 3 kinds:

1. What the team wrote in its report for the exercise of the rule, without the lines of the
   statement.
2. The files of the repository: those of the rule, and those the report links to. A short file
   is imported, a file of the template shows its changes, and a long one is linked.
3. What the evaluator made: its notes and its files, such as screenshots and logs, next to the
   report, and the tags of the images of the rule on Docker Hub, with their push dates.

```sh
mc evidence -l L1 --template ~/github/mincong-classroom/containers
mc evidence -l L1 -t red --late   # origin/main, for a late pass
mc evidence -l L1 -t red --force  # gather again, even when the generated report is up to date
```

The rules are those of the year, from the settings, or the file of the flag `--rules-file`. The
commit evaluated is the head of `main` at the deadline of the lab, from the activity of the
repository on GitHub. The report of `docs/lab-1.md` is `docs/lab-1.evidence.md`, and the files of
the evaluator go to `docs/lab-1.evidence/`: its notes for a rule are `L1_DKF.md`, and its files
`l1-dkf-*`, such as `l1-dkf-build.log`. Run the command again once they are there.

The report goes to the local branch `teacher` of the clone of the team, checked out in a worktree
under `~/.mc/{year}/teacher/`, one directory per team: open it in an editor to read the reports
next to the files of the team. The branch starts from the commit evaluated, the next pass merges
the new one into it, and each change of a report is committed. `mc` never pushes it.

The evaluator can generate a report of its own from the evidence, such as a report for the
teacher, in `docs/lab-1.generated.md`. The front matter of the evidence holds 2 commits: the
commit of the team evaluated, and the commit of `mc` that gathered the evidence. `mc info` prints
the second one:

```yaml
---
commit: c028aea876b2754fb24541d0ea89f90a06eda946
mc: 949d7aa740ab
---
```

The generated report copies this front matter. While both commits stay the same, nothing changed
since it was written: `mc evidence` skips the team, and says that its report is up to date. The
flag `--force` gathers the evidence again anyway, e.g. after a new image on Docker Hub, or a new
version of the generated report. A local build of `mc` with changes never skips a team, since its
commit doesn't tell which code ran.

## Evaluation

`mc eval` is the command of the teacher for the evaluation of a lab, and `mc rule` the one for
its rules. `mc eval run` starts an AI agent, which does the work: it gathers the evidence with `mc
evidence`, puts boxes on the screenshots with `mc annotate`, and evaluates each rule. The agent is
a command line of the settings, run with `sh` in the Git repository of the rules, where `{lab}` is
the lab, such as `L2`, and `{teams}` the teams of `-t`, or `all`:

```yaml
agents:
  eval_run: claude -p "/evaluate-lab {lab} {teams}"
```

The agent writes what it found for each team in a YAML file, one per team and lab, so that several
agents can work at once: `~/.mc/{year}/evaluations/lab-{n}/{team}.yaml`. Then `mc eval run`
checks these files against the evaluation rules, before they become the rows of the evaluation
sheet, and `mc eval ls` lists them. Here is the file of a made-up team, `teal`, in Lab 2:

```yaml
team: teal
commit: 0a1b2c3                # the commit evaluated, the one of the facts of mc evidence
time_spent: Session only, done at 11:21
action_items: Run it yourself  # or Talk with them, Less attention, Late work to decide
rules:
  L2_POD:
    completeness: 0.8          # from 0 to 1, by steps of 0.1
    done_well: The Pod runs the image of Lab 1.
    to_improve: The report doesn't show the logs of the Pod.
    to_check: ""               # what the teacher should check, if anything
    analysis: short            # a key of analysis_levels in the rules
    writing_style: own words
    penalty: null              # or {kind: secret, remove: 0.3, why: The report shows a token.}
```

The comment of the sheet joins `done_well` and `to_improve`, so that it always says what the team
did well and what it can improve: the check asks for both. To list the files, one line per team
and lab:

```sh
mc eval ls                      # every lab, in the order of the rules
mc eval ls -l L2                # one lab
mc eval ls --dir DIR            # the files of DIR/lab-{n}/
```

Each line gives the commit evaluated, the mandatory rules evaluated out of those of the lab (and
the optional ones after a `+`), the rules that the teacher should check, the penalties and the
action items:

```
LAB  TEAM   COMMIT   RULES   TO CHECK  PENALTIES  ACTION ITEMS
L2   coral  0a1b2c3  1/1 +1  1         1          Talk with them
L2   teal   0a1b2c3  1/1     0         0          Run it yourself
```

A file that can't be read is listed with dashes, and the standard error says to run `mc eval run`
for its problems. To evaluate a lab, then check its files:

```sh
mc eval run -l L2               # the agent for all, then every file of ~/.mc/{year}/evaluations/lab-2/
mc eval run -l 2 -t teal        # one team
mc eval run -l 2 --check-only   # the checks only, without the agent
mc eval run -l 2 --dir DIR      # the files of DIR/lab-2/
```

A failed agent stops the command before the checks. The agent runs with the variable
`MC_EVAL_AGENT` set: `mc eval run` refuses to start another agent from it, so the agent checks its
own files with `--check-only`. With `claude -p`, a tool that the settings of Claude Code don't
allow is refused, since nobody is there to approve it: allow the commands of the skill in its
settings, or pass the options of `claude` in the command line of the agent.

The rules come from the settings, or the file of the flag `--rules-file`, as for `mc rule`. Each
mandatory rule of the lab must be there once, and an optional rule may be absent. The commit must
be the one of the facts that `mc evidence` gathered for the team: the commit in the front matter
of its report, such as `~/.mc/{year}/teacher/k8s-teal/docs/lab-2.evidence.md`. A short hash
matches the full one. Without facts, the check says to run `mc evidence` first.

A penalty removes a share of the score of a rule. Its `kind` is one of 3:

- a penalty of every rule, by its name in `scoring.penalties`, such as `secret`: `remove` is the
  share that the rules give
- `rule`, the penalty of the rule itself, when the rule has one: `remove` is one of its shares
- `late`, for work pushed after the deadline: `remove` goes from the `min` to the `max` of
  `scoring.late_penalty`

A problem gives the file and its line, and the command exits with 1 if there is any, like `mc rule
check`:

```
/Users/me/.mc/2026/evaluations/lab-2/coral.yaml:8: L2_POD: unknown key "to_chek"
/Users/me/.mc/2026/evaluations/lab-2/teal.yaml:7: L2_POD: the completeness 0.85 is not from 0 to 1 by steps of 0.1
/Users/me/.mc/2026/evaluations/lab-2/teal.yaml:9: L2_POD: no to_improve: what the team can improve, which the comment of the sheet ends with
/Users/me/.mc/2026/evaluations/lab-2/teal.yaml:13: L2_POD: the penalty secret removes 0.3 of the score: got 0.2
Error: 4 problems in 2 of 2 files
```

A file with an unknown key, a key twice or a value of the wrong type stops at these problems: the
other checks of the file run once they are fixed. Without a problem, the command prints how many
files it checked.

## Annotate

`mc annotate` puts numbered boxes on a screenshot, around what the evaluator found: a green box
validates something, and a red box is a flag. The text next to the image refers to each box by
its number. It takes 2 steps: `find` reads the screenshot with
[tesseract](https://github.com/tesseract-ocr/tesseract) and prints the box of each line that
contains a text, then `draw` copies the screenshot with the boxes:

```sh
brew install tesseract   # for mc annotate find only

mc annotate find build.png "BUILD SUCCESS"
# 120,40,300,22	[INFO] BUILD SUCCESS
mc annotate find build.png --all   # every line, to choose another text
mc annotate draw build.png build-boxed.png 120,40,300,22,green,1 40,90,210,22,red,2
```

A box is written `x,y,w,h,COLOR,LABEL`: the box that `find` prints, `green` or `red`, and its
number. The border is thick, with a padding around the text and no fill, and the number is in a
circle of its color, on the right side of the box: the image is widened when a box ends too close
to its right edge. The search of `find` ignores the case, the spaces and the kind of dash, since
tesseract often splits a word in two and reads a long dash as a hyphen.

By default, a gray band at the bottom of the copy says that the screenshot is the team's, and
that the evaluator added the boxes. The flag `--evaluator` leaves it out, for a screenshot of the
evaluator, which carries its own watermark. The flag `--padding` sets the space between a text and
its box, e.g. `--padding 1` when the box crosses the lines around it.

## Sheet

`mc sheet` moves the data of a tab of the evaluation spreadsheet between JSON files and the
spreadsheet, by hand or through a browser. It never calls the API of Google Sheets, so it needs no
project of Google Cloud. Each tab has its files in `~/.mc/{year}/sheet/`, named after the tab, such
as `evaluation` for the tab "Evaluation":

| File | What it holds |
|---|---|
| `evaluation.json` | What waits to be written in the tab |
| `evaluation.read.json` | The tab, as read in the browser by the script of `mc sheet snippet` |
| `evaluation.exported.json` | What was written last, to tell a cell that the teacher changed |

A file holds one object per row, like the documents of a collection in MongoDB: each attribute is
a column, named by its header. Here is a file with a made-up team:

```json
{
  "tab": "Evaluation",
  "keys": ["Team", "Lab", "Rule"],
  "columns": {"A": "Team", "D": "Lab", "E": "Rule", "G": "Completeness", "L": "TA Commit", "O": "TA Analysis"},
  "rows": [
    {"_row": 81, "Team": "teal", "Lab": "L2", "Rule": "L2_RUN", "TA Commit": "2e88112", "TA Analysis": "detailed"},
    {"_row": 82, "Team": "teal", "Lab": "L2", "Rule": "L2_FBP", "Completeness": 0.9,
     "_check": {"Completeness": "0.7 at the last export, and no read of the tab since"}}
  ]
}
```

As you can see:

- the keys, Team, Lab and Rule, find a row, and `_row` is its number in the tab
- a row holds the cells to write only: an attribute absent is a cell left alone, never cleared
- `columns` gives the letter of each header. A column out of it is never read nor written, such as
  a column of personal data
- `_check` names the cells to look at before writing them, which the teacher may have changed
  since the last export

To write a tab by hand:

```sh
mc sheet blocks evaluation       # the blocks of cells to paste
mc sheet copy evaluation 1       # the block 1 to the clipboard, --print to print it instead
# In the spreadsheet: type the range of the block in the Name Box, press Enter, then paste.
mc sheet pasted evaluation       # every block was pasted: they become the last export
mc sheet pasted evaluation 1 2   # or the blocks 1 and 2 only: the others wait still
```

A block is a rectangle of adjacent cells, every one of them written, since a paste of an empty cell
would clear it. The keys are never written. A text that Sheets would read as something else, such
as the commit `2e88112` (a number), starts with an apostrophe, which keeps it a text. The blocks
with a `_check` come last.

To check the tab afterwards, read it in the browser:

```sh
mc sheet snippet evaluation | pbcopy   # then run it in the console of the page of the spreadsheet
pbpaste > ~/.mc/2026/sheet/evaluation.read.json
mc sheet verify evaluation             # one difference per line, and an exit code of 1 if any
```

The script reads the columns of the files only, with the query language of
[Google Visualization](https://developers.google.com/chart/interactive/docs/querylanguage) (gviz),
and refuses to run in another spreadsheet than the `evaluation_sheet` of the team registry. gviz
types a column by the majority of its values: a value of another type, such as a text in a column
of numbers, comes back empty. `verify` never compares a formula, since the tab gives its result.

## Key Components

The `cmd` directory wires the commands of the command line interface with Cobra: [its
README](cmd/README.md) explains how a command runs, and how `mc eval run` starts an agent.

The `rubric` directory reads the YAML of the evaluation rules: it checks the file, and renders its
page from the template `rubric/page.md.tmpl`.

The `evidence` directory gathers the evidence of a lab for a team, on the local branch `teacher`
of its clone: [its README](evidence/README.md) shows where the facts come from.

The `evaluation` directory reads the evaluation of a lab for a team, as the evaluator writes it,
and checks it against the evaluation rules: [its README](evaluation/README.md) shows what a check
reads.

The `annotate` directory finds a text in a screenshot with tesseract, and draws numbered boxes on
it.

The `sheet` directory keeps the data of the tabs of the evaluation spreadsheet in JSON files, and
moves it through a browser: [its README](sheet/README.md) explains why, with a diagram.

The `rules` directory contains the rules of 2025 for the auto-grading, which are deprecated.

The `github` directory manages the GitHub organization (repositories, GitHub teams and their members) through the `gh` CLI.

The `common` directory holds what the commands share: the settings, the year, the team registry
and the commit of `mc`. [Its README](common/README.md) draws its types, and the packages that use
them.

The `e2e` directory runs the binary of `mc` end to end, in a home of its own and with a fake `gh`:
[its README](e2e/README.md) explains how.

The `.mc` directory is private. It contains the team registry `.mc/teams-{year}.yaml`, and the lab session results `.mc/assignments-L{i}.yaml`, such as `.mc/assignments-L1.yaml` for Lab Session 1. This directory is ignored by Git.
