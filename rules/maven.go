package rules

import (
	"fmt"
	"os"

	"github.com/mincong-classroom/mc/common"
)

type MavenJarRule struct{}

func (r MavenJarRule) Id() string {
	return fmt.Sprintf("%s_%s", r.LabId(), r.Symbol())
}

func (r MavenJarRule) LabId() string {
	return "L1"
}

func (r MavenJarRule) Symbol() string {
	return "JAR"
}

func (r MavenJarRule) Name() string {
	return "JAR Creation Test"
}

func (r MavenJarRule) Exercice() string {
	return "1.1"
}

func (r MavenJarRule) Description() string {
	return `The team is expected to create a JAR manually using a maven command and the server
	should start locally under the port 8080.`
}

func (r MavenJarRule) Run(team common.Team, command string) common.RuleEvaluationResult {
	if command == "" {
		return common.RuleEvaluationResult{
			Team:         team,
			RuleId:       r.Id(),
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
			RuleId:       r.Id(),
			Completeness: 0,
			Reason:       "The maven command failed",
			ExecError:    err,
		}
	} else {
		return common.RuleEvaluationResult{
			Team:         team,
			RuleId:       r.Id(),
			Completeness: 1,
			Reason:       "The maven command succeeded",
			ExecError:    nil,
		}
	}
}
