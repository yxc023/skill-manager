package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/yxc023/skill-manager/internal/skillmanager"
	"github.com/yxc023/skill-manager/internal/ui"
)

var outdatedCmd = &cobra.Command{
	Use:   "outdated",
	Short: "Show which skills have new commits upstream (compared to locked HEAD)",
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
			return err
		}

		lock, err := skillmanager.LoadLock(manifestPath + ".lock.json")
		if err != nil {
			return err
		}

		g := skillmanager.NewGitRunner(verbose)
		outdated := 0
		for name, sd := range m.Skills {
			if !sd.IsEnabled() || sd.Source.Type == "local" {
				continue
			}
			cacheDir, err := sd.Source.CachePath(cacheRoot)
			if err != nil {
				fmt.Fprintln(os.Stderr, ui.Fail(fmt.Sprintf("%s: %v", name, err)))
				continue
			}
			if info, err := os.Stat(cacheDir); err != nil || !info.IsDir() {
				fmt.Printf("  %s %s (no cache — run install)\n", ui.Warn("skip"), name)
				continue
			}
			ref := sd.Source.Ref
			if ref == "" {
				ref = "main"
			}
			// Fetch first to get latest remote
			if err := g.FetchIn(cacheDir, ref); err != nil {
				fmt.Fprintf(os.Stderr, "  %s %s: %v\n", ui.Fail("fetch error"), name, err)
				continue
			}
			head, err := g.RevParseHead(cacheDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  %s %s: %v\n", ui.Fail("rev-parse HEAD"), name, err)
				continue
			}
			remote, err := g.RevParseRemoteRef(cacheDir, ref)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  %s %s: %v\n", ui.Fail("rev-parse remote"), name, err)
				continue
			}
			_, hasLock := lock.Skills[name]
			if !hasLock {
				fmt.Printf("  %s %s — never installed\n", ui.Warn("new"), name)
				outdated++
				continue
			}
			if head != remote {
				fmt.Printf("  %s %s — %s != %s\n", ui.Warn("behind"), name, head[:8], remote[:8])
				outdated++
			}
		}
		fmt.Printf("\nOutdated: %d\n", outdated)
		if outdated > 0 {
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(outdatedCmd)
}