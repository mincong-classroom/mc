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
users to input and view email addresses. This is a manual verification.`,
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
users to input and view qualifications. This is a manual verification.`,
}
