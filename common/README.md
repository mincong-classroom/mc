# Package `common`

The package `common` holds what the commands of `mc` share: the settings of the machine, the year
of the cohort, the team registry of the year, and the commit of `mc` that runs. The settings and
the registry are private files of `~/.mc`, which never go to Git: the registry holds the names of
the students.

## The Packages That Use It

3 packages import `common`, for the settings and the teams. Here are the types that they use:

```mermaid
---
title: "The types of common, and the packages that use them"
---
classDiagram
    direction LR
    namespace common {
        class Settings {
            Year int
            Rules string
            Agents Agents
        }
        class TeamRegistry {
            Students []Student
            Teams []Team
            EvaluationSheet string
        }
        class Team {
            Name string
            Members []TeamMember
            GetRepoName() string
            GetRepoPath() string
        }
        class TeamMember {
            Name string
            Github string
        }
    }
    class cmd {
        Execute() error
    }
    class cmd_git["cmd/git"] {
        GitCmd
    }
    class cmd_team["cmd/team"] {
        TeamCmd
    }
    TeamRegistry *-- Team
    Team *-- TeamMember
    cmd ..> Settings : CheckSettings, Year, RulesPath
    cmd ..> TeamRegistry : ListTeams, the evaluation sheet
    cmd_git ..> Team : ListTeams, the clones
    cmd_team ..> TeamRegistry : LoadRegistry, AddTeam
```

- `cmd` checks the settings before every command, then reads the year, the file of the rules, the
  teams, the URL of the evaluation spreadsheet, and the commit of `mc` that runs.
- `cmd/git` runs `git` in the clone of each team, which `Team` finds from its name.
- `cmd/team` reads the registry and changes it, for `mc team {team} provision`.

The other packages, such as `evidence`, `evaluation` and `sheet`, don't import `common`: `cmd`
passes them what they need, such as a team or the rules, so they never read `~/.mc` themselves.

## The Settings

`~/.mc/settings.yaml` is optional, and so is each of its keys:

```yaml
year: 2026
rules: ~/course/{year}/evaluation-rules.yaml
agents:
  report:
    run: claude -p "/report-lab {lab} {teams}"
    inputs: [.claude/skills/report-lab]
  grade:
    run: claude -p "/grade-lab {lab} {teams}"
    inputs: [.claude/skills/grade-lab]
```

An unknown key is an error, so that a typo doesn't go unnoticed, and `CheckSettings` runs before
every command. `CurrentSettings` reads the file once per run. The key `agents` holds the AI agents
of the steps of `mc eval run`: the command line of each one, and the files that it reads. The
former key `agents.eval_run`, a single agent for every step, is an error that says what to write
instead.

## The Team Registry

`LoadRegistry` reads the registry of the year: its students, its teams and the URL of the
evaluation spreadsheet. An unknown top-level key is an error, for the same reason as in the
settings. `ListTeams` and `FilterTeams` give its teams, and `AddTeam` and `SetTeamMembers` change
it while keeping its comments, for `mc team {team} provision`. A `Team` knows its repository, such
as `k8s-red`, and its clone, such as `~/github/mincong-classroom/k8s-red`.

`ParseName` and `SameName` read the names of the students in the format "LAST First", for
`mc team ls`, `provision` and `validate`.

## The Commit of `mc`

`Revision` returns the commit of `mc` that runs, from the build information of Go, such as
`8e39f42dfb73`, with `-dirty` for a local build with uncommitted changes. `mc eval gather-facts`
writes it in the front matter of the facts, and gathers the facts again when it changes. `StableRevision` tells
a commit without local changes, or a tag, whose two runs run the same code.

## Code

The files of the package:

| File | What it holds |
|---|---|
| `settings.go` | `Settings` and `Agents`, `CheckSettings`, `Year`, `RulesPath` and `ExpandHome` |
| `team.go` | `ConfigDir`, the team registry: `LoadRegistry`, `ListTeams`, `AddTeam`, `SetTeamMembers` |
| `types.go` | `TeamRegistry`, `Team` and `Student` |
| `name.go` | `ParseName` and `SameName`, the names of the students |
| `revision.go` | `Revision` and `StableRevision`, the commit of `mc` |

The tests use made-up teams and names only.
