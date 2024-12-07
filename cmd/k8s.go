package cmd

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/mincong-classroom/mc/common"
	"github.com/spf13/cobra"
)

var k8sCmd = &cobra.Command{
	Use:   "k8s",
	Short: "Perform Kubernetes operations",
}

var createNamespaceCmd = &cobra.Command{
	Use:   "create-namespaces",
	Short: `Create new Kubernetes namespaces, one per team. The namespace name is the team name, such as "teams-red".`,
	Run:   createNamespaces,
}

func init() {
	k8sCmd.AddCommand(createNamespaceCmd)
}

func createNamespaces(cmd *cobra.Command, args []string) {
	teams, err := listTeams()
	if err != nil {
		fmt.Printf("Failed to list teams: %v", err)
		return
	}

	for _, team := range teams {
		if err := createNamespace(team); err != nil {
			fmt.Printf("Failed to create namespace for team %s: %v\n", team.Name, err)
		} else {
			fmt.Printf("Namespace created for team %s\n", team.Name)
		}
	}
}

func createNamespace(team common.Team) error {
	namespace := "team-" + team.Name
	createCmd := exec.Command("kubectl", "create", "namespace", namespace)

	// Capture stdout and stderr
	var out bytes.Buffer
	var stderr bytes.Buffer
	createCmd.Stdout = &out
	createCmd.Stderr = &stderr

	// Run the command
	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("failed to create namespace: %v\nstderr: %s", err, stderr.String())
	}
	return nil
}
