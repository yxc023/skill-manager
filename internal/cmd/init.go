package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/yxc023/skill-manager/internal/skillmanager"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a starter skills-manage.json in the current directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		manifestPath, _ := cmd.Flags().GetString("manifest")
		if _, err := os.Stat(manifestPath); err == nil {
			return fmt.Errorf("%s already exists", manifestPath)
		}
		m := &skillmanager.Manifest{
			Version: skillmanager.ManifestVersion,
			Targets: []skillmanager.Target{
				{Agent: "opencode", Path: ".opencode/skills/{category}", Mode: skillmanager.ModeSymlink},
			},
			Skills: map[string]*skillmanager.SkillDef{},
		}
		if err := skillmanager.SaveManifest(m, manifestPath); err != nil {
			return fmt.Errorf("write manifest: %w", err)
		}
		fmt.Printf("Created %s with default target (opencode)\n", manifestPath)
		fmt.Println("Add entries under \"skills\" and run `skill-manager install`.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
