package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/yxc023/skill-manager/internal/skillmanager"
	"github.com/yxc023/skill-manager/internal/ui"
)

var (
	installHardReset bool
	installUpdate    bool
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Clone/fetch each skill's source and link/copy into targets",
	RunE: func(cmd *cobra.Command, args []string) error {
		manifestPath, _ := cmd.Flags().GetString("manifest")
		verbose, _ := cmd.Flags().GetBool("verbose")

		m, err := skillmanager.LoadManifest(manifestPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, ui.Fail(err.Error()))
			os.Exit(1)
		}

		cacheRoot, err := skillmanager.ExpandCacheRoot()
		if err != nil {
			return fmt.Errorf("resolve cache root: %w", err)
		}

		lock, err := skillmanager.LoadLock(manifestPath + ".lock.json")
		if err != nil {
			return fmt.Errorf("read lock: %w", err)
		}

		installed := 0
		skipped := 0
		errored := 0
		for name, sd := range m.Skills {
			if !sd.IsEnabled() {
				continue
			}
			targets := sd.EffectiveTargets(m.Targets)
			if len(targets) == 0 {
				fmt.Fprintln(os.Stderr, ui.Warn(fmt.Sprintf("skill %s has no targets — skipping", name)))
				skipped++
				continue
			}
			fmt.Printf("→ %s\n", ui.Name(name))
			results, err := skillmanager.Install(name, sd, targets, skillmanager.InstallOpts{
				CacheRoot: cacheRoot,
				Verbose:   verbose,
				HardReset: installHardReset || installUpdate,
			})
			if err != nil {
				fmt.Fprintln(os.Stderr, ui.Fail(fmt.Sprintf("  %v", err)))
				errored++
				continue
			}
			for _, r := range results {
				fmt.Printf("  %s [%s] %s @ %s", ui.OK(r.Action), r.Mode, r.Target, r.ResolvedAt)
				if r.Hash != "" {
					fmt.Printf("  hash=%s", r.Hash[:8])
				}
				fmt.Println()
				lock.Skills[name] = &skillmanager.LockEntry{
					Source:          sd.Source,
					Category:        sd.Category,
					SkillFolderHash: r.Hash,
				}
				installed++
			}
		}

		if err := skillmanager.SaveLock(lock, manifestPath+".lock.json"); err != nil {
			return fmt.Errorf("write lock: %w", err)
		}

		fmt.Printf("\nInstalled: %d, Skipped: %d, Errors: %d\n", installed, skipped, errored)
		if errored > 0 {
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.Flags().BoolVar(&installHardReset, "hard-reset", false, "force re-clone even if cache exists")
	installCmd.Flags().BoolVar(&installUpdate, "update", false, "alias: hard reset cache before installing")
}
