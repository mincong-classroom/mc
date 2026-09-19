package team

import (
	"encoding/json"
	"io"

	"github.com/mincong-classroom/mc/common"
)

// The output of the commands with the flag --json.

type teamJSON struct {
	Name     string       `json:"name"`
	Repo     string       `json:"repo"`
	Members  []memberJSON `json:"members"`
	Status   *statusJSON  `json:"status,omitempty"`
	Problems []string     `json:"problems,omitempty"`
}

type memberJSON struct {
	Name       string `json:"name"`
	Github     string `json:"github"`
	GithubName string `json:"githubName,omitempty"` // Display name on GitHub, once validated
	State      string `json:"state,omitempty"`      // "active" or "pending", with the status
}

type statusJSON struct {
	RepoExists bool   `json:"repoExists"`
	TeamExists bool   `json:"teamExists"`
	Access     string `json:"access,omitempty"` // Role of the GitHub team on the repository
	Error      string `json:"error,omitempty"`
}

type lsJSON struct {
	Year              int        `json:"year"`
	Teams             []teamJSON `json:"teams"`
	StudentsNotInTeam []string   `json:"studentsNotInTeam"` // null without a school list
}

// newTeamJSON describes the team, with its validation and its status when they are not nil.
func newTeamJSON(team common.Team, v *Validation, s *Status) teamJSON {
	t := teamJSON{Name: team.Name, Repo: team.GetRepoName(), Members: []memberJSON{}}
	for _, member := range team.Members {
		m := memberJSON{Name: member.Name, Github: member.Github}
		if v != nil && v.Users[member.Github] != nil {
			m.GithubName = v.Users[member.Github].Name
		}
		if s != nil {
			m.State = s.States[member.Github]
		}
		t.Members = append(t.Members, m)
	}
	if v != nil {
		t.Problems = v.Problems
	}
	if s != nil {
		t.Status = &statusJSON{RepoExists: s.RepoExists, TeamExists: s.TeamExists, Access: s.RepoRole}
		if s.Err != nil {
			t.Status.Error = s.Err.Error()
		}
	}
	return t
}

// studentsNotInTeam returns the names of the students not in a team, or nil without a school list.
func studentsNotInTeam(students []common.Student, teams []common.Team) []string {
	if students == nil {
		return nil
	}
	names := []string{}
	for _, student := range unassignedStudents(students, teams) {
		names = append(names, student.Name)
	}
	return names
}

func writeJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "    ")
	return encoder.Encode(value)
}
