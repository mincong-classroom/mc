package rules

import (
	"fmt"
	"os/exec"

	"github.com/mincong-classroom/mc/common"
)

type K8sNginxPodRule struct{}

func (r K8sNginxPodRule) Spec() common.RuleSpec {
	return common.RuleSpec{
		LabId:    "L3",
		Symbol:   "NGY",
		Name:     "Nginx Yaml Test",
		Exercice: "3",
		Description: `
The team is expected to create a new pod running with Nginx using a kubectl-apply
command. This pod should be reachable using the port 80. The manifest should
be saved under the path k8s/pod-nginx.yaml of the Git repository.`,
	}
}

func (r K8sNginxPodRule) Run(team common.Team, _ string) common.RuleEvaluationResult {
	result := common.RuleEvaluationResult{
		Team:         team,
		RuleId:       r.Spec().Id(),
		Completeness: 0,
		Reason:       "",
		ExecError:    nil,
	}
	var (
		manifestPath = fmt.Sprintf("%s/k8s/pod-nginx.yaml", team.GetRepoPath())
		namespace    = team.Name
	)
	applyCmd := exec.Command("kubectl", "apply", "-f", manifestPath, "-n", namespace, "--dry-run=client")
	if err := applyCmd.Run(); err != nil {
		result.Reason = "Failed to apply the manifest: " + manifestPath
		result.ExecError = err
	} else {
		result.Completeness = 1
		result.Reason = "The manifest has been successfully applied"
	}
	return result
}
