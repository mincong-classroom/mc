// main.go
package main

import (
	"fmt"
	"os"
	"github.com/mincong-classroom/grading/cmd"
)

func main() {
	if err := cmd.rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
