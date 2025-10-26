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
	// CustomRepoName is optional. It overrides the default repository name. This is useful for
	// teams that encountered name conflicts during the team registration on GitHub Classroom. For
	// example, one team took the name "alpha", but decided to use another name. A second team
	// tried to register with the same name "alpha" but failed because the name was taken.
	CustomRepoName *string `yaml:"repo_name"`
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

func (t Team) GetContainerRepoForWeekendServer() string {
	return fmt.Sprintf("mincongclassroom/weekend-server-%s", t.Name)
}

func (t Team) GetRepoURL() string {
	var repoName string
	if t.CustomRepoName != nil {
		repoName = *t.CustomRepoName
	} else {
		repoName = "k8s-" + t.Name
	}
	return fmt.Sprintf("git@github.com:mincong-classroom/%s.git", repoName)
}

func (t Team) GetKubeNamespace() string {
	return "team-" + t.Name
}

func (t Team) HasAllMembers(content string) bool {
	var founds []bool = make([]bool, len(t.Members))
	var lowerContent = strings.ToLower(content)

	for i, member := range t.Members {
		parts := strings.Split(member.Name, " ")
		for _, part := range parts {
			if strings.Contains(lowerContent, strings.ToLower(part)) {
				founds[i] = true
				break
			}
		}
	}

	var allFound = true
	for _, found := range founds {
		if !found {
			allFound = false
			break
		}
	}

	return allFound
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

type TeamAssignmentL2 struct{}

type TeamAssignmentL3 struct {
	NginxPodName string `yaml:"nginx_pod_name"`
}

type TeamAssignmentL4 struct{}
