# grading

The AI teaching assistant for auto-grading assignments 🤖

## Installation

Install Golang:

```sh
brew install go

go version
# go version go1.23.3 darwin/arm64
```

Then build the CLI:

```sh
go mod tidy
go build -o mc
```

## Usage

```sh
./mc help
```

```
Mincong Classroom (mc) is a command line interface for grading student
assignments in the Software Containerization and Orchestration course.

Usage:
  mc [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  grade       Grade assignments
  help        Help about any command
  info        Display CLI information
  rule        List grading rules
  team        List all teams

Flags:
  -h, --help   help for mc

Use "mc [command] --help" for more information about a command.
```