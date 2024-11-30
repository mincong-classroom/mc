package rules

import (
	"fmt"
	"os"
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

func (r MavenJarRule) Run(team string, command string) error {
	if command == "" {
		return fmt.Errorf("maven command is empty")
	}

	gitPath := fmt.Sprintf("%s/github/classroom/containers-%s", os.Getenv("HOME"), team)

	script := fmt.Sprintf(`#!/bin/bash
cd "%s/weekend-server"
%s
`, gitPath, command)

	return runScript(script)
}
