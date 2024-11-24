package cmd

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
