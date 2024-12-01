package rules

import (
	"fmt"
	"log"
	"os"

	"github.com/mincong-classroom/mc/common"
	"gopkg.in/yaml.v3"
)

type Grader struct {
	assignmentsL1  map[string]common.TeamAssignmentL1
	mvnJarRule     common.Rule[string]
	dockerfileRule common.Rule[string]
}

func NewGrader() (*Grader, error) {
	path := fmt.Sprintf("%s/.mc/assignments-L1.yaml", os.Getenv("HOME"))
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	var assignmentsL1 map[string]common.TeamAssignmentL1
	err = yaml.Unmarshal(bytes, &assignmentsL1)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %v", err)
	}

	return &Grader{
		assignmentsL1:  assignmentsL1,
		mvnJarRule:     NewMavenJarRule(),
		dockerfileRule: NewDockerfileRule(),
	}, nil
}

func (g *Grader) ListRuleRepresentations() []string {
	return []string{
		g.mvnJarRule.Representation(),
		g.dockerfileRule.Representation(),
	}
}

func (g *Grader) GradeL1(team common.Team) []common.RuleEvaluationResult {
	fmt.Printf("\n=== Grading Team %s ===\n", team.Name)
	results := make([]common.RuleEvaluationResult, 0)

	if assigment, ok := g.assignmentsL1[team.Name]; ok {
		mavenResult := g.mvnJarRule.Run(team, assigment.MavenCommand)
		results = append(results, mavenResult)

		dockerfileResult := g.dockerfileRule.Run(team, "")
		results = append(results, dockerfileResult)
	} else {
		log.Print(fmt.Printf("team %s not found in assignments", team.Name))
	}

	fmt.Print("Grading done")
	return results
}
