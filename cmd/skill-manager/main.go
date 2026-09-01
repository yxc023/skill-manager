// Package main is the entry point for the skill-manager CLI.
package main

import (
	"fmt"
	"os"

	"github.com/yxc023/skill-manager/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
