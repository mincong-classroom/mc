package rules

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Grader struct {
	assignmentsL1 map[string]TeamAssignmentL1
	mvnJarRule    MavenJarRule
}

func (g *Grader) GradeL1(team string) error {
	if assigment, ok := g.assignmentsL1[team]; ok {
		err := g.mvnJarRule.Run(team, assigment.MavenCommand)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("team %s not found in assignments", team)
	}

	return nil
}

func NewGrader() (*Grader, error) {
	path := fmt.Sprintf("%s/.mc/assignments-L1.yaml", os.Getenv("HOME"))
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	var assignmentsL1 map[string]TeamAssignmentL1
	err = yaml.Unmarshal(bytes, &assignmentsL1)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %v", err)
	}

	return &Grader{
		assignmentsL1: assignmentsL1,
		mvnJarRule:    MavenJarRule{},
	}, nil
}
