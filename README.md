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


L2_CTL: Kubernetes Control Plane Test (Ex 1)

    The team is expected to list all the Pods running in all namespaces in
    Kubernetes. Then, list all the nodes available in the cluster. It allows the
    students to get familiar with the Kubernetes and ensure that the command line
    tool kubectl is properly installed on their local machines.


L2_RUN: Kubernetes Run Nginx Pod Test (Ex 2)

    The team is expected to create a new Pod using the command kubectl-run. The Pod
    needs to be running and accessible. The students should provide evidence of the
    HTTP response from the Pod, such as a screenshot or the command output. A list
    of fields are expected to be filled in the report for describing the
    characteristics of the Pod. Also, the resource should be deleted after the
    test.


L2_NGY: Nginx YAML Test (Ex 3)

    The team is expected to create a new Pod running with Nginx using a kubectl-apply
    command. This Pod should be reachable using the port 80 and should be named as
    "nginx". The manifest should be saved under the path k8s/pod-nginx.yaml
    of the Git repository. Also, a team label should be added to the Pod definition.


L3_JVY: Java YAML Test (Ex 4)

    The team is expected to create a new pod running with Java using a kubectl-apply
    command. This pod should be reachable using the port 8080 and should be named as
    "spring-petclinic". The manifest should be saved under the path k8s/pod-petclinic.yaml
    of the Git repository. The Pod should contain 2 labels, app=spring-petclinic and
    team=${team}. The Pod must be up and running.


L2_OJP: Kubernetes Operate Java Pod Test (Ex 5)

    The team is expected to perform basic operations on the Java Pod they created.
    These operations include executing a command inside the Pod to get the process
    ID (PID) of the Java application, retrieving logs from the Pod, and finding the
    Pod using kubectl-get with label selectors. The students should provide evidence
    of each operation, such as command outputs or screenshots.


L2_FBP: Kubernetes Fix Broken Pod Test (Ex 6)

    The team is expected to troubleshoot and fix a broken Pod provided by the
    teacher. The Pod is intentionally misconfigured to simulate common issues that
    may arise in a Kubernetes environment. The students need to identify the two
    problems, including the incorrect Docker image and the missing team name in the
    environment variables. After fixing the issues, the Pod should be up and
    running.


L3_RST: ReplicaSet Test (Ex 1)

    The team is expected to create a new ReplicaSet and put the definition under the path
    "k8s/replicaset-petclinic.yaml" of the Git repository. Operations should be assessed
    manually by the teacher. The container should use port 8080 to receive incoming
    traffic. The container name should be "main". The docker image should be the
    one published by the team in the previous lab, i.e.
    "mincongclassroom/spring-petclinic-{team}". The team should use 2 labels:
    app=spring-petclinic and team=<team-name>. The ReplicaSet should be created
    successfully and the Pods should be running. Then, the team should describe how
    they scale the ReplicaSet. Most importantly, they should explain the behavior
    of the system when they delete a Pod managed by the ReplicaSet.


L3_DPL: Deployment Test (Ex 2)

    The team is expected to create a new Deployment and put the definition under the path
    "k8s/deployment-petclinic.yaml" of the Git repository. Operations should be assessed
    manually by the teacher. Most of the requirements are similar to the ReplicaSet.
    That is, the container should use port 8080 to receive incoming
    traffic; the container name should be "main"; the team should use 2 labels:
    petclinicDeploymentManifestPath),
    app=spring-petclinic and team=<team-name>. Then, they are expected to create a
    environment variable "TEAM" with the value in lowercase and observe the rollout
    history. Finally, they should disrupt the Deployment and observe what happens.


L3_DIF: Docker Frontend Image Test (Ex 3)

    The team is expected to build a Docker image for the frontend service. The image
    should be published to DockerHub under the mincongclassroom namespace:
    mincongclassroom/spring-petclinic-api-gateway-{team}, where {team} is the team
    name in lowercase. Inspection is done locally to verify the image published,
    runnable, and accessible. The footer should display the team name. This is a
    manual verification. The image tag should be 3.0 which corresponds to the Lab
    Session 3.


L3_DIC: Docker Customer Image Test (Ex 3)

    The team is expected to build a Docker image for the customer service. The image
    should be published to DockerHub under the mincongclassroom namespace:
    mincongclassroom/spring-petclinic-customers-service-{clinic}, where {clinic} is
    the groupe name in lowercase. Inspection is done locally to verify the image
    published, runnable, and accessible. It should contain a new customer. This is
    a manual verification. The image tag should be 3.0 which corresponds to the Lab
    Session 3.


L3_DIV: Docker Veterinarian Image Test (Ex 3)

    The team is expected to build a Docker image for the veterinarian service. The
    image should be published to DockerHub under the mincongclassroom namespace:
    mincongclassroom/spring-petclinic-vets-service-{clinic}, where {clinic} is
    the groupe name in lowercase. Inspection is done locally to verify the image
    published, runnable, and accessible. It should contain a new veterinarian.
    This is a manual verification. The image tag should be 3.0 which corresponds to
    the Lab Session 3.


L4_SVC: Service Test (Ex 3)

    The team is expected to create a new Service and put the definition under the path
    k8s/deployment-petclinic.yaml of the Git repository. Operations should be assessed
    manually by the professor.


```

## Key Components

The `cmd` directory contains all the commands exposed in the command line interface. Each command is registered in the `root.go`.

The `rules` directory contains all the rules for the auto-grading.

The `.mc` directory is private. It contains all the team information `.mc/teams.yaml` and the lab session results `.mc/assignments-L{i}.yaml`, such as `.mc/assignments-L1.yaml` for Lab Session 1. This directory is ignored by Git.
