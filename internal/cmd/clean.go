package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/yxc023/skill-manager/internal/skillmanager"
	"github.com/yxc023/skill-manager/internal/ui"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove skill-manager-managed symlinks/directories at all configured targets (cache is preserved)",
	RunE: func(cmd *cobra.Command, args []string) error {
		manifestPath, _ := cmd.Flags().GetString("manifest")
		m, err := skillmanager.LoadManifest(manifestPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, ui.Fail(err.Error()))
			os.Exit(1)
		}

		removed := 0
		skipped := 0
		for name, sd := range m.Skills {
			if !sd.IsEnabled() {
				continue
			}
			targets := sd.EffectiveTargets(m.Targets)
			for _, t := range targets {
				dest, err := t.ResolvedPath(sd.Category, name)
				if err != nil {
					fmt.Fprintln(os.Stderr, ui.Fail(err.Error()))
					continue
				}
				info, err := os.Lstat(dest)
				if err != nil {
					// already absent — nothing to clean
					skipped++
					continue
				}
				if info.Mode()&os.ModeSymlink != 0 {
					if err := os.Remove(dest); err != nil {
						fmt.Fprintln(os.Stderr, ui.Fail(fmt.Sprintf("remove symlink %s: %v", dest, err)))
						continue
					}
				} else if info.IsDir() {
					if err := os.RemoveAll(dest); err != nil {
						fmt.Fprintln(os.Stderr, ui.Fail(fmt.Sprintf("remove dir %s: %v", dest, err)))
						continue
					}
				} else {
					fmt.Fprintln(os.Stderr, ui.Warn(fmt.Sprintf("skip %s (not symlink/dir)", dest)))
					skipped++
					continue
				}
				fmt.Printf("%s removed %s\n", ui.OK("✓"), dest)
				removed++
			}
		}
		fmt.Printf("\nRemoved: %d, Skipped: %d\n", removed, skipped)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)
}