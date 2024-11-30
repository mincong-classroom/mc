package common

import (
	"fmt"
	"strings"
)

type TeamRegistry struct {
	Teams []Team
}

type Team struct {
	Name    string
	Members []TeamMember
}

type TeamMember struct {
	Name   string // Full name in format "LAST, First", e.g. "SMITH, John"
	Github string // Github username
}

func (t Team) GetMembersAsString() string {
	var values []string
	for _, member := range t.Members {
		values = append(values, fmt.Sprintf("%s (@%s)", member.Name, member.Github))
	}
	return strings.Join(values, ", ")
}

// Rule represents a rule to grade the assignment.
//
// The type T is the type of the options to be used when executing the rule.
type Rule[T any] interface {
	// Id The ID of the rule. The combination of Symbol and LabId, such as "L1_JAR", "L1_MST", etc.
	Id() string

	// Symbol The symbol of the rule, such as "JAR", "MST", etc.
	Symbol() string

	// LabId The ID of the lab session of this rule, such as "L1", "L2", "L3", etc.
	LabId() string

	// Exercice The exercice of the rule, such as "1.1", "1.2", "2.1", etc.
	Exercice() string

	// Name The name of the rule.
	Name() string

	// Run Run the rule for the given team.
	Run(team Team, opts T) RuleEvaluationResult

	// Description The description of the rule.
	Description() string

	// Representation The representation of the rule.
	Representation() string
}

type RuleEvaluationResult struct {
	Team         Team    // The team evaluated
	RuleId       string  // The ID of the rule
	Completeness float32 // Between 0 and 1 (100%)
	Reason       string  // Reason of the evaluation
	ExecError    error   // Execution error if the rule fails
}

type TeamAssignmentL1 struct {
	MavenCommand string `yaml:"mvn_command"`
}
