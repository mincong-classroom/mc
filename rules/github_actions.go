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
