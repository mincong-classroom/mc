package rules

import (
	"fmt"
	"os"

	"github.com/mincong-classroom/mc/common"
	"gopkg.in/yaml.v3"
)

type Grader struct {
	assignmentsL1 map[string]common.TeamAssignmentL1
	assignmentsL2 map[string]common.TeamAssignmentL2
	assignmentsL3 map[string]common.TeamAssignmentL3
	assignmentsL4 map[string]common.TeamAssignmentL4

	// L1
	mavenJarRule      common.Rule[string]
	dockerfileRule    common.Rule[string]
	dockerImageRule   common.Rule[string]
	dockerProcessRule common.Rule[string]
	dockerTeamRule    common.Rule[string]

	// L2
	k8sControlPlaneRule   common.Rule[string]
	k8sRunNginxPodRule    common.Rule[string]
	k8sNginxPodRule       common.Rule[string]
	k8sJavaPodRule        common.Rule[string]
	k8sOperateJavaPodRule common.Rule[string]
	k8sFixBrokenPodRule   common.Rule[string]

	// L3
	k8sNginxReplicaSetRule      common.Rule[string]
	k8sJavaDeploymentSetRule    common.Rule[string]
	dockerFrontendImageRule     common.Rule[string]
	dockerCustomerImageRule     common.Rule[string]
	dockerVeterinarianImageRule common.Rule[string]

	// L4
	K8sServiceRule common.Rule[string]
}

func NewGrader() (*Grader, error) {
	var (
		assignmentsL1 map[string]common.TeamAssignmentL1
		assignmentsL2 map[string]common.TeamAssignmentL2
		assignmentsL3 map[string]common.TeamAssignmentL3
		assignmentsL4 map[string]common.TeamAssignmentL4
	)

	path1 := fmt.Sprintf("%s/.mc/assignments-L1.yaml", os.Getenv("HOME"))
	bytes1, err := os.ReadFile(path1)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	err = yaml.Unmarshal(bytes1, &assignmentsL1)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %v", err)
	}

	path2 := fmt.Sprintf("%s/.mc/assignments-L2.yaml", os.Getenv("HOME"))
	bytes2, err := os.ReadFile(path2)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	err = yaml.Unmarshal(bytes2, &assignmentsL2)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %v", err)
	}

	path3 := fmt.Sprintf("%s/.mc/assignments-L3.yaml", os.Getenv("HOME"))
	bytes3, err := os.ReadFile(path3)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	err = yaml.Unmarshal(bytes3, &assignmentsL3)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %v", err)
	}

	path4 := fmt.Sprintf("%s/.mc/assignments-L4.yaml", os.Getenv("HOME"))
	bytes4, err := os.ReadFile(path4)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	err = yaml.Unmarshal(bytes4, &assignmentsL4)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %v", err)
	}

	return &Grader{
		assignmentsL1:     assignmentsL1,
		mavenJarRule:      MavenJarRule{},
		dockerfileRule:    DockerfileRule{},
		dockerImageRule:   DockerImageRule{},
		dockerProcessRule: DockerProcessRule{},
		dockerTeamRule:    DockerTeamRule{},

		assignmentsL2:         assignmentsL2,
		k8sControlPlaneRule:   ManualRule{ruleSpec: k8sControlPlaneRuleSet},
		k8sRunNginxPodRule:    ManualRule{ruleSpec: k8sRunNginxPodRuleSet},
		k8sNginxPodRule:       K8sNginxPodRule{Assignments: assignmentsL3},
		k8sJavaPodRule:        K8sJavaPodRule{Assignments: assignmentsL3},
		k8sOperateJavaPodRule: ManualRule{ruleSpec: k8sOperateJavaPodRuleSpec},
		k8sFixBrokenPodRule:   ManualRule{ruleSpec: k8sFixBrokenPodRuleSpec},

		assignmentsL3:               assignmentsL3,
		k8sNginxReplicaSetRule:      K8sReplicaSetRule{Assignments: assignmentsL4},
		k8sJavaDeploymentSetRule:    K8sDeploymentRule{Assignments: assignmentsL4},
		dockerFrontendImageRule:     ManualRule{ruleSpec: dockerFrontendImageRuleSpec},
		dockerCustomerImageRule:     ManualRule{ruleSpec: dockerCustomerImageRuleSpec},
		dockerVeterinarianImageRule: ManualRule{ruleSpec: dockerVeterinarianImageRuleSpec},

		assignmentsL4:  assignmentsL4,
		K8sServiceRule: K8sServiceRule{Assignments: assignmentsL4},
	}, nil
}

func (g *Grader) ListRuleRepresentations() []string {
	return []string{
		// L1
		g.mavenJarRule.Spec().Representation(),
		g.dockerfileRule.Spec().Representation(),
		g.dockerImageRule.Spec().Representation(),
		g.dockerProcessRule.Spec().Representation(),
		g.dockerTeamRule.Spec().Representation(),

		// L2
		g.k8sControlPlaneRule.Spec().Representation(),
		g.k8sRunNginxPodRule.Spec().Representation(),
		g.k8sNginxPodRule.Spec().Representation(),
		g.k8sJavaPodRule.Spec().Representation(),
		g.k8sOperateJavaPodRule.Spec().Representation(),
		g.k8sFixBrokenPodRule.Spec().Representation(),

		// L3
		g.k8sNginxReplicaSetRule.Spec().Representation(),
		g.k8sJavaDeploymentSetRule.Spec().Representation(),
		g.k8sDockerFrontendImageRule.Spec().Representation(),
		g.k8sDockerCustomerImageRule.Spec().Representation(),
		g.k8sDockerVeterinarianImageRule.Spec().Representation(),

		// L4
		g.K8sServiceRule.Spec().Representation(),
	}
}

func (g *Grader) GradeL1(team common.Team) []common.RuleEvaluationResult {
	fmt.Printf("\n=== L1: Grading Team %s ===\n", team.Name)
	results := make([]common.RuleEvaluationResult, 0)

	if _, ok := g.assignmentsL1[team.Name]; ok {
		// mavenResult := g.mavenJarRule.Run(team, assignment.MavenCommand)
		// results = append(results, mavenResult)

		dockerfileResult := g.dockerfileRule.Run(team, "")
		results = append(results, dockerfileResult)

		dockerImageResult := g.dockerImageRule.Run(team, "")
		results = append(results, dockerImageResult)

		dockerProcessResult := g.dockerProcessRule.Run(team, "")
		results = append(results, dockerProcessResult)

		dockerTeamResult := g.dockerTeamRule.Run(team, "")
		results = append(results, dockerTeamResult)
	} else {
		fmt.Printf("team %s not found in assignments", team.Name)
	}

	fmt.Println("Grading done")
	return results
}

func (g *Grader) GradeL2(team common.Team) []common.RuleEvaluationResult {
	fmt.Printf("\n=== L2: Grading Team %s ===\n", team.Name)
	results := make([]common.RuleEvaluationResult, 0)

	if _, ok := g.assignmentsL1[team.Name]; ok {
		k8sControlPlaneRuleResults := g.k8sControlPlaneRule.Run(team, "")
		results = append(results, k8sControlPlaneRuleResults)

		k8sRunNginxPodRuleResults := g.k8sRunNginxPodRule.Run(team, "")
		results = append(results, k8sRunNginxPodRuleResults)

		k8sNginxPodRuleResults := g.k8sNginxPodRule.Run(team, "")
		results = append(results, k8sNginxPodRuleResults)

		k8sJavaPodResults := g.k8sJavaPodRule.Run(team, "")
		results = append(results, k8sJavaPodResults)

		k8sOperateJavaPodRuleResults := g.k8sOperateJavaPodRule.Run(team, "")
		results = append(results, k8sOperateJavaPodRuleResults)

		k8sFixBrokenPodRuleResults := g.k8sFixBrokenPodRule.Run(team, "")
		results = append(results, k8sFixBrokenPodRuleResults)
	} else {
		fmt.Printf("team %s not found in assignments", team.Name)
	}

	fmt.Println("Grading done")
	return results
}

func (g *Grader) GradeL3(team common.Team) []common.RuleEvaluationResult {
	fmt.Printf("\n=== L3: Grading Team %s ===\n", team.Name)
	results := make([]common.RuleEvaluationResult, 0)

	if _, ok := g.assignmentsL1[team.Name]; ok {
		k8sReplicaSetResults := g.k8sNginxReplicaSetRule.Run(team, "")
		results = append(results, k8sReplicaSetResults)

		k8sJavaDeploymentSetResults := g.k8sJavaDeploymentSetRule.Run(team, "")
		results = append(results, k8sJavaDeploymentSetResults)

		if team.Role == "frontend" {
			dockerFrontendImageResults := g.dockerFrontendImageRule.Run(team, "")
			results = append(results, dockerFrontendImageResults)
		}
		if team.Role == "customer" {
			dockerCustomerImageResults := g.dockerCustomerImageRule.Run(team, "")
			results = append(results, dockerCustomerImageResults)
		}
		if team.Role == "veterinarian" {
			dockerVeterinarianImageResults := g.dockerVeterinarianImageRule.Run(team, "")
			results = append(results, dockerVeterinarianImageResults)
		}
	} else {
		fmt.Printf("team %s not found in assignments", team.Name)
	}

	fmt.Println("Grading done")
	return results
}

func (g *Grader) GradeL4(team common.Team) []common.RuleEvaluationResult {
	fmt.Printf("\n=== L4: Grading Team %s ===\n", team.Name)
	results := make([]common.RuleEvaluationResult, 0)

	if _, ok := g.assignmentsL4[team.Name]; ok {
		k8sServiceResult := g.K8sServiceRule.Run(team, "")
		results = append(results, k8sServiceResult)
	} else {
		fmt.Printf("team %s not found in assignments", team.Name)
	}

	fmt.Println("Grading done")
	return results
}

func (g *Grader) GradeL5(team common.Team) []common.RuleEvaluationResult {
	fmt.Printf("\n=== L5: Grading Team %s ===\n", team.Name)
	results := make([]common.RuleEvaluationResult, 0)
	fmt.Println("Grading done")
	return results
}
