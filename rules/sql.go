package rules

import (
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
The team is expected to perform operations to the MySQL container. The
operations related to database connection, how to USE database, and SELECT
data from the target table should be listed clearly in the report.`,
	}
}

func (r SqlInitRule) Run(team common.Team, _ string) common.RuleEvaluationResult {
	return common.RuleEvaluationResult{
		Team:         team,
		RuleId:       r.Spec().Id(),
		Completeness: 0,
		Reason:       "Check the report manually",
		ExecError:    nil,
	}
}
