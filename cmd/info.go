package cmd

import (
	"fmt"

	"github.com/mincong-classroom/mc/common"
	"github.com/spf13/cobra"
)

var (
	version   = "0.1.0"
	goVersion = "1.22"
	aiModel   = "gpt-4"
	aiVendor  = "OpenAI"
	projectID = "mc-123456"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Display CLI information",
	Run:   runInfo,
}

func runInfo(cmd *cobra.Command, args []string) {
	teams, _ := common.ListTeams()
	fmt.Printf("CLI Version: %s\n", version)
	fmt.Printf("Go Version: %s\n", goVersion)
	fmt.Printf("AI Model: %s\n", aiModel)
	fmt.Printf("AI Vendor: %s\n", aiVendor)
	fmt.Printf("Project ID: %s\n", projectID)
	fmt.Printf("Registered Teams: %d\n", len(teams))
}
