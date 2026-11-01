# Contributing

## Packages

The commands of `mc` are in the package `cmd`, and the work is done in the packages below it. The
diagram shows each package with its main types and functions, and an arrow from a package to each
package that it imports:

```mermaid
classDiagram
    direction TB
    class cmd {
        RootCmd
        Execute() error
    }
    class cmd_git["cmd/git"] {
        GitCmd
    }
    class cmd_team["cmd/team"] {
        TeamCmd
        AddTeamCommands(args)
    }
    class rules {
        <<deprecated>>
        Grader
        NewGrader() Grader
    }
    class common {
        Settings
        Team
        Year() int
        RulesPath(file) string
        Revision() string
        ListTeams()
    }
    class github {
        CLI
        Org
    }
    class rubric {
        Rubric
        Load(path) Rubric
        Check(opts) Problems
        Render(w, opts) error
    }
    class evidence {
        Gather(team, opts) Result
        ReadFacts(report) Facts
        Template
        DockerHub
    }
    class evaluation {
        File
        Load(path) File
        Check(opts) Problems
        Teams(labDir) []string
        Sync(opts) SyncResult
    }
    class sheet {
        File
        Load(path) File
        Blocks(f) []Block
        Verify(read, exported) []string
    }
    class annotate {
        Mark
        Read(ctx, img) []Line
        Draw(img, marks, opts) RGBA
    }
    cmd ..> cmd_git
    cmd ..> cmd_team
    cmd ..> rules
    cmd ..> common
    cmd ..> evidence
    cmd ..> evaluation
    cmd ..> annotate
    cmd ..> sheet
    cmd ..> github
    cmd ..> rubric
    cmd_git ..> common
    cmd_team ..> common
    cmd_team ..> github
    rules ..> common
    evidence ..> github
    evidence ..> rubric
    evaluation ..> rubric
    evaluation ..> sheet
```

What each package does, in the order of the diagram:

| Package | What it does |
|---|---|
| `cmd` | Wires the commands with Cobra: `Execute()` checks the settings, adds the commands of the teams, then runs the command. |
| `cmd/git` | The commands on the clones of the teams: `mc git clone`, `pull`, `show` and `exec`. |
| `cmd/team` | The commands on the teams: `mc team ls`, `mc team <team> provision`, etc. The only package that changes GitHub. |
| `evidence` | Gathers the evidence of a lab for a team: the rules of the lab from `rubric`, the pushes from `github`, the files of the clone with `git`, and the tags on Docker Hub. It takes the rules and the team as arguments, so it doesn't import `common`. |
| `evaluation` | Reads the evaluation of a lab for a team, one YAML file per team and lab, as the evaluator writes it. It checks it against the rules of `rubric`, and against the commit of the facts of `mc evidence`, which `cmd` reads and passes to it. It computes the cells that write the evaluations of a lab in the tab Evaluation, as a file of `sheet`. |
| `annotate` | Finds a text in a screenshot with `tesseract`, and draws numbered boxes around what the evaluator found. It imports no package of `mc`. |
| `sheet` | Keeps the data of a tab of the evaluation spreadsheet in JSON files, one object per row: the blocks of cells to paste, the script that reads the tab in a browser, and the check of the tab against the last export. It imports no package of `mc`. |
| `rules` | The grading engine of 2025, which `mc grade` runs. Deprecated. |
| `common` | What every command shares: the settings of `~/.mc/settings.yaml`, the year, the commit of `mc` that runs, the team registry and the paths of the clones. It imports no package of `mc`. |
| `github` | Runs the CLI `gh` on the organization: the repositories, the teams and their members, and the pushes to `main`. |
| `rubric` | Reads the YAML of the evaluation rules, checks it and renders its page. It imports no package of `mc`: its tests only need the made-up file of `rubric/testdata/`. |
| `e2e` | Builds the binary `mc` and runs it, with a made-up `~/.mc` and a fake `gh`. It imports no package, so the diagram leaves it out. |
