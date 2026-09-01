// Package cmd wires up all cobra subcommands for the skill-manager CLI.
package cmd

import (
	"github.com/spf13/cobra"
)

// Version is the skill-manager release version. Overridden at build time via:
//
//	go build -ldflags "-X github.com/yxc023/skill-manager/internal/cmd.Version=0.3.0" ./cmd/skill-manager
var Version = "0.3.0"

// rootCmd is the base command invoked when the binary runs without a subcommand.
var rootCmd = &cobra.Command{
	Use:     "skill-manager",
	Short:   "Declarative skill manager for AI agents",
	Long:    "skill-manager clones skill repos from git, computes hashes, and links them into agent directories.",
	Version: Version,
}

// Execute runs the root command and returns any error encountered.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.SetVersionTemplate("skill-manager {{.Version}}\n")

	rootCmd.PersistentFlags().StringP("manifest", "m", "skills-manage.json", "path to manifest file")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "print shell commands as they run")
}
