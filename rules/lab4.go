package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mincong-classroom/mc/common"
)

// Lab Session 4 (Kubernetes Networking) — 2025/2026 redesign.
//
// The lab has three exercises: (1) create the "classroom" namespace, (2) deploy
// and expose the team-info-server in that namespace and validate cross-namespace
// DNS, and (3) route "/api/about" from the API Gateway to the team-info Service.
// See ../esigelec/slides/lab-4.md for the reference implementation.

const (
	teamInfoNamespace     = "classroom"
	teamInfoServiceName   = "team-info"
	teamInfoServicePort   = 80   // the Service port, mapped to the container port
	teamInfoContainerPort = 8090 // the port the team-info-server listens on
	teamInfoManifestPath  = "k8s/lab-4/app-team-info.yaml"
	teamInfoLocalPort     = 8090 // local port used for port-forward (avoids 8080 used by petclinic)
)

// Exercise 1 — create the "classroom" namespace and list the namespaces. There
// is nothing durable to inspect after the fact, so this is graded from the
// report evidence.
var k8sNamespaceCreateRuleSpec = common.RuleSpec{
	LabId:    "L4",
	Symbol:   "NSC",
	Exercice: "1",
	Name:     "Namespace Creation Test",
	Description: `
The team is expected to create a new namespace called "classroom" and list all
the existing namespaces in the cluster. The namespace can be created either
imperatively with "kubectl create namespace" or declaratively via a YAML
manifest applied with "kubectl apply". This is a manual verification based on the
evidence provided in the report.`,
}

// Exercise 3 — the "About" page is already shipped in the API Gateway image; the
// student only wires the network route. Verifying "no hard-coding" and reading
// the ConfigMap route requires the full microservice stack, so this stays manual.
var apiGatewayAboutRouteRuleSpec = common.RuleSpec{
	LabId:    "L4",
	Symbol:   "AGR",
	Exercice: "3",
	Name:     "API Gateway About Route Test",
	Description: `
The team is expected to make the PetClinic "About" page work by configuring
Kubernetes networking only (no frontend or Java code). They must route requests
for "/api/about" from the API Gateway to the "team-info" Service in the
"classroom" namespace, using cross-namespace DNS ("team-info.classroom") rather
than hard-coding the team information. The route is added to the
"api-gateway-config" ConfigMap. Validation: opening http://localhost:8080/#!/about
displays the information served by the Team Info Server, and
"curl http://localhost:8080/api/about/" returns the expected JSON. This is a
manual verification.`,
}

// K8sTeamInfoServerRule grades Exercise 2. It is automated: it applies the
// committed manifest, waits for the Pod, port-forwards the Service, and inspects
// the JSON response. The cross-namespace DNS write-up (querying from "classroom"
// vs "default") is documented in the report and reviewed manually.
type K8sTeamInfoServerRule struct{}

func (r K8sTeamInfoServerRule) Spec() common.RuleSpec {
	return common.RuleSpec{
		LabId:    "L4",
		Symbol:   "TIS",
		Exercice: "2",
		Name:     "Team Info Server Deployment Test",
		Description: fmt.Sprintf(`
The team is expected to deploy and expose the classroom application
"team-info-server" (image mincongclassroom/team-info-server) in the %q namespace.
They must create a Deployment named %q with 1 replica and a ClusterIP Service
named %q exposing port %d and targeting the container port %d, stored in a single
manifest committed at %q. The web server fails to start until the required
TEAM_ID environment variable is set (the same style of fix as Lab Session 1).
This rule applies the manifest, waits for the Pod, and queries the Service: it
checks that the manifest is committed (0.2), the Service is reachable (0.3),
TEAM_ID matches the team name (0.3), and the team members are listed (0.2). The
cross-namespace DNS validation is documented in the report and reviewed manually.`,
			teamInfoNamespace, teamInfoServiceName, teamInfoServiceName,
			teamInfoServicePort, teamInfoContainerPort, teamInfoManifestPath),
	}
}

func (r K8sTeamInfoServerRule) Run(team common.Team, _ string) common.RuleEvaluationResult {
	result := common.RuleEvaluationResult{
		Team:         team,
		RuleId:       r.Spec().Id(),
		Completeness: 0,
		Reason:       "",
		ExecError:    nil,
	}

	manifestPath := fmt.Sprintf("%s/%s", team.GetRepoPath(), teamInfoManifestPath)
	if _, err := os.ReadFile(manifestPath); err != nil {
		result.Reason = "The manifest file is missing: " + manifestPath + ", please grade manually."
		result.ExecError = err
		return result
	}
	result.Completeness += 0.2 // manifest committed at the expected path

	// The manifest targets the "classroom" namespace; make sure it exists before applying.
	ensureNamespace(teamInfoNamespace)

	if err := kubeApply(manifestPath, teamInfoNamespace); err != nil {
		result.Reason = "Failed to apply the manifest: " + manifestPath
		fmt.Println(result.Reason)
		fmt.Println(err)
		result.ExecError = err
		return result
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const sleepSeconds = 5
	fmt.Printf("Waiting %d seconds for the Pod to be ready...\n", sleepSeconds)
	time.Sleep(sleepSeconds * time.Second)

	fmt.Println("Setting up port-forward...")
	if err := kubePortForward(ctx, teamInfoNamespace, "svc/"+teamInfoServiceName, teamInfoLocalPort, teamInfoServicePort); err != nil {
		result.ExecError = fmt.Errorf("failed to set up port-forward: %v", err)
		result.Reason = "The team-info Service is not reachable"
		return result
	}

	fmt.Println("Fetching content from the team-info Service...")
	content, err := getHttpContent(fmt.Sprintf("http://localhost:%d/", teamInfoLocalPort))
	if err != nil {
		result.ExecError = fmt.Errorf("failed to query team-info service: %v", err)
		result.Reason = "The team-info Service did not return a response"
		return result
	}
	result.Completeness += 0.3 // service reachable and responding

	var payload struct {
		Team    string   `json:"team"`
		Members []string `json:"members"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		result.Reason = "The team-info response is not valid JSON: " + content
		result.ExecError = err
		return result
	}

	if payload.Team == team.Name {
		result.Completeness += 0.3 // TEAM_ID correctly set to the team name
	} else {
		result.Reason += fmt.Sprintf("TEAM_ID mismatch: expected %q, got %q. ", team.Name, payload.Team)
	}

	if team.HasAllMembers(content) {
		result.Completeness += 0.2 // members present in the response
	} else {
		result.Reason += "Not all team members are listed. "
	}

	if result.Completeness == 1 {
		result.Reason = "team-info Server deployed correctly and reachable via the Service"
	}
	result.Reason = strings.TrimSpace(result.Reason)
	return result
}

// ensureNamespace creates the namespace if it does not already exist. The error
// (including "already exists") is ignored on purpose.
func ensureNamespace(namespace string) {
	_ = exec.Command("kubectl", "create", "namespace", namespace).Run()
}
