package rules

import "github.com/mincong-classroom/mc/common"

// Lab Session 1 (Containerize Pet Clinic).
//
// Every rule of the lab is graded by reading the work of the team: its report, the files of its
// repository, and its images on Docker Hub, which are run.

var mavenJarRuleSpec = common.RuleSpec{
	LabId:    "L1",
	Symbol:   "JAR",
	Name:     "JAR Creation Test",
	Exercice: "1",
	Description: `
The team is expected to create a JAR manually using a maven command and the
server should start locally under the port 8080. The team is also expected to
extract the JAR file to inspect the content of the MANIFEST.MF file.`,
}

var dockerfileRuleSpec = common.RuleSpec{
	LabId:    "L1",
	Symbol:   "DKF",
	Name:     "Dockerfile Test",
	Exercice: "2",
	Description: `
The team is expected to create a Dockerfile on the path "apps/spring-petclinic/Dockerfile". The Java
version should be an LTS version 17+ (17, 21, 25, ...), the minimum of Spring PetClinic, from the
distribution "eclipse-temurin", pulled from Docker Hub or a mirror of it. The port 8080 should be
exposed.
Note that the team can expose a container port at runtime even if the port is not specified with
the EXPOSE instruction in the Dockerfile. The EXPOSE instruction is primarily for documentation
purposes and does not control or enforce which ports are exposed at runtime. If the team did not
commit the content of the Dockerfile, but provided a correct Dockerfile implementation in the
report, we provide 80% of the score for this rule.`,
}

var dockerImageRuleSpec = common.RuleSpec{
	LabId:    "L1",
	Symbol:   "IMG",
	Name:     "Docker Image Test",
	Exercice: "3, 4",
	Description: `
The team is expected to build a Docker image using one single command. The
Docker image should support the platform linux/amd64; a multi-architecture
image, built for linux/amd64 and linux/arm64, is accepted too. It should be
published to Docker Hub under the mincongclassroom namespace:
mincongclassroom/spring-petclinic-{team}, where {team} is the team name in
lowercase. Inspection is done locally to verify the image published,
runnable, and accessible. This is a manual verification.`,
}

var dockerProcessRuleSpec = common.RuleSpec{
	LabId:    "L1",
	Symbol:   "DPS",
	Name:     "Docker Process Test",
	Exercice: "5",
	Description: `
The team is expected to inspect a Docker container using docker-ps. This is a
manual verification.`,
}

var dockerTeamRuleSpec = common.RuleSpec{
	LabId:    "L1",
	Symbol:   "DTM",
	Name:     "Docker Team Test",
	Exercice: "6",
	Description: `
The team is expected to update the source code to include their team name and
publish a new version of the Docker image under version 1.1.0. This is a manual
verification.`,
}
