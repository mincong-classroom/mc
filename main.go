package main

import (
	"os"

	"github.com/mincong-classroom/mc/cmd"
)

func main() {
	// Cobra already prints the error.
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
