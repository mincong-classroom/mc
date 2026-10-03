package rules

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"time"

	"github.com/mincong-classroom/mc/common"
)

const (
	nginxPodName       = "nginx"
	nginxManifestPath  = "k8s/pod-nginx.yaml"
	nginxContainerPort = 80

	petclinicPodName         = "spring-petclinic"
	petclinicContainerPort   = 8080
	petclinicPodManifestPath = "k8s/pod-petclinic.yaml"

	petclinicReplicaSetManifestPath = "k8s/replicaset-petclinic.yaml"
	petclinicDeploymentManifestPath = "k8s/deployment-petclinic.yaml"

	localPort = 8080

	// podReadyTimeout covers the pull of the image, e.g. about 200 MB for PetClinic.
	podReadyTimeout = 3 * time.Minute
	// appStartTimeout covers the start of the application once its container runs, e.g. the
	// Spring context of PetClinic, longer when the image runs through emulation.
	appStartTimeout = 2 * time.Minute
)

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
	fmt.Printf("Starting port-forward: %v\n", cmd)
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

// kubeWaitPodReady waits until the containers of the Pod run, so that a port-forward can reach
// it. A Pod without a readiness probe is ready as soon as its containers start, before the
// application listens: see getPodHttpContent.
func kubeWaitPodReady(namespace, podName string, timeout time.Duration) error {
	cmd := exec.Command("kubectl", "wait",
		"-n", namespace,
		"--for=condition=Ready",
		"pod/"+podName,
		"--timeout="+timeout.String(),
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pod %s is not ready: %w\n%s", podName, err, out)
	}
	return nil
}

// getPodHttpContent fetches the home page of the Pod through a port-forward, again until it
// answers or the timeout expires. Each attempt needs a new port-forward: kubectl exits ("lost
// connection to pod") at the first connection that the Pod refuses, which is what happens
// while the application is still starting.
func getPodHttpContent(namespace, podName string, remotePort int, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		content, err := tryPodHttpContent(namespace, podName, remotePort)
		if err == nil {
			return content, nil
		}
		if time.Now().After(deadline) {
			return "", err
		}
		fmt.Printf("No answer yet, retrying in 5 seconds: %v\n", err)
		time.Sleep(5 * time.Second)
	}
}

func tryPodHttpContent(namespace, podName string, remotePort int) (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // terminates the port-forward process
	if err := kubePortForward(ctx, namespace, podName, localPort, remotePort); err != nil {
		return "", err
	}
	return getHttpContent(fmt.Sprintf("http://localhost:%d", localPort))
}

func getHttpContent(url string) (string, error) {
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
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

var k8sControlPlaneRuleSet = common.RuleSpec{
	LabId:    "L2",
	Symbol:   "CTL",
	Exercice: "1",
	Name:     "Kubernetes Control Plane Test",
	Description: `
The team is expected to list all the Pods running in all namespaces in
Kubernetes. Then, list all the nodes available in the cluster. It allows the
students to get familiar with the Kubernetes and ensure that the command line
tool kubectl is properly installed on their local machines.`,
}

var k8sSecretRuleSpec = common.RuleSpec{
	LabId:    "L5",
	Symbol:   "SEC",
	Exercice: "1",
	Name:     "Kubernetes Secret Test",
	Description: `
The team is expected to create a Kubernetes Secret to an API key as sensitive
data in the cluster. The secret must be named as "openai" and data entry should
be "api-key". The value should be encoded in base64 format. And the resource
should be applied to the "dev" namespace. The team should verify the result by
putting the analysis in the report. A -30% penalty will be applied if the team
exposes the API key in the report. This is not acceptable: they have been
warned during the lecture; and in the description of the lab exercise. This is
a manual verification.
`,
}
