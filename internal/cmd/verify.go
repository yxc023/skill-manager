package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/yxc023/skill-manager/internal/skillmanager"
	"github.com/yxc023/skill-manager/internal/ui"
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify each skill's SKILL.md, source hash (if available), and target existence",
	RunE: func(cmd *cobra.Command, args []string) error {
		manifestPath, _ := cmd.Flags().GetString("manifest")
		m, err := skillmanager.LoadManifest(manifestPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, ui.Fail(err.Error()))
			os.Exit(1)
		}

		lock, err := skillmanager.LoadLock(manifestPath + ".lock.json")
		if err != nil {
			return fmt.Errorf("read lock: %w", err)
		}

		cacheRoot, err := skillmanager.ExpandCacheRoot()
		if err != nil {
			return err
		}

		ok := 0
		mismatch := 0
		missing := 0

		for name, sd := range m.Skills {
			if !sd.IsEnabled() {
				continue
			}
			fmt.Printf("→ %s\n", ui.Name(name))
			targets := sd.EffectiveTargets(m.Targets)

			// 1) Resolve skill_dir and compute current hash
			var skillDir string
			if sd.Source.Type == "local" {
				p, err := expandPath(sd.Source.Path)
				if err != nil {
					fmt.Fprintln(os.Stderr, ui.Fail(err.Error()))
					missing++
					continue
				}
				if info, err := os.Stat(p); err != nil || !info.IsDir() {
					fmt.Fprintln(os.Stderr, ui.Fail(fmt.Sprintf("local path missing: %s", p)))
					missing++
					continue
				}
				skillDir = p
			} else {
				cacheDir, err := sd.Source.CachePath(cacheRoot)
				if err != nil {
					fmt.Fprintln(os.Stderr, ui.Fail(err.Error()))
					missing++
					continue
				}
				skillDir = joinSubpath(cacheDir, sd.Source.Subpath)
				if info, err := os.Stat(skillDir); err != nil || !info.IsDir() {
					fmt.Fprintln(os.Stderr, ui.Fail(fmt.Sprintf("cache missing (run install first): %s", skillDir)))
					missing++
					continue
				}
			}

			// 2) SKILL.md present?
			if _, err := os.Stat(fmt.Sprintf("%s/SKILL.md", skillDir)); err != nil {
				fmt.Fprintln(os.Stderr, ui.Fail("SKILL.md missing"))
				missing++
				continue
			}

			// 3) Hash check (only for non-local; local sources don't lock by hash)
			currentHash, _ := skillmanager.ComputeFolderHash(skillDir)
			if lockEntry, hasLock := lock.Skills[name]; hasLock && lockEntry.SkillFolderHash != "" {
				if lockEntry.SkillFolderHash != currentHash {
					fmt.Fprintln(os.Stderr, ui.Fail(fmt.Sprintf("hash mismatch: lock=%s current=%s", lockEntry.SkillFolderHash[:8], currentHash[:8])))
					mismatch++
					continue
				}
				fmt.Printf("  %s hash=%s\n", ui.OK("match"), currentHash[:8])
			}

			// 5) Target existence
			for _, t := range targets {
				dest, err := t.ResolvedPath(sd.Category, name)
				if err != nil {
					fmt.Fprintln(os.Stderr, ui.Fail(err.Error()))
					missing++
					continue
				}
				if _, err := os.Lstat(dest); err != nil {
					fmt.Fprintln(os.Stderr, ui.Fail(fmt.Sprintf("target %s missing: %s", t.Agent, dest)))
					missing++
					continue
				}
				fmt.Printf("  %s target %s at %s\n", ui.OK("present"), t.Agent, dest)
			}
			ok++
		}

		fmt.Printf("\nOK: %d, Hash mismatch: %d, Missing: %d\n", ok, mismatch, missing)
		if mismatch > 0 || missing > 0 {
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(verifyCmd)
}

// expandPath is a thin wrapper around filepath.Abs for test seams.
func expandPath(p string) (string, error) {
	if len(p) >= 2 && p[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return home + "/" + p[2:], nil
	}
	if p == "~" {
		return os.UserHomeDir()
	}
	return abs(p)
}

func joinSubpath(parent, sub string) string {
	if sub == "" {
		return parent
	}
	return parent + "/" + sub
}

func abs(p string) (string, error) {
	if isAbs := len(p) > 0 && p[0] == '/'; isAbs {
		return p, nil
	}
	return os.Getwd() // best-effort fallback; manifest paths should already be absolute
}