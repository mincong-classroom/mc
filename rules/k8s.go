package rules

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"time"
)

const (
	nginxPodName       = "nginx"
	nginxManifestPath  = "k8s/pod-nginx.yaml"
	nginxContainerPort = 80

	javaPodName       = "weekend-server"
	javaContainerPort = 8080
	javaManifestPath  = "k8s/pod-weekend-server.yaml"

	nginxReplicaSetManifestPath = "k8s/replicaset-nginx.yaml"
	javaDeploymentManifestPath  = "k8s/deployment-weekend-server.yaml"
	javaServiceManifestPath     = "k8s/service-weekend-server.yaml"

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
