package rules

import (
	"fmt"
	"os"
	"strings"

	"github.com/mincong-classroom/mc/common"
)

type DockerfileRule struct{}

func (r DockerfileRule) Id() string {
	return fmt.Sprintf("%s_%s", r.LabId(), r.Symbol())
}

func (r DockerfileRule) LabId() string {
	return "L1"
}

func (r DockerfileRule) Symbol() string {
	return "DKF"
}

func (r DockerfileRule) Name() string {
	return "Dockerfile Test"
}

func (r DockerfileRule) Exercice() string {
	return "1.2"
}

func (r DockerfileRule) Description() string {
	return `
  The team is expected to create a Dockerfile on the path
  "weekend-server/Dockerfile". The Java version should be 21, from the
  distribution "eclipse-temurin". The port 8080 should be exposed. Note that
  you can expose a container port at runtime even if the port is not specified
  with the EXPOSE instruction in the Dockerfile. The EXPOSE instruction is
  primarily for documentation purposes and does not control or enforce which
  ports are exposed at runtime.`
}

func (r DockerfileRule) Representation() string {
	ruleId := r.LabId() + "_" + r.Symbol()

	// e.g. L1_JAR: JAR Creation Test (Ex 1.1)
	title := fmt.Sprintf("%s: %s (Ex %s)\n  ", ruleId, r.Name(), r.Exercice())
	body := r.Description()
	return title + body
}

func (r DockerfileRule) Run(team common.Team, _ string) common.RuleEvaluationResult {
	result := common.RuleEvaluationResult{
		Team:         team,
		RuleId:       r.Id(),
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
