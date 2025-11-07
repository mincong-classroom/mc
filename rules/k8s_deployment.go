package rules

import (
	"fmt"
	"os"

	"github.com/mincong-classroom/mc/common"
)

type K8sDeploymentRule struct {
	Assignments map[string]common.TeamAssignmentL4
}

func (r K8sDeploymentRule) Spec() common.RuleSpec {
	return common.RuleSpec{
		LabId:    "L3",
		Symbol:   "DPL",
		Name:     "Deployment Test",
		Exercice: "2",
		Description: fmt.Sprintf(`
The team is expected to create a new Deployment and put the definition under the path
%q of the Git repository. Operations should be assessed
manually by the teacher. Most of the requirements are similar to the ReplicaSet.
That is, the container should use port 8080 to receive incoming
traffic; the container name should be "main"; the team should use 2 labels:
petclinicDeploymentManifestPath),
app=spring-petclinic and team=<team-name>. Then, they are expected to create a
environment variable "TEAM" with the value in lowercase and observe the rollout
history. Finally, they should disrupt the Deployment and observe what happens.`, petclinicDeploymentManifestPath),
	}
}

func (r K8sDeploymentRule) Run(team common.Team, _ string) common.RuleEvaluationResult {
	result := common.RuleEvaluationResult{
		Team:         team,
		RuleId:       r.Spec().Id(),
		Completeness: 0,
		Reason:       "",
		ExecError:    nil,
	}
	var (
		manifestPath = fmt.Sprintf("%s/%s", team.GetRepoPath(), petclinicDeploymentManifestPath)
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
