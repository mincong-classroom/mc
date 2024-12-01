package rules

import (
	"fmt"
	"os"
	"strings"

	"github.com/mincong-classroom/mc/common"
)

type MavenSetupRule struct{}

func (r MavenSetupRule) Spec() common.RuleSpec {
	return common.RuleSpec{
		LabId:    "L2",
		Symbol:   "MST",
		Name:     "Maven Setup Test",
		Exercice: "1",
		Description: `
The team is expected to run unit tests with Maven in GitHub Actions on the path
".github/workflows/app.yml". It should contain the keyword "mvn"`,
	}
}

func (r MavenSetupRule) Run(team common.Team, _ string) common.RuleEvaluationResult {
	result := common.RuleEvaluationResult{
		Team:         team,
		RuleId:       r.Spec().Id(),
		Completeness: 0,
		Reason:       "",
		ExecError:    nil,
	}

	bytes, err := os.ReadFile(fmt.Sprintf("%s/.github/workflows/app.yml", team.GetRepoPath()))
	if err != nil {
		result.Reason = "The workflow file \"app.yaml\" is missing"
		result.ExecError = err
		return result
	}

	content := string(bytes)
	if strings.Contains(content, "mvn") {
		result.Completeness = 1
		result.Reason = "The workflow file contains the keyword \"mvn\""
	} else {
		result.Reason = "The workflow file does not contain the keyword \"mvn\""
	}

	return result
}

type DockerSetupRule struct{}

func (r DockerSetupRule) Spec() common.RuleSpec {
	return common.RuleSpec{
		LabId:    "L2",
		Symbol:   "DST",
		Name:     "Docker Setup Test",
		Exercice: "3",
		Description: `
The team is expected to build a Docker image and publish it to the Docker
registry. The docker login should be done by retrieving the username and
password from the secrets, such as "secrets.DOCKER_USERNAME". This is probably
done using the GitHub Action "docker/login-action" but other approaches are
fine too.`,
	}
}

func (r DockerSetupRule) Run(team common.Team, _ string) common.RuleEvaluationResult {
	result := common.RuleEvaluationResult{
		Team:         team,
		RuleId:       r.Spec().Id(),
		Completeness: 0,
		Reason:       "",
		ExecError:    nil,
	}

	bytes, err := os.ReadFile(fmt.Sprintf("%s/.github/workflows/app.yml", team.GetRepoPath()))
	if err != nil {
		result.Reason = "The workflow file \"app.yaml\" is missing"
		result.ExecError = err
		return result
	}

	content := string(bytes)
	if strings.Contains(content, "docker/login-action") || strings.Contains(content, "docker login") {
		result.Completeness += 0.6
	} else {
		result.Reason += "Missing action \"docker/login-action\" or docker-login, "
	}

	if strings.Contains(content, "secrets.DOCKER_USERNAME") {
		result.Completeness += 0.2
	} else {
		result.Reason += "Missing secret \"DOCKER_USERNAME\", "
	}
	if strings.Contains(content, "secrets.DOCKER_PASSWORD") {
		result.Completeness += 0.2
	} else {
		result.Reason += "Missing secret \"DOCKER_PASSWORD\", "
	}

	if result.Completeness == 1 {
		result.Reason = "The workflow file is correct"
	} else {
		result.Reason = strings.TrimSpace(result.Reason)
	}

	return result
}
