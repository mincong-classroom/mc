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
	k8sControlPlaneRule common.Rule[string]
	k8sRunNginxPodRule  common.Rule[string]
	k8sNginxPodRule     common.Rule[string]

	// L3
	k8sJavaPodRule common.Rule[string]

	// L4
	k8sNginxReplicaSetRule   common.Rule[string]
	k8sJavaDeploymentSetRule common.Rule[string]
	K8sServiceRule           common.Rule[string]
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

		assignmentsL2:       assignmentsL2,
		k8sControlPlaneRule: ManualRule{ruleSpec: k8sControlPlaneRuleSet},
		k8sRunNginxPodRule:  ManualRule{ruleSpec: k8sRunNginxPodRuleSet},
		k8sNginxPodRule:     K8sNginxPodRule{Assignments: assignmentsL3},

		assignmentsL3:  assignmentsL3,
		k8sJavaPodRule: K8sJavaPodRule{Assignments: assignmentsL3},

		assignmentsL4:            assignmentsL4,
		k8sNginxReplicaSetRule:   K8sNginxReplicaSetRule{Assignments: assignmentsL4},
		k8sJavaDeploymentSetRule: K8sJavaDeploymentRule{Assignments: assignmentsL4},
		K8sServiceRule:           K8sServiceRule{Assignments: assignmentsL4},
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

		// L3
		g.k8sJavaPodRule.Spec().Representation(),

		// L4
		g.k8sNginxReplicaSetRule.Spec().Representation(),
		g.k8sJavaDeploymentSetRule.Spec().Representation(),
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
		k8sNginxPodResult := g.k8sNginxPodRule.Run(team, "")
		results = append(results, k8sNginxPodResult)

		k8sJavaPodResult := g.k8sJavaPodRule.Run(team, "")
		results = append(results, k8sJavaPodResult)
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
		k8sNginxReplicaSetResult := g.k8sNginxReplicaSetRule.Run(team, "")
		results = append(results, k8sNginxReplicaSetResult)

		k8sJavaDeploymentSetResult := g.k8sJavaDeploymentSetRule.Run(team, "")
		results = append(results, k8sJavaDeploymentSetResult)

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
