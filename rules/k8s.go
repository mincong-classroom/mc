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
	javaServiceManifestPath         = "k8s/service-weekend-server.yaml"

	localPort = 8080
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
data in the cluster.
`,
}
