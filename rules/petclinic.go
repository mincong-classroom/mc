package rules

import "github.com/mincong-classroom/mc/common"

var petclinicEmailSupportRuleSpec = common.RuleSpec{
	LabId:    "L4",
	Symbol:   "EML",
	Exercice: "4",
	Name:     "PetClinic Email Support Test",
	Description: `
The team is expected to make necessary changes to support the email field for
the customers in the PetClinic application. This includes updating the database
schema, modifying the backend services, and ensuring that the frontend UI allows
users to input and view email addresses. This is a manual verification. This
exercise is a bonus for Lab Session 4. Its score is not included in the base
score of 20, but students who complete it can earn additional points.`,
}

var petclinicVeterinarianQualificationRuleSpec = common.RuleSpec{
	LabId:    "L4",
	Symbol:   "VTQ",
	Exercice: "4",
	Name:     "PetClinic Veterinarian Qualification Test",
	Description: `
The team is expected to make necessary changes to support the qualification
field for the veterinarians in the PetClinic application. This includes updating the database
schema, modifying the backend services, and ensuring that the frontend UI allows
users to input and view qualifications. This is a manual verification. This
exercise is a bonus for Lab Session 4. Its score is not included in the base
score of 20, but students who complete it can earn additional points.`,
}

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
