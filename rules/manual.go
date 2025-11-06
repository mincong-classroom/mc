package rules

import (
	"github.com/mincong-classroom/mc/common"
)

type ManualRule struct {
	ruleSpec common.RuleSpec
}

func (r ManualRule) Spec() common.RuleSpec {
	return r.ruleSpec
}

func (r ManualRule) Run(team common.Team, _ string) common.RuleEvaluationResult {
	return common.RuleEvaluationResult{
		Team:         team,
		RuleId:       r.Spec().Id(),
		Completeness: 0,
		Reason:       "Manual grading is required",
		ExecError:    nil,
	}
}
