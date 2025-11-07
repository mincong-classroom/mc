package rules

import (
	"fmt"
	"os"

	"github.com/mincong-classroom/mc/common"
)

type K8sReplicaSetRule struct {
	Assignments map[string]common.TeamAssignmentL4
}

func (r K8sReplicaSetRule) Spec() common.RuleSpec {
	return common.RuleSpec{
		LabId:    "L3",
		Symbol:   "RST",
		Name:     "ReplicaSet Test",
		Exercice: "1",
		Description: fmt.Sprintf(`
The team is expected to create a new ReplicaSet and put the definition under the path
%q of the Git repository. Operations should be assessed
manually by the teacher. The container should use port 8080 to receive incoming
traffic. The container name should be "main". The docker image should be the
one published by the team in the previous lab, i.e.
"mincongclassroom/spring-petclinic-{team}". The team should use 2 labels:
app=spring-petclinic and team=<team-name>. The ReplicaSet should be created
successfully and the Pods should be running. Then, the team should describe how
they scale the ReplicaSet and what happens if they delete a Pod managed by the
ReplicaSet.`,
			petclinicReplicaSetManifestPath),
	}
}

func (r K8sReplicaSetRule) Run(team common.Team, _ string) common.RuleEvaluationResult {
	result := common.RuleEvaluationResult{
		Team:         team,
		RuleId:       r.Spec().Id(),
		Completeness: 0,
		Reason:       "",
		ExecError:    nil,
	}
	var (
		manifestPath = fmt.Sprintf("%s/%s", team.GetRepoPath(), petclinicReplicaSetManifestPath)
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
