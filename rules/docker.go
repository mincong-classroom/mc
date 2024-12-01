package rules

import (
	"fmt"
	"os"
	"strings"

	"github.com/mincong-classroom/mc/common"
)

type DockerfileRule struct {
	spec common.RuleSpec
}

func NewDockerfileRule() DockerfileRule {
	return DockerfileRule{
		spec: common.RuleSpec{
			LabId:    "L1",
			Symbol:   "DKF",
			Name:     "Dockerfile Test",
			Exercice: "1.2",
			Description: `
  The team is expected to create a Dockerfile on the path
  "weekend-server/Dockerfile". The Java version should be 21, from the
  distribution "eclipse-temurin". The port 8080 should be exposed. Note that
  you can expose a container port at runtime even if the port is not specified
  with the EXPOSE instruction in the Dockerfile. The EXPOSE instruction is
  primarily for documentation purposes and does not control or enforce which
  ports are exposed at runtime.`,
		},
	}
}

func (r DockerfileRule) Spec() common.RuleSpec {
	return r.spec
}

func (r DockerfileRule) Representation() string {
	return r.spec.Representation()
}

func (r DockerfileRule) Run(team common.Team, _ string) common.RuleEvaluationResult {
	result := common.RuleEvaluationResult{
		Team:         team,
		RuleId:       r.spec.Id(),
		Completeness: 0,
		Reason:       "",
		ExecError:    nil,
	}

	content, err := os.ReadFile(fmt.Sprintf("%s/weekend-server/Dockerfile", team.GetRepoPath()))
	if err != nil {
		result.Reason = "The Dockerfile is missing"
		return result
	}

	if strings.Contains(string(content), "FROM eclipse-temurin:21") {
		result.Completeness += 0.8
	} else {
		result.Reason += "The Dockerfile does not use the correct Java version or distribution. "
	}

	if strings.Contains(string(content), "EXPOSE 8080") {
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
