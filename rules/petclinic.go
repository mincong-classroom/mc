package rules

import "github.com/mincong-classroom/mc/common"

var petclinicGenaiServiceRuleSpec = common.RuleSpec{
	LabId:    "L5",
	Symbol:   "GAI",
	Exercice: "2",
	Name:     "PetClinic GenAI Service Test",
	Description: `
The team is expected to integrate the GenAI service to the existing
microservice stack. This includes the configuration of the Service, Deployment,
the API routes in the API Gateway, the API key stored in the Secret, and the
related troubleshooting to ensure that the entire solution works. This is a
manual verification.`,
}
