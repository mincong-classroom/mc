package rules

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/mincong-classroom/mc/common"
)

type DockerfileRule struct{}

func (r DockerfileRule) Spec() common.RuleSpec {
	return common.RuleSpec{
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
}

func (r DockerfileRule) Run(team common.Team, _ string) common.RuleEvaluationResult {
	result := common.RuleEvaluationResult{
		Team:         team,
		RuleId:       r.Spec().Id(),
		Completeness: 0,
		Reason:       "",
		ExecError:    nil,
	}

	bytes, err := os.ReadFile(fmt.Sprintf("%s/apps/spring-petclinic/Dockerfile", team.GetRepoPath()))
	if err != nil {
		result.Reason = "The Dockerfile is missing"
		result.ExecError = err
		return result
	}

	content := string(bytes)
	if usesTemurinLTS(content) {
		result.Completeness += 0.8
	} else {
		result.Reason += "The Dockerfile does not use the correct Java version or distribution. "
	}

	if strings.Contains(content, "EXPOSE 8080") {
		result.Completeness += 0.2
	} else {
		result.Reason += "The Dockerfile does not expose the port 8080. "
	}

	if result.Completeness == 1 {
		result.Reason = "The Dockerfile is correct"
	}

	result.Reason = strings.TrimSpace(result.Reason)

	return result
}

type DockerImageRule struct{}

func (r DockerImageRule) Spec() common.RuleSpec {
	return common.RuleSpec{
		LabId:    "L1",
		Symbol:   "IMG",
		Name:     "Docker Image Test",
		Exercice: "3, 4",
		Description: `
The team is expected to build a Docker image using one single command. The
Docker image should be published to DockerHub under the mincongclassroom
namespace: mincongclassroom/spring-petclinic-{team}, where {team} is the team
name in lowercase. Inspection is done locally to verify the image published,
runnable, and accessible. This is a manual verification.`,
	}
}

func (r DockerImageRule) Run(team common.Team, _ string) common.RuleEvaluationResult {
	return common.RuleEvaluationResult{
		Team:         team,
		RuleId:       r.Spec().Id(),
		Completeness: 0,
		Reason:       "Check the report manually",
		ExecError:    nil,
	}
}

type DockerProcessRule struct{}

func (r DockerProcessRule) Spec() common.RuleSpec {
	return common.RuleSpec{
		LabId:    "L1",
		Symbol:   "DPS",
		Name:     "Docker Process Test",
		Exercice: "5",
		Description: `
The team is expected to inspect a Docker container using docker-ps. This is a
manual verification.`,
	}
}

func (r DockerProcessRule) Run(team common.Team, _ string) common.RuleEvaluationResult {
	return common.RuleEvaluationResult{
		Team:         team,
		RuleId:       r.Spec().Id(),
		Completeness: 0,
		Reason:       "Check the report manually",
		ExecError:    nil,
	}
}

type DockerTeamRule struct{}

func (r DockerTeamRule) Spec() common.RuleSpec {
	return common.RuleSpec{
		LabId:    "L1",
		Symbol:   "DTM",
		Name:     "Docker Team Test",
		Exercice: "6",
		Description: `
The team is expected to update the source code to include their team name and
publish a new version of the Docker image under version 1.1.0. This is a manual
verification.`,
	}
}

func (r DockerTeamRule) Run(team common.Team, _ string) common.RuleEvaluationResult {
	return common.RuleEvaluationResult{
		Team:         team,
		RuleId:       r.Spec().Id(),
		Completeness: 0,
		Reason:       "Check the report manually",
		ExecError:    nil,
	}
}

var dockerFrontendImageRuleSpec = common.RuleSpec{
	LabId:    "L3",
	Symbol:   "DIF",
	Exercice: "3",
	Name:     "Docker Frontend Image Test",
	Description: `
The team is expected to build a Docker image for the frontend service. The image
should be published to DockerHub under the mincongclassroom namespace:
mincongclassroom/spring-petclinic-api-gateway-{team}, where {team} is the team
name in lowercase. Inspection is done locally to verify the image published,
runnable, and accessible. The footer should display the team name. This is a
manual verification. The image tag should be 3.0 which corresponds to the Lab
Session 3.`,
}

var dockerCustomerImageRuleSpec = common.RuleSpec{
	LabId:    "L3",
	Symbol:   "DIC",
	Exercice: "3",
	Name:     "Docker Customer Image Test",
	Description: `
The team is expected to build a Docker image for the customer service. The image
should be published to DockerHub under the mincongclassroom namespace:
mincongclassroom/spring-petclinic-customers-service-{clinic}, where {clinic} is
the groupe name in lowercase. Inspection is done locally to verify the image
published, runnable, and accessible. It should contain a new customer. This is
a manual verification. The image tag should be 3.0 which corresponds to the Lab
Session 3.`,
}

var dockerVeterinarianImageRuleSpec = common.RuleSpec{
	LabId:    "L3",
	Symbol:   "DIV",
	Exercice: "3",
	Name:     "Docker Veterinarian Image Test",
	Description: `
The team is expected to build a Docker image for the veterinarian service. The
image should be published to DockerHub under the mincongclassroom namespace:
mincongclassroom/spring-petclinic-vets-service-{clinic}, where {clinic} is
the groupe name in lowercase. Inspection is done locally to verify the image
published, runnable, and accessible. It should contain a new veterinarian.
This is a manual verification. The image tag should be 3.0 which corresponds to
the Lab Session 3.`,
}

// temurinFrom matches a FROM instruction on the image "eclipse-temurin", from Docker Hub or a
// mirror of it, such as "public.ecr.aws/docker/library/" or "mirror.gcr.io/library/" (used during
// incidents of Docker Hub), and captures the feature release of its tag: "21" in
// "eclipse-temurin:21-jre-alpine".
var temurinFrom = regexp.MustCompile(`(?im)^\s*FROM\s+(?:--platform=\S+\s+)?(?:\S+/)?eclipse-temurin:(\d+)`)

// isJavaLTS tells whether a feature release of Java is an LTS that Spring PetClinic runs on: 17 is
// its minimum, and an LTS comes every four releases since then (17, 21, 25, 29, ...).
func isJavaLTS(release int) bool {
	return release >= 17 && (release-17)%4 == 0
}

// usesTemurinLTS tells whether a Dockerfile builds on an image of "eclipse-temurin" in an LTS
// version supported by Spring PetClinic.
func usesTemurinLTS(dockerfile string) bool {
	for _, match := range temurinFrom.FindAllStringSubmatch(dockerfile, -1) {
		if release, err := strconv.Atoi(match[1]); err == nil && isJavaLTS(release) {
			return true
		}
	}
	return false
}
