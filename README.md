# mc

The CLI tool for Mincong Classroom (mc). It's mainly for auto-grading assignments 🤖

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
go build -o dist/mc
```

## Usage

```sh
mc help
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

## Rule

These are the rules which are part of the auto-grading. If some rules fail to evaluate, it require manual correction later on.

```sh
mc rule
```

```
L1_JAR: JAR Creation Test (Ex 1.1)

    The team is expected to create a JAR manually using a maven command and the
    server should start locally under the port 8080.


L1_DKF: Dockerfile Test (Ex 1.2)

    The team is expected to create a Dockerfile on the path
    "weekend-server/Dockerfile". The Java version should be 21, from the
    distribution "eclipse-temurin". The port 8080 should be exposed. Note that
    you can expose a container port at runtime even if the port is not specified
    with the EXPOSE instruction in the Dockerfile. The EXPOSE instruction is
    primarily for documentation purposes and does not control or enforce which
    ports are exposed at runtime.


L1_IMG: Docker Image Test (Ex 1.3+)

    The team is expected to build a Docker image using one single command. The
    inspection should be done locally to verify the image is successfully created
    and runnable. This rule includes the exercise 1.4, 1.5, 1.6 as well. This is
    a manual verification.


L1_SQL: SQL Init Test (Ex 2.1.2)

    The team is expected to complete the SQL script located at the path
    "weekend-mysql/init.sql". The script should contain an "INSERT INTO" statement
    followed by 7 values, either using VARCHAR or INT as key for the table
    "mappings" or a similar table.
```