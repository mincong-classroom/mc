package rules

import (
	"fmt"
	"os"

	"github.com/mincong-classroom/mc/common"
)

type K8sServiceRule struct {
	Assignments map[string]common.TeamAssignmentL4
}

func (r K8sServiceRule) Spec() common.RuleSpec {
	return common.RuleSpec{
		LabId:    "L4",
		Symbol:   "SVC",
		Name:     "Service Test",
		Exercice: "3",
		Description: fmt.Sprintf(`
The team is expected to create a new Service and put the definition under the path
%s of the Git repository. Operations should be assessed
manually by the professor.`,
			petclinicDeploymentManifestPath),
	}
}

func (r K8sServiceRule) Run(team common.Team, _ string) common.RuleEvaluationResult {
	result := common.RuleEvaluationResult{
		Team:         team,
		RuleId:       r.Spec().Id(),
		Completeness: 0,
		Reason:       "",
		ExecError:    nil,
	}
	var (
		manifestPath = fmt.Sprintf("%s/%s", team.GetRepoPath(), javaServiceManifestPath)
		namespace    = team.GetKubeNamespace()
	)
	if _, err := os.ReadFile(manifestPath); err != nil {
		result.Reason = "The manifest file is missing: " + manifestPath + ", please grade manually."
		result.ExecError = err
		return result
	}

	err := kubeApply(manifestPath, namespace)
	if err != nil {
		result.Reason = "Failed to apply the manifest: " + manifestPath
		fmt.Println(result.Reason)
		fmt.Println(err)
		result.ExecError = err
		return result
	} else {
		fmt.Println("The manifest has been applied successfully")
	}

	result.Reason = "Manual grading is required"
	return result
}
