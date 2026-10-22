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
	assignmentsL5 map[string]common.TeamAssignmentL5

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
	k8sNginxReplicaSetRule   common.Rule[string]
	k8sJavaDeploymentSetRule common.Rule[string]

	// L4
	k8sNamespaceCreateRule   common.Rule[string]
	k8sTeamInfoServerRule    common.Rule[string]
	apiGatewayAboutRouteRule common.Rule[string]

	// L5
	k8sSecretRule             common.Rule[string]
	petclinicGenaiServiceRule common.Rule[string]
}

func NewGrader() (*Grader, error) {
	var (
		assignmentsL1 map[string]common.TeamAssignmentL1
		assignmentsL2 map[string]common.TeamAssignmentL2
		assignmentsL3 map[string]common.TeamAssignmentL3
		assignmentsL4 map[string]common.TeamAssignmentL4
		assignmentsL5 map[string]common.TeamAssignmentL5
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
		mavenJarRule:      ManualRule{ruleSpec: mavenJarRuleSpec},
		dockerfileRule:    ManualRule{ruleSpec: dockerfileRuleSpec},
		dockerImageRule:   ManualRule{ruleSpec: dockerImageRuleSpec},
		dockerProcessRule: ManualRule{ruleSpec: dockerProcessRuleSpec},
		dockerTeamRule:    ManualRule{ruleSpec: dockerTeamRuleSpec},

		assignmentsL2:         assignmentsL2,
		k8sControlPlaneRule:   ManualRule{ruleSpec: k8sControlPlaneRuleSet},
		k8sRunNginxPodRule:    ManualRule{ruleSpec: k8sRunNginxPodRuleSet},
		k8sNginxPodRule:       K8sNginxPodRule{Assignments: assignmentsL3},
		k8sJavaPodRule:        K8sJavaPodRule{Assignments: assignmentsL3},
		k8sOperateJavaPodRule: ManualRule{ruleSpec: k8sOperateJavaPodRuleSpec},
		k8sFixBrokenPodRule:   ManualRule{ruleSpec: k8sFixBrokenPodRuleSpec},

		assignmentsL3:            assignmentsL3,
		k8sNginxReplicaSetRule:   K8sReplicaSetRule{Assignments: assignmentsL4},
		k8sJavaDeploymentSetRule: K8sDeploymentRule{Assignments: assignmentsL4},

		assignmentsL4:            assignmentsL4,
		k8sNamespaceCreateRule:   ManualRule{ruleSpec: k8sNamespaceCreateRuleSpec},
		k8sTeamInfoServerRule:    K8sTeamInfoServerRule{},
		apiGatewayAboutRouteRule: ManualRule{ruleSpec: apiGatewayAboutRouteRuleSpec},

		assignmentsL5:             assignmentsL5,
		k8sSecretRule:             ManualRule{ruleSpec: k8sSecretRuleSpec},
		petclinicGenaiServiceRule: ManualRule{ruleSpec: petclinicGenaiServiceRuleSpec},
	}, nil
}

// ListRuleSpecs returns the specification of every rule, ordered by lab and exercise.
func (g *Grader) ListRuleSpecs() []common.RuleSpec {
	return []common.RuleSpec{
		// L1
		g.mavenJarRule.Spec(),
		g.dockerfileRule.Spec(),
		g.dockerImageRule.Spec(),
		g.dockerProcessRule.Spec(),
		g.dockerTeamRule.Spec(),

		// L2
		g.k8sControlPlaneRule.Spec(),
		g.k8sRunNginxPodRule.Spec(),
		g.k8sNginxPodRule.Spec(),
		g.k8sJavaPodRule.Spec(),
		g.k8sOperateJavaPodRule.Spec(),
		g.k8sFixBrokenPodRule.Spec(),

		// L3
		g.k8sNginxReplicaSetRule.Spec(),
		g.k8sJavaDeploymentSetRule.Spec(),

		// L4
		g.k8sNamespaceCreateRule.Spec(),
		g.k8sTeamInfoServerRule.Spec(),
		g.apiGatewayAboutRouteRule.Spec(),

		// L5
		g.k8sSecretRule.Spec(),
		g.petclinicGenaiServiceRule.Spec(),
	}
}

func (g *Grader) GradeL1(team common.Team) []common.RuleEvaluationResult {
	fmt.Printf("\n=== L1: Grading Team %s ===\n", team.Name)
	results := make([]common.RuleEvaluationResult, 0)

	// Every team of the registry: the rules of Lab 1 are graded by reading the work of the team, and
	// need no data of an assignments file.
	results = append(results, g.mavenJarRule.Run(team, ""))
	results = append(results, g.dockerfileRule.Run(team, ""))
	results = append(results, g.dockerImageRule.Run(team, ""))
	results = append(results, g.dockerProcessRule.Run(team, ""))
	results = append(results, g.dockerTeamRule.Run(team, ""))

	fmt.Println("Grading done")
	return results
}

func (g *Grader) GradeL2(team common.Team) []common.RuleEvaluationResult {
	fmt.Printf("\n=== L2: Grading Team %s ===\n", team.Name)
	results := make([]common.RuleEvaluationResult, 0)

	// Every team of the registry: no rule of Lab 2 needs the data of an assignments file. The
	// custom name of the nginx Pod, from assignments-L3.yaml, is only a fallback of L2_NGY.
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
		k8sNamespaceCreateResults := g.k8sNamespaceCreateRule.Run(team, "")
		results = append(results, k8sNamespaceCreateResults)

		k8sTeamInfoServerResults := g.k8sTeamInfoServerRule.Run(team, "")
		results = append(results, k8sTeamInfoServerResults)

		apiGatewayAboutRouteResults := g.apiGatewayAboutRouteRule.Run(team, "")
		results = append(results, apiGatewayAboutRouteResults)
	} else {
		fmt.Printf("team %s not found in assignments", team.Name)
	}

	fmt.Println("Grading done")
	return results
}

func (g *Grader) GradeL5(team common.Team) []common.RuleEvaluationResult {
	fmt.Printf("\n=== L5: Grading Team %s ===\n", team.Name)
	results := make([]common.RuleEvaluationResult, 0)

	if _, ok := g.assignmentsL5[team.Name]; ok {
		k8sSecretResults := g.k8sSecretRule.Run(team, "")
		results = append(results, k8sSecretResults)

		petclinicGenaiServiceResults := g.petclinicGenaiServiceRule.Run(team, "")
		results = append(results, petclinicGenaiServiceResults)
	} else {
		fmt.Printf("team %s not found in assignments", team.Name)
	}

	fmt.Println("Grading done")
	return results
}
