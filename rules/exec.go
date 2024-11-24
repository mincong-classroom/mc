package rules

import (
	"fmt"
	"os"
	"os/exec"
)

func runScript(script string) error {
	// Create a temporary script file
	tmpFile, err := os.CreateTemp("", "grade-*.sh")
	if err != nil {
		return fmt.Errorf("failed to create temp script: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write the script content
	if _, err := tmpFile.WriteString(script); err != nil {
		return fmt.Errorf("failed to write script: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close script file: %v", err)
	}

	// Make the script executable
	if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
		return fmt.Errorf("failed to make script executable: %v", err)
	}

	// Execute the script
	cmd := exec.Command("bash", tmpFile.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run grading script: %v", err)
	}

	return nil
}
