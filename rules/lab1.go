package rules

import "github.com/mincong-classroom/mc/common"

// Lab Session 1 (Containerize Pet Clinic).
//
// Every rule of the lab is graded by reading the work of the team: its report, the files of its
// repository, and its images on Docker Hub, which are run. The descriptions say what to check.

var mavenJarRuleSpec = common.RuleSpec{
	LabId:    "L1",
	Symbol:   "JAR",
	Exercice: "1",
	Name:     "JAR Creation Test",
	Description: `
The team is expected to build the JAR of Spring PetClinic from the directory
"apps/spring-petclinic", with the Maven wrapper of the project ("./mvnw" on macOS,
Linux and Git Bash, ".\mvnw.cmd" in PowerShell or the Command Prompt) or with
Maven itself, and a JDK of a long-term support (LTS) version from 17. Then, to
start the server with the command "java -jar" and say what the browser shows at
http://localhost:8080: the home page of Spring PetClinic. The report gives these
commands. The team is also expected to look inside the JAR and paste the content
of its file "META-INF/MANIFEST.MF" in the report, such as its "Main-Class" and
"Start-Class".`,
}

var dockerfileRuleSpec = common.RuleSpec{
	LabId:    "L1",
	Symbol:   "DKF",
	Exercice: "2",
	Name:     "Dockerfile Test",
	Description: `
The team is expected to create a Dockerfile on the path "apps/spring-petclinic/Dockerfile",
committed and pushed. The Java version should be an LTS version 17+ (17, 21, 25, ...), the minimum
of Spring PetClinic, from the distribution "eclipse-temurin", pulled from Docker Hub or a mirror of
it (80%). The port 8080 should be exposed (20%). The JAR should be built outside of Docker, with
the command of Exercise 1, and copied into the image: a multi-stage build, which builds the JAR
inside Docker, is reviewed by the teacher.
Note that the team can expose a container port at runtime even if the port is not specified with
the EXPOSE instruction in the Dockerfile. The EXPOSE instruction is primarily for documentation
purposes and does not control or enforce which ports are exposed at runtime. If the team did not
commit the content of the Dockerfile, but provided a correct Dockerfile implementation in the
report, we provide 80% of the score for this rule.`,
}

var dockerImageRuleSpec = common.RuleSpec{
	LabId:    "L1",
	Symbol:   "IMG",
	Exercice: "2, 3, 4",
	Name:     "Docker Image Test",
	Description: `
The team is expected to build a Docker image using one single command, run from
the root of the Git repository (Exercise 2). The Docker image should support the
platform linux/amd64; a multi-architecture image, built for linux/amd64 and
linux/arm64, is accepted too. Then, to run a container of the image with
"docker run", receiving the HTTP requests on the port 8080, and to paste some
logs of Spring Boot showing that the application started (Exercise 3). Finally,
to publish the image to Docker Hub under the mincongclassroom namespace, with
the version 1.0.0: mincongclassroom/spring-petclinic-{team}:1.0.0, where {team}
is the team name in lowercase (Exercise 4). The report gives the three commands
and the logs. The image published is pulled and run to verify that it is
runnable and accessible.`,
}

var dockerProcessRuleSpec = common.RuleSpec{
	LabId:    "L1",
	Symbol:   "DPS",
	Exercice: "5",
	Name:     "Docker Process Test",
	Description: `
The team is expected to inspect its running Docker container using docker-ps,
and to find out its container ID. The report gives the command and the ID.`,
}

var dockerTeamRuleSpec = common.RuleSpec{
	LabId:    "L1",
	Symbol:   "DTM",
	Exercice: "6",
	Name:     "Docker Team Test",
	Description: `
The team is expected to update the source code to include their team name in
the footer of the Spring PetClinic UI, and publish a new version of the Docker
image under version 1.1.0: mincongclassroom/spring-petclinic-{team}:1.1.0. The
image is pulled and run, and its pages are opened in a browser: the footer
shows the name of the team, and the pages of the application still work.`,
}
