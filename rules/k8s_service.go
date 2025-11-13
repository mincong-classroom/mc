package rules

import (
	"github.com/mincong-classroom/mc/common"
)

var k8sHelloServerServiceRuleSpec = common.RuleSpec{
	LabId:    "L4",
	Symbol:   "HSV",
	Exercice: "1",
	Name:     "K8s Hello Server Service Test",
	Description: `
The team is expected to expose the hello-server as a Kubernetes Service. They
are expected to create a Deployment for the container image "hello-server";
create a Service called "hello" under the port 80; and perform a validation to
prove that the networking is working successfully. This is a manual
verification.`,
}

var k8sNodePortRuleSpec = common.RuleSpec{
	LabId:    "L4",
	Symbol:   "NPT",
	Exercice: "2",
	Name:     "K8s NodePort Test",
	Description: `
The team is expected to change the Service type of the API Gateway from
ClusterIP to NodePort so that it can be accessed externally. They are expected
to perform a validation to prove that the networking is working successfully.
This is a manual verification.`,
}
