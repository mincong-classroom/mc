package rules

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/mincong-classroom/mc/common"
)

type SqlInitRule struct{}

func (r SqlInitRule) Spec() common.RuleSpec {
	return common.RuleSpec{
		LabId:    "L1",
		Symbol:   "SQL",
		Name:     "SQL Init Test",
		Exercice: "2.1.2",
		Description: `
The team is expected to complete the SQL script located at the path
"weekend-mysql/init.sql". The script should contain an "INSERT INTO" statement
followed by 7 values, either using VARCHAR or INT as key for the table
"mappings" or a similar table.`,
	}
}

func (r SqlInitRule) Run(team common.Team, _ string) common.RuleEvaluationResult {
	result := common.RuleEvaluationResult{
		Team:         team,
		RuleId:       r.Spec().Id(),
		Completeness: 0,
		Reason:       "",
		ExecError:    nil,
	}

	templateBytes, err := os.ReadFile(fmt.Sprintf("%s/github/classroom/assignment-public-template/weekend-mysql/init.sql", os.Getenv("HOME")))
	if err != nil {
		result.Reason = "Internal error"
		result.ExecError = fmt.Errorf("failed to read file: %v", err)
		return result
	}

	assignmentBytes, err := os.ReadFile(fmt.Sprintf("%s/weekend-mysql/init.sql", team.GetRepoPath()))
	if err != nil {
		result.Reason = "The SQL script is missing"
		result.ExecError = err
		return result
	}

	var (
		originalContent = string(templateBytes)
		content         = string(assignmentBytes)
		lowerContent    = strings.ToLower(content)
		daysOfWeek      = map[string]string{
			"Monday":    "1",
			"Tuesday":   "2",
			"Wednesday": "3",
			"Thursday":  "4",
			"Friday":    "5",
			"Saturday":  "6",
			"Sunday":    "7",
		}
	)

	if content == originalContent {
		result.Reason = "The SQL script is not modified"
		return result
	}

	if strings.Contains(content, "VARCHAR") || strings.Contains(content, "INT") {
		result.Completeness += 0.2
	} else {
		result.Reason += "The SQL script does not contain VARCHAR or INT. "
	}

	if matched, _ := regexp.MatchString("INSERT (.+)INTO", content); matched {
		result.Completeness += 0.1
	} else {
		result.Reason += "The SQL script does not contain insert statement. "
	}

	for day, dayId := range daysOfWeek {
		if strings.Contains(lowerContent, strings.ToLower(day)) || strings.Contains(content, dayId) {
			result.Completeness += 0.1
		} else {
			result.Reason += fmt.Sprintf("Missing weekday %s. ", day)
		}
	}

	if result.Completeness == 1 {
		result.Reason = "fully passed"
	} else {
		result.Reason = strings.TrimSpace(result.Reason)
	}

	return result
}
