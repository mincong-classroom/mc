package common

import (
	"fmt"
	"os"
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

func (t Team) GetRepoPath() string {
	return fmt.Sprintf("%s/github/classroom/containers-%s", os.Getenv("HOME"), t.Name)
}

// Rule represents a rule to grade the assignment.
//
// The type T is the type of the options to be used when executing the rule.
type Rule[T any] interface {
	// Spec The specification of the rule.
	Spec() RuleSpec

	// Run Run the rule for the given team.
	Run(team Team, opts T) RuleEvaluationResult
}

type RuleSpec struct {
	Symbol      string // The symbol of the rule, such as "JAR", "MST", etc.
	LabId       string // The ID of the lab session of this rule, such as "L1", "L2", "L3", etc.
	Exercice    string // The exercice of the rule, such as "1.1", "1.2", "2.1", etc.
	Name        string // The name of the rule.
	Description string // The description of the rule.
}

func (r RuleSpec) Id() string {
	return fmt.Sprintf("%s_%s", r.LabId, r.Symbol)
}

func (r RuleSpec) Representation() string {
	// e.g. L1_JAR: JAR Creation Test (Ex 1.1)
	title := fmt.Sprintf("%s: %s (Ex %s)\n  ", r.Id(), r.Name, r.Exercice)
	body := strings.ReplaceAll(r.Description, "\n", "\n    ")

	return title + body + "\n"
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
