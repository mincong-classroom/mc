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

	// L1
	mavenJarRule    common.Rule[string]
	dockerfileRule  common.Rule[string]
	dockerImageRule common.Rule[string]
	sqlInitRule     common.Rule[string]

	// L2
	mavenSetupRule common.Rule[string]
	registryRule   common.Rule[string]
}

func NewGrader() (*Grader, error) {
	var (
		assignmentsL1 map[string]common.TeamAssignmentL1
		assignmentsL2 map[string]common.TeamAssignmentL2
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

	return &Grader{
		assignmentsL1:   assignmentsL1,
		mavenJarRule:    MavenJarRule{},
		dockerfileRule:  DockerfileRule{},
		dockerImageRule: DockerImageRule{},
		sqlInitRule:     SqlInitRule{},

		assignmentsL2:  assignmentsL2,
		mavenSetupRule: MavenSetupRule{},
		registryRule:   RegistryRule{},
	}, nil
}

func (g *Grader) ListRuleRepresentations() []string {
	return []string{
		// L1
		g.mavenJarRule.Spec().Representation(),
		g.dockerfileRule.Spec().Representation(),
		g.dockerImageRule.Spec().Representation(),
		g.sqlInitRule.Spec().Representation(),

		// L2
		g.mavenSetupRule.Spec().Representation(),
		g.registryRule.Spec().Representation(),
	}
}

func (g *Grader) GradeL1(team common.Team) []common.RuleEvaluationResult {
	fmt.Printf("\n=== L1: Grading Team %s ===\n", team.Name)
	results := make([]common.RuleEvaluationResult, 0)

	if assignment, ok := g.assignmentsL1[team.Name]; ok {
		mavenResult := g.mavenJarRule.Run(team, assignment.MavenCommand)
		results = append(results, mavenResult)

		dockerfileResult := g.dockerfileRule.Run(team, "")
		results = append(results, dockerfileResult)

		dockerImageResult := g.dockerImageRule.Run(team, "")
		results = append(results, dockerImageResult)

		sqlResult := g.sqlInitRule.Run(team, "")
		results = append(results, sqlResult)
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
		mavenSetupResult := g.mavenSetupRule.Run(team, "")
		results = append(results, mavenSetupResult)

		registryResult := g.registryRule.Run(team, "")
		results = append(results, registryResult)
	} else {
		fmt.Printf("team %s not found in assignments", team.Name)
	}

	fmt.Println("Grading done")
	return results
}
