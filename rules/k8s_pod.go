package rules

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mincong-classroom/mc/common"
)

type K8sNginxPodRule struct {
	Assignments map[string]common.TeamAssignmentL3
}

type K8sJavaPodRule struct {
	Assignments map[string]common.TeamAssignmentL3
}

func (r K8sNginxPodRule) Spec() common.RuleSpec {
	return common.RuleSpec{
		LabId:    "L2",
		Symbol:   "NGY",
		Name:     "Nginx YAML Test",
		Exercice: "3",
		Description: fmt.Sprintf(`
The team is expected to create a new Pod running with Nginx using a kubectl-apply
command. This Pod should be reachable using the port %d and should be named as
%q. The manifest should be saved under the path %s
of the Git repository. Also, a team label should be added to the Pod definition.`,
			nginxContainerPort, nginxPodName, nginxManifestPath),
	}
}

func (r K8sJavaPodRule) Spec() common.RuleSpec {
	return common.RuleSpec{
		LabId:    "L3",
		Symbol:   "JVY",
		Name:     "Java YAML Test",
		Exercice: "4",
		Description: fmt.Sprintf(`
The team is expected to create a new pod running with Java using a kubectl-apply
command. This pod should be reachable using the port %d and should be named as
%q. The manifest should be saved under the path %s
of the Git repository. The Pod should contain 2 labels, app=spring-petclinic and
team=${team}. The Pod must be up and running.`,
			javaContainerPort, javaPodName, javaManifestPath),
	}
}

func (r K8sNginxPodRule) Run(team common.Team, _ string) common.RuleEvaluationResult {
	result := common.RuleEvaluationResult{
		Team:         team,
		RuleId:       r.Spec().Id(),
		Completeness: 0,
		Reason:       "",
		ExecError:    nil,
	}
	var (
		manifestPath = fmt.Sprintf("%s/%s", team.GetRepoPath(), nginxManifestPath)
		namespace    = team.GetKubeNamespace()
	)
	if _, err := os.ReadFile(manifestPath); err != nil {
		result.Reason = "The manifest file is missing: " + manifestPath + ", please grade manually."
		result.ExecError = err
		return result
	}

	err := kubeApply(manifestPath, namespace)
	if err != nil {
		result.Reason = "Failed to apply the manifest: " + manifestPath
		fmt.Println(result.Reason)
		fmt.Println(err)
		result.ExecError = err
		return result
	} else {
		fmt.Println("The manifest has been applied successfully")
	}

	// Create a context to manage the kubectl process lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const sleepSeconds = 5
	fmt.Printf("Waiting %d seconds for the Pod to be ready...\n", sleepSeconds)
	time.Sleep(sleepSeconds * time.Second) // Wait for the pod to be ready

	// Start port-forwarding
	fmt.Println("Setting up port-forward...")
	if err := kubePortForward(ctx, namespace, nginxPodName, localPort, nginxContainerPort); err != nil {
		result.ExecError = fmt.Errorf("failed to set up port-forward: %v", err)
		fmt.Printf("Failed to port-forward: %v\n", err)
		customPodName := r.Assignments[team.Name].NginxPodName
		if customPodName == "" {
			fmt.Println("Fallback port-forward: no custom pod name")
			return result
		}
		fmt.Println("Fallback port-forward: custom pod name is " + customPodName)
		fmt.Println("Setting up port-forward again with custom name " + customPodName + "...")
		if err := kubePortForward(ctx, namespace, customPodName, localPort, nginxContainerPort); err != nil {
			result.ExecError = fmt.Errorf("failed to set up port-forward with custom name: %v", err)
			return result
		}
		fmt.Println("Port-forwarding with custom name has been set up successfully")
	} else {
		result.Completeness += 0.2
	}
	defer cancel() // Ensure the port-forward process is terminated when we're done

	fmt.Println("Fetching content from pod...")
	content, err := getHttpContent(fmt.Sprintf("http://localhost:%d", localPort))
	if err != nil {
		result.ExecError = fmt.Errorf("failed to curl localhost: %v", err)
		return result
	}

	if strings.Contains(content, "Welcome to nginx!") {
		result.Completeness += 0.8
		result.Reason = "Nginx is running successfully"
	} else {
		result.Reason = "Nginx is not running successfully"
	}
	return result
}

func (r K8sJavaPodRule) Run(team common.Team, _ string) common.RuleEvaluationResult {
	result := common.RuleEvaluationResult{
		Team:         team,
		RuleId:       r.Spec().Id(),
		Completeness: 0,
		Reason:       "",
		ExecError:    nil,
	}
	var (
		manifestPath = fmt.Sprintf("%s/%s", team.GetRepoPath(), javaManifestPath)
		namespace    = team.GetKubeNamespace()
	)
	if _, err := os.ReadFile(manifestPath); err != nil {
		result.Reason = "The manifest file is missing: " + manifestPath + ", please grade manually."
		result.ExecError = err
		return result
	}

	err := kubeApply(manifestPath, namespace)
	if err != nil {
		result.Reason = "Failed to apply the manifest: " + manifestPath
		fmt.Println(result.Reason)
		fmt.Println(err)
		result.ExecError = err
		return result
	} else {
		fmt.Println("The manifest has been applied successfully")
	}

	// Create a context to manage the kubectl process lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	time.Sleep(5 * time.Second) // Wait for the pod to be ready

	// Start port-forwarding
	fmt.Println("Setting up port-forward...")
	if err := kubePortForward(ctx, namespace, javaPodName, localPort, javaContainerPort); err != nil {
		result.ExecError = fmt.Errorf("failed to set up port-forward: %v", err)
		fmt.Printf("Failed to port-forward: %v\n", err)
	}
	defer cancel() // Ensure the port-forward process is terminated when we're done

	fmt.Println("Fetching content from pod...")
	content, err := getHttpContent(fmt.Sprintf("http://localhost:%d", localPort))
	if err != nil {
		result.ExecError = fmt.Errorf("failed to curl localhost: %v", err)
		return result
	} else {
		result.Completeness += 0.8
	}

	if strings.Contains(content, team.Name) {
		result.Completeness += 0.1
	}
	if team.HasAllMembers(content) {
		result.Completeness += 0.1
	}

	return result
}

func kubeApply(manifestPath, namespace string) error {
	// Verify the manifest file exists
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return fmt.Errorf("manifest file does not exist: %s", manifestPath)
	}

	// Prepare kubectl apply command
	applyCmd := exec.Command("kubectl", "apply", "-f", manifestPath, "-n", namespace)

	// Capture stdout and stderr
	var out bytes.Buffer
	var stderr bytes.Buffer
	applyCmd.Stdout = &out
	applyCmd.Stderr = &stderr

	// Run the command
	if err := applyCmd.Run(); err != nil {
		return fmt.Errorf("failed to apply manifest: %v\nstderr: %s", err, stderr.String())
	}

	// Print the output
	fmt.Printf("kubectl apply output:\n%s\n", out.String())
	return nil
}

var k8sRunNginxPodRuleSet = common.RuleSpec{
	LabId:    "L2",
	Symbol:   "RUN",
	Exercice: "2",
	Name:     "Kubernetes Run Nginx Pod Test",
	Description: `
The team is expected to create a new Pod using the command kubectl-run. The Pod
needs to be running and accessible. The students should provide evidence of the
HTTP response from the Pod, such as a screenshot or the command output. A list
of fields are expected to be filled in the report for describing the
characteristics of the Pod. Also, the resource should be deleted after the
test.`,
}

var k8sOperateJavaPodRuleSet = common.RuleSpec{
	LabId:    "L2",
	Symbol:   "OJP",
	Exercice: "5",
	Name:     "Kubernetes Operate Java Pod Test",
	Description: `
The team is expected to perform basic operations on the Java Pod they created.
These operations include executing a command inside the Pod to get the process
ID (PID) of the Java application, retrieving logs from the Pod, and finding the
Pod using kubectl-get with label selectors. The students should provide evidence
of each operation, such as command outputs or screenshots.`,
}

var k8sFixBrokenPodRuleSet = common.RuleSpec{
	LabId:    "L2",
	Symbol:   "FBP",
	Exercice: "6",
	Name:     "Kubernetes Fix Broken Pod Test",
	Description: `
The team is expected to troubleshoot and fix a broken Pod provided by the
teacher. The Pod is intentionally misconfigured to simulate common issues that
may arise in a Kubernetes environment. The students need to identify the two
problems, including the incorrect Docker image and the missing team name in the
environment variables. After fixing the issues, the Pod should be up and
running.`,
}
