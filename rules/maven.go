package rules

import (
	"fmt"

	"github.com/mincong-classroom/mc/common"
)

// MavenJarRule checks whether the team can create a JAR file using maven.
//
// To find the Maven commands, use the following bash command:
//
//	rg -g 'k8s*/docs/lab-1.md' mvn -C 3
//
// Then, extract the command manually and put it into the file "assignments-L1.yaml".
type MavenJarRule struct{}

func (r MavenJarRule) Spec() common.RuleSpec {
	return common.RuleSpec{
		LabId:    "L1",
		Symbol:   "JAR",
		Name:     "JAR Creation Test",
		Exercice: "1",
		Description: `
The team is expected to create a JAR manually using a maven command and the
server should start locally under the port 8080. The team is also expected to
extract the JAR file to inspect the content of the MANIFEST.MF file.`,
	}
}

func (r MavenJarRule) Run(team common.Team, command string) common.RuleEvaluationResult {
	if command == "" {
		return common.RuleEvaluationResult{
			Team:         team,
			RuleId:       r.Spec().Id(),
			Completeness: 0,
			Reason:       "The maven command is empty",
			ExecError:    nil,
		}
	}

	gitPath := team.GetRepoPath()

	script := fmt.Sprintf(`#!/bin/bash
cd "%s/weekend-server"
%s
`, gitPath, command)

	err := runScript(script)
	if err != nil {
		return common.RuleEvaluationResult{
			Team:         team,
			RuleId:       r.Spec().Id(),
			Completeness: 0,
			Reason:       "The maven command failed",
			ExecError:    err,
		}
	} else {
		return common.RuleEvaluationResult{
			Team:         team,
			RuleId:       r.Spec().Id(),
			Completeness: 1,
			Reason:       "The maven command succeeded",
			ExecError:    nil,
		}
	}
}
