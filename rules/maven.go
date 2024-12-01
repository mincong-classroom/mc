package rules

import (
	"fmt"
	"os"

	"github.com/mincong-classroom/mc/common"
)

type MavenJarRule struct{}

func (r MavenJarRule) Spec() common.RuleSpec {
	return common.RuleSpec{
		LabId:    "L1",
		Symbol:   "JAR",
		Name:     "JAR Creation Test",
		Exercice: "1.1",
		Description: `
The team is expected to create a JAR manually using a maven command and the
server should start locally under the port 8080.`,
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

	gitPath := fmt.Sprintf("%s/github/classroom/containers-%s", os.Getenv("HOME"), team.Name)

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
