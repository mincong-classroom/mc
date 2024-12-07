package rules

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mincong-classroom/mc/common"
)

const (
	nginxPodName       = "nginx"
	nginxManifestPath  = "k8s/pod-nginx.yaml"
	nginxContainerPort = 80
	javaPodName        = "weekend-server"
	javaContainerPort  = 8080
	javaManifestPath   = "k8s/pod-weekend-server.yaml"
	localPort          = 8080
)

type K8sNginxPodRule struct {
	Assignments map[string]common.TeamAssignmentL3
}

type K8sJavaPodRule struct {
	Assignments map[string]common.TeamAssignmentL3
}

func (r K8sNginxPodRule) Spec() common.RuleSpec {
	return common.RuleSpec{
		LabId:    "L3",
		Symbol:   "NGY",
		Name:     "Nginx Yaml Test",
		Exercice: "3",
		Description: fmt.Sprintf(`
The team is expected to create a new pod running with Nginx using a kubectl-apply
command. This pod should be reachable using the port %d and should be named as %q.
The manifest should be saved under the path %s of the Git repository.`,
			nginxContainerPort, nginxPodName, nginxManifestPath),
	}
}

func (r K8sJavaPodRule) Spec() common.RuleSpec {
	return common.RuleSpec{
		LabId:    "L3",
		Symbol:   "JVY",
		Name:     "Java Yaml Test",
		Exercice: "4",
		Description: fmt.Sprintf(`
The team is expected to create a new pod running with Java using a kubectl-apply
command. This pod should be reachable using the port %d and should be named as %q.
The manifest should be saved under the path %s of the Git repository. The HTTP
response of the root API (/) should contains the team and authors.`,
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

func kubePortForward(ctx context.Context, namespace, podName string, localPort, remotePort int) error {
	cmd := exec.CommandContext(ctx,
		"kubectl", "port-forward",
		"-n", namespace,
		podName, fmt.Sprintf("%d:%d", localPort, remotePort),
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start the process
	if err := cmd.Start(); err != nil {
		fmt.Printf("Failed to start port-forward: %v\n", err)
		return fmt.Errorf("failed to start port-forward: %w\nstderr: %s", err, stderr.String())
	}

	// Wait briefly to ensure the port-forward is established
	select {
	case <-time.After(2 * time.Second):
		return nil
	case <-ctx.Done():
		err := fmt.Errorf("context canceled while waiting for port-forward")
		fmt.Print("Context canceled while waiting for port-forward\n")
		killErr := cmd.Process.Kill() // Ensure the process is terminated if the context is canceled
		if killErr != nil {
			return fmt.Errorf("%w, %w", err, killErr)
		}
		return err
	}
}

func getHttpContent(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
