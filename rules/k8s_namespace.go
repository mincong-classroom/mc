package rules

import "github.com/mincong-classroom/mc/common"

var k8sNamespaceRuleSpec = common.RuleSpec{
	LabId:    "L4",
	Symbol:   "PNS",
	Exercice: "3",
	Name:     "K8s PetClinic Namespace Test",
	Description: `
The team is expected to create two namespaces: "prod" and "dev". Each namespace
should contain the whole stack in microservice, including the API gateway and
the backend services. All resources should be up and running.`,
}
