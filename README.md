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
L1_JAR: JAR Creation Test (Ex 1)
  
    The team is expected to create a JAR manually using a maven command and the
    server should start locally under the port 8080. The team is also expected to
    extract the JAR file to inspect the content of the MANIFEST.MF file.


L1_DKF: Dockerfile Test (Ex 2)
  
    The team is expected to create a Dockerfile on the path "apps/spring-petclinic/Dockerfile". The Java
    version should be 21+, from the distribution "eclipse-temurin". The port 8080 should be exposed.
    Note that the team can expose a container port at runtime even if the port is not specified with
    the EXPOSE instruction in the Dockerfile. The EXPOSE instruction is primarily for documentation
    purposes and does not control or enforce which ports are exposed at runtime. If the team did not
    commit the content of the Dockerfile, but provided a correct Dockerfile implementation in the
    report, we provide 80% of the score for this rule.


L1_IMG: Docker Image Test (Ex 3, 4)
  
    The team is expected to build a Docker image using one single command. The
    Docker image should be published to DockerHub under the mincongclassroom
    namespace: mincongclassroom/spring-petclinic-{team}, where {team} is the team
    name in lowercase. Inspection is done locally to verify the image published,
    runnable, and accessible. This is a manual verification.


L1_DPS: Docker Process Test (Ex 5)
  
    The team is expected to inspect a Docker container using docker-ps. This is a
    manual verification.


L1_DTM: Docker Team Test (Ex 6)
  
    The team is expected to update the source code to include their team name and
    publish a new version of the Docker image under version 1.1.0. This is a manual
    verification.


L2_MST: Maven Setup Test (Ex 1)
  
    The team is expected to run unit tests with Maven in GitHub Actions on the path
    ".github/workflows/app.yml". It should contain the keyword "mvn"


L2_RGT: Registry Test (Ex 1)
  
    The team is expected to upload the Docker image to the registry (Dockerhub).
    This is the key of the whole lab session. By completing this exercise, it means
    that the students were able to define the Dockerfile correctly, build the
    Docker image, connect to the Dockerhub, and push the image with the right tag.
    Else, teacher (Mincong) should check the steps by breaking it down into
    multiple steps. Two kinds of tags are published to the registry, the "latest"
    kind and the "commit" kind.


L2_DST: Docker Setup Test (Ex 3)
  
    The team is expected to build a Docker image and publish it to the Docker
    registry. The docker login should be done by retrieving the username and
    password from the secrets, such as "secrets.DOCKER_USERNAME". This is probably
    done using the GitHub Action "docker/login-action" but other approaches are
    fine too.


L3_NGY: Nginx Yaml Test (Ex 3)
  
    The team is expected to create a new pod running with Nginx using a kubectl-apply
    command. This pod should be reachable using the port 80 and should be named as
    "nginx". The manifest should be saved under the path k8s/pod-nginx.yaml
    of the Git repository.


L3_JVY: Java Yaml Test (Ex 4)
  
    The team is expected to create a new pod running with Java using a kubectl-apply
    command. This pod should be reachable using the port 8080 and should be named as
    "weekend-server". The manifest should be saved under the path k8s/pod-weekend-server.yaml
    of the Git repository. The HTTP response of the root API (/) should contains the
    team and authors. The Docker image should be pulled from the Docker Hub repository
    "mincongclassroom/weekend-server-${team}", such as "mincongclassroom/weekend-server-red".


L4_RST: ReplicaSet Test (Ex 1)
  
    The team is expected to create a new ReplicaSet and put the definition under the path
    k8s/replicaset-nginx.yaml of the Git repository. Operations should be assessed
    manually by the professor.


L4_DPL: Deployment Test (Ex 2)
  
    The team is expected to create a new Deployment and put the definition under the path
    k8s/deployment-weekend-server.yaml of the Git repository. Operations should be assessed
    manually by the professor.


L4_SVC: Service Test (Ex 3)
  
    The team is expected to create a new Service and put the definition under the path
    k8s/deployment-weekend-server.yaml of the Git repository. Operations should be assessed
    manually by the professor.

```

## Key Components

The `cmd` directory contains all the commands exposed in the command line interface. Each command is registered in the `root.go`.

The `rules` directory contains all the rules for the auto-grading.

The `.mc` directory is private. It contains all the team information `.mc/teams.yaml` and the lab session results `.mc/assignments-L{i}.yaml`, such as `.mc/assignments-L1.yaml` for Lab Session 1. This directory is ignored by Git.
