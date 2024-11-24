// main.go
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := cmd.rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
