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
  team        Manage the teams

Flags:
  -h, --help               help for mc
      --team-file string   Team registry to use instead of ~/.mc/teams-{year}.yaml, e.g. a test registry to try the commands

Use "mc [command] --help" for more information about a command.
```

## Team

The teams are registered in a private YAML file, the team registry `~/.mc/teams-{year}.yaml`.
A team has a name, lowercase letters only (such as a color), and at most two members. A team
can have no members yet: it is created before the course, and its members are added later.

```yaml
teams:
  - name: red
    members:
      - name: "SMITH, John"   # "LAST, First"
        github: jsmith        # GitHub username
  - name: orange
    members: []               # not taken yet
```

The school list `~/.mc/students-{year}.yaml` is optional and informational. It lists the students of
the year, with the same names as in the registry: `mc team ls` lists the ones who are not in a team
yet. It never blocks a validation or a provisioning, which only rely on the registry and GitHub.

```yaml
students:
  - name: "SMITH, John"
  - name: "DOE, Jane"
```

The year is the current cohort, 2026. Set the environment variable `MC_YEAR` to use another one,
e.g. `MC_YEAR=2025 mc grade`.

The global flag `--team-file` replaces the team registry for any command, e.g. to rehearse with a
test registry before the course. `provision` still acts on the real organization: add `--dry-run`.

```sh
mc team ls --team-file ~/.mc/test-teams-2026.yaml
mc team provision --dry-run --team-file ~/.mc/test-teams-2026.yaml
```

Each team gets a private repository `k8s-{team}` in the GitHub organization, generated from the
template repository `mincong-classroom/containers`, and a secret GitHub team `{team}` with push
access to it. With the organization's base permission set to `none`, the members of a team only
see their own repository. The GitHub calls go through the `gh` CLI, logged in with the scopes
`repo` and `admin:org`:

```sh
gh auth refresh -s admin:org
```

The commands are `mc team <action>`, and `mc team <team> <action>` for the actions on one team
of the registry: each team is a subcommand. `mc team --help` lists the teams with their members,
and `mc team red --help` the actions on the team `red`. The names `ls` and `provision` are
reserved.

| Command | What it does |
|---|---|
| `mc team ls [--json]` | Lists the teams with their members, their status on GitHub and their validation problems, then the students not in a team yet |
| `mc team provision [--dry-run]` | Asks for a team, then creates its repository and its GitHub team, grants the team push access, and adds the members, which invites them to the organization; then asks for the next team |
| `mc team red validate [--json]` | Checks the name format and its uniqueness, at most 2 members, each member in one team only, each GitHub username existing; prints the display name of each GitHub account, for the students to confirm it |
| `mc team red status [--json]` | Shows whether the repository and the GitHub team exist, the access of the team to the repository, and whether each member is `active` or `pending` (invitation not accepted yet) |

`provision` is interactive: it describes each step with the `gh` command it runs, and runs it
only once confirmed. The steps already done are skipped, so a team can be provisioned again,
e.g. once its members are known. A team that is not valid is not provisioned. The registry is
read again before each team, so it can be edited in between. With `--dry-run`, the steps are
described and confirmed, but nothing runs.

```
Teams: red, orange, yellow
Team to provision (empty to quit): red

== Team red
[1/5] Create the private repository mincong-classroom/k8s-red from the template mincong-classroom/containers
      ✓ the repository exists
[2/5] Create the secret GitHub team red
      ✓ the GitHub team exists
[3/5] Grant the GitHub team red push access to k8s-red
      ✓ the GitHub team has push access
[4/5] Add SMITH, John (@jsmith), "John Smith" on GitHub, to the GitHub team red: GitHub invites them to the organization by email
      $ gh api -X PUT orgs/mincong-classroom/teams/red/memberships/jsmith -f role=member
      Run it? [y]es, [n]o (skip the team), [a]ll (yes to the next steps of the team), [q]uit:
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


L4_NSC: Namespace Creation Test (Ex 1)

    The team is expected to create a new namespace called "classroom" and list all
    the existing namespaces in the cluster. The namespace can be created either
    imperatively with "kubectl create namespace" or declaratively via a YAML
    manifest applied with "kubectl apply". This is a manual verification based on the
    evidence provided in the report.


L4_TIS: Team Info Server Deployment Test (Ex 2)

    The team is expected to deploy and expose the classroom application
    "team-info-server" (image mincongclassroom/team-info-server) in the
    "classroom" namespace. They must create a Deployment named "team-info" with 1
    replica and a ClusterIP Service named "team-info" exposing port 80 and
    targeting the container port 8090, stored in a single manifest committed at
    "k8s/lab-4/app-team-info.yaml". The web server fails to start until the
    required TEAM_ID environment variable is set (the same style of fix as Lab
    Session 1); the optional TEAM_MEMBERS variable lists the members shown on the
    About page. The report must also cover the two validation scenarios: querying
    the Service from a temporary Pod with the current context set to the
    "classroom" namespace, then with the context set to "default". It is expected
    to explain how the namespace is switched, how the resources of that namespace
    are verified, how the temporary Pod is created, which URL is used for the HTTP
    request and what it means for the DNS, and how the response is analysed — in
    particular that the short name "team-info" only resolves from within
    "classroom", while "team-info.classroom" or the fully-qualified name
    "team-info.classroom.svc.cluster.local" is required from "default". This rule
    applies the manifest, waits for the Pod, and queries the Service: it checks
    that the manifest is committed (0.2), the Service is reachable (0.3), TEAM_ID
    matches the team name (0.3), and the team members are listed (0.2). The
    cross-namespace DNS write-up is reviewed manually.


L4_AGR: API Gateway About Route Test (Ex 3)

    The team is expected to make the PetClinic "About" page work by configuring
    Kubernetes networking only (no frontend or Java code). They must route
    requests for "/api/about" from the API Gateway to the "team-info" Service in
    the "classroom" namespace, using cross-namespace DNS ("team-info.classroom",
    or the fully-qualified name "team-info.classroom.svc.cluster.local") rather
    than hard-coding the team information. The route belongs to the
    "api-gateway-config" ConfigMap: it matches the predicate "Path=/api/about/**"
    and strips the two prefix segments ("StripPrefix=2") so that the request
    reaches "/" on the Team Info Server. The updated manifest must be committed at
    "k8s/lab-4/microservices.yaml". Validation: opening
    http://localhost:8080/#!/about displays the team, the team members and the
    source code served by the Team Info Server, and "curl
    http://localhost:8080/api/about/" returns the expected JSON. The report is
    expected to trace the request through the logs: without a matching route the
    API Gateway logs no "Route matched" line and returns 404, whereas a matching
    route pointing at an unreachable backend surfaces as 405 — the circuit breaker
    forwards the GET to the POST-only "/fallback" endpoint. This is a manual
    verification.


L5_SEC: Kubernetes Secret Test (Ex 1)

    The team is expected to create a Kubernetes Secret to an API key as sensitive
    data in the cluster. The secret must be named as "openai" and data entry should
    be "api-key". The value should be encoded in base64 format. And the resource
    should be applied to the "dev" namespace. The team should verify the result by
    putting the analysis in the report. A -30% penalty will be applied if the team
    exposes the API key in the report. This is not acceptable: they have been
    warned during the lecture; and in the description of the lab exercise. This is
    a manual verification.


L5_GAI: PetClinic GenAI Service Test (Ex 2)

    The team is expected to integrate the GenAI service to the existing
    microservice stack. This includes the configuration of the Service, Deployment,
    the API routes in the API Gateway, the API key stored in the Secret, and the
    related troubleshooting to ensure that the entire solution works. This is a
    manual verification.


```

## Key Components

The `cmd` directory contains all the commands exposed in the command line interface. Each command is registered in the `root.go`.

The `rules` directory contains all the rules for the auto-grading.

The `github` directory manages the GitHub organization (repositories, GitHub teams and their members) through the `gh` CLI.

The `.mc` directory is private. It contains the team registry `.mc/teams-{year}.yaml`, the school list `.mc/students-{year}.yaml` and the lab session results `.mc/assignments-L{i}.yaml`, such as `.mc/assignments-L1.yaml` for Lab Session 1. This directory is ignored by Git.
