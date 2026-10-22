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

The year is the current cohort, 2026. Set the environment variable `MC_YEAR` to use another one,
e.g. `MC_YEAR=2025 mc grade`.

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

These are the grading rules. `mc grade` runs the automated ones; the others are graded by reading
the work of the team: its report, the files of its repository and its Docker images. All the rules
of Lab 1 are graded this way. If some rules fail to evaluate, it requires manual correction later
on.

```sh
mc rule          # all the rules
mc rule -l L1    # the rules of Lab 1 only
```

```
L1_JAR: JAR Creation Test (Ex 1)

    The team is expected to create a JAR manually using a Maven command, either
    using Maven directly or via a Maven wrapper script. The server should start
    locally on port 8080. If, for some reason, port 8080 is occupied, the team can
    also choose another port. The team is also expected to extract the JAR file to
    inspect the content of the MANIFEST.MF file and paste the content in the
    report. The lab accepts any LTS from 17, with 25 recommended.


L1_DKF: Dockerfile Test (Ex 2)

    The team is expected to create a Dockerfile on the path "apps/spring-petclinic/Dockerfile". The Java
    version should be an LTS version 17+ (17, 21, 25, ...), the minimum of Spring PetClinic, from the
    distribution "eclipse-temurin", pulled from Docker Hub or a mirror of it (60%). The port 8080 should
    be exposed (20%). The team is also expected to build a Docker image using one single command (20%).
    The Docker image should support the platform linux/amd64; a multi-architecture image, built for
    linux/amd64 and linux/arm64, is accepted too.
    Note that the team can expose a container port at runtime even if the port is not specified with
    the EXPOSE instruction in the Dockerfile. The EXPOSE instruction is primarily for documentation
    purposes and does not control or enforce which ports are exposed at runtime. If the team did not
    commit the content of the Dockerfile, but provided a correct Dockerfile implementation in the
    report, we provide 80% of the score for this rule.


L1_RUN: Docker Run Test (Ex 3)

    The team is expected to run a container using docker-run, receiving the HTTP
    requests on the port 8080, and to paste some logs of Spring Boot in the report.
    This is a manual verification.


L1_IMG: Docker Image Test (Ex 4)

    The team is expected to publish the Docker image to Docker Hub under the
    mincongclassroom namespace, with the version 1.0.0:
    mincongclassroom/spring-petclinic-{team}:1.0.0, where {team} is the team name
    in lowercase. Inspection is done locally to verify the image published,
    runnable, and accessible. This is a manual verification.


L1_DPS: Docker Process Test (Ex 5)

    The team is expected to inspect a Docker container using docker-ps, and find
    out its container ID. This is a manual verification.


L1_DTM: Docker Team Test (Ex 6)

    The team is expected to update the source code to include their team name in
    the footer of the Spring PetClinic UI, and publish a new version of the Docker
    image under version 1.1.0: mincongclassroom/spring-petclinic-{team}:1.1.0.
    Inspection is done locally: the image is run, and the footer shows the team
    name in the browser. This is a manual verification.


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
    "nginx". The manifest should be saved under the path k8s/lab-2/pod-nginx.yaml
    of the Git repository. Also, a team label should be added to the Pod definition.


L2_JVY: Java YAML Test (Ex 4)

    The team is expected to create a new pod running with Java using a kubectl-apply
    command. This pod should be reachable using the port 8080 and should be named as
    "spring-petclinic". The manifest should be saved under the path k8s/lab-2/pod-petclinic.yaml
    of the Git repository. The Pod should contain 2 labels, app=spring-petclinic and
    team=${team}. The Pod must be up and running. This rule applies the manifest,
    waits for the Pod, and reads its home page (80%), then checks the team name on
    the page (10%) and the two labels (5% each).


L2_OJP: Kubernetes Operate Java Pod Test (Ex 5)

    The team is expected to perform basic operations on the Java Pod they created.
    These operations include executing a command inside the Pod to get the process
    ID (PID) of the Java application, retrieving logs from the Pod, and finding the
    Pod using kubectl-get with label selectors. The students should provide evidence
    of each operation, such as command outputs or screenshots.


L2_FBP: Kubernetes Fix Broken Pod Test (Ex 6)

    The team is expected to troubleshoot and fix a broken Pod named "team-info-server",
    whose manifest is provided in the Git repository under the path
    k8s/lab-2/pod-team-info-server.yaml. The Pod is intentionally misconfigured to
    simulate common issues that may arise in a Kubernetes environment. The students
    need to identify the two problems, a tag of the Docker image that does not
    exist and a missing environment variable, and fix them in that manifest,
    keeping the same image. After fixing the issues, the Pod should be up and
    running.


L3_RST: ReplicaSet Test (Ex 1)

    The team is expected to create a new ReplicaSet and put the definition under the path
    "k8s/lab-3/replicaset-petclinic.yaml" of the Git repository. Operations should be assessed
    manually by the teacher. The container should use port 8080 to receive incoming
    traffic. The container name should be "main". The docker image should be the
    one published by the team in Lab 1, with the team name in the footer, i.e.
    "mincongclassroom/spring-petclinic-{team}:1.1.0". The team should use 2 labels:
    app=spring-petclinic and team=<team-name>. The ReplicaSet should be created
    successfully and the Pods should be running. Then, the team should describe how
    they scale the ReplicaSet. Most importantly, they should explain the behavior
    of the system when they delete a Pod managed by the ReplicaSet.


L3_DPL: Deployment Test (Ex 2)

    The team is expected to create a new Deployment and put the definition under the path
    "k8s/lab-3/deployment-petclinic.yaml" of the Git repository. Operations should be assessed
    manually by the teacher. Most of the requirements are similar to the ReplicaSet.
    That is, the container should use port 8080 to receive incoming
    traffic; the container name should be "main"; the docker image should be
    "mincongclassroom/spring-petclinic-{team}:1.1.0"; the team should use 2 labels:
    app=spring-petclinic and team=<team-name>. Then, they are expected to create a
    environment variable "TEAM" with the value in lowercase and observe the rollout
    history. Finally, they should disrupt the Deployment and observe what happens.


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
    "petclinic-config" ConfigMap: it matches the predicate "Path=/api/about/**"
    and strips the two prefix segments ("StripPrefix=2") so that the request
    reaches "/" on the Team Info Server. The updated manifest must be committed at
    "k8s/lab-4/microservices.yaml". Validation: opening
    http://localhost:8080/about displays the team, the team members and the source
    code served by the Team Info Server, and "curl
    http://localhost:8080/api/about/" returns the expected JSON. The report is
    expected to trace the request through the logs: without a matching route the
    API Gateway logs no "Route matched" line and returns 404, whereas a matching
    route pointing at an unreachable backend surfaces as 503 — the circuit breaker
    forwards the request to the "/fallback" endpoint of the gateway. The "About"
    page shows the status and the body of the response in both cases. This is a
    manual verification.


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

The `.mc` directory is private. It contains the team registry `.mc/teams-{year}.yaml`, and the lab session results `.mc/assignments-L{i}.yaml`, such as `.mc/assignments-L1.yaml` for Lab Session 1. This directory is ignored by Git.
