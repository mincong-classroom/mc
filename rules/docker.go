package rules

import (
	"fmt"
	"os"
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
version should be 21+, from the distribution "eclipse-temurin". The port 8080 should be exposed.
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
	// note: we encountered an incident from DockerHub, so we switched to ECR public registry
	if strings.Contains(content, "FROM eclipse-temurin:21") ||
		strings.Contains(content, "FROM public.ecr.aws/docker/library/eclipse-temurin:21") {
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
