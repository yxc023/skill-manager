package skillmanager

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// InstallResult describes what Install did for one (skill, target) pair.
type InstallResult struct {
	Skill      string
	Target     string
	Mode       string
	Action     string // "linked" | "copied" | "verified" | "skipped" | "removed-old+linked"
	ResolvedAt string // absolute path on disk
	Hash       string // SHA-256 hex of skill_dir (empty for local)
}

// CacheRoot is the per-user cache directory (~/.skills-manage).
const CacheRoot = "~/.skills-manage"

// ExpandCacheRoot returns the absolute cache root, expanding ~.
func ExpandCacheRoot() (string, error) {
	if strings.HasPrefix(CacheRoot, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, CacheRoot[2:]), nil
	}
	return CacheRoot, nil
}

// InstallOpts configures Install.
type InstallOpts struct {
	CacheRoot string   // absolute path to cache root, e.g. /Users/x/.skills-manage
	Verbose   bool     // print actions as they happen
	HardReset bool     // if true, always re-clone/fetch even if cache exists
}

// Install resolves and materializes one skill to its targets.
//
// For non-local sources:
//  1. ensureFresh() the cache directory
//  2. resolve subpath → skill_dir inside cache
//  3. compute SHA-256 hash of skill_dir
//  4. for each target: expand {category}, append /<name>, apply mode
//
// For local sources:
//  1. use source.Path directly
//  2. compute hash if the path is a directory
//  3. for each target: smart-skip if resolved == source.Path, else apply mode
func Install(name string, sd *SkillDef, targets []Target, opts InstallOpts) ([]InstallResult, error) {
	if !sd.IsEnabled() {
		return nil, nil // silently skip disabled skills
	}

	var skillDir string
	var hash string
	var err error

	if sd.Source.Type == "local" {
		skillDir, err = expandLocalPath(sd.Source.Path)
		if err != nil {
			return nil, fmt.Errorf("skill %q local path: %w", name, err)
		}
		if info, statErr := os.Stat(skillDir); statErr != nil || !info.IsDir() {
			return nil, fmt.Errorf("skill %q local path is not a directory: %s", name, skillDir)
		}
		hash, err = ComputeFolderHash(skillDir)
		if err != nil {
			return nil, fmt.Errorf("skill %q hash: %w", name, err)
		}
	} else {
		// non-local: ensure cache clone is fresh
		fetchURL, ferr := sd.Source.FetchURL()
		if ferr != nil {
			return nil, fmt.Errorf("skill %q fetch url: %w", name, ferr)
		}
		ref := sd.Source.Ref
		if ref == "" {
			ref = "main"
		}
		cacheDir, cerr := sd.Source.CachePath(opts.CacheRoot)
		if cerr != nil {
			return nil, fmt.Errorf("skill %q cache path: %w", name, cerr)
		}
		g := newGitRunner(opts.Verbose)
		if opts.HardReset {
			// Force fresh clone
			_ = os.RemoveAll(cacheDir)
		}
		if err = g.ensureFresh(fetchURL, cacheDir, ref); err != nil {
			return nil, fmt.Errorf("skill %q git fetch: %w", name, err)
		}
		// Resolve subpath
		skillDir = filepath.Join(cacheDir, sd.Source.Subpath)
		if info, statErr := os.Stat(skillDir); statErr != nil || !info.IsDir() {
			return nil, fmt.Errorf("skill %q subpath %q not found in %s", name, sd.Source.Subpath, cacheDir)
		}
		if _, hasSkill := os.Stat(filepath.Join(skillDir, "SKILL.md")); hasSkill != nil {
			return nil, fmt.Errorf("skill %q missing SKILL.md in %s", name, skillDir)
		}
		hash, err = ComputeFolderHash(skillDir)
		if err != nil {
			return nil, fmt.Errorf("skill %q hash: %w", name, err)
		}
	}

	var results []InstallResult
	for _, t := range targets {
		if err := ValidateMode(t.Mode); err != nil {
			return nil, fmt.Errorf("skill %q target %q: %w", name, t.Agent, err)
		}
		dest, err := t.ResolvedPath(sd.Category, name)
		if err != nil {
			return nil, fmt.Errorf("skill %q target %q path: %w", name, t.Agent, err)
		}
		mode := t.effectiveMode()

		// smart-skip: if local source's resolved dir == resolved dest, no-op.
		if sd.Source.Type == "local" {
			absSkill, _ := filepath.Abs(skillDir)
			if absSkill == dest {
				results = append(results, InstallResult{
					Skill: name, Target: t.Agent, Mode: mode, Action: "skipped",
					ResolvedAt: dest, Hash: hash,
				})
				continue
			}
		}

		action, err := applyMode(skillDir, dest, mode, opts.Verbose)
		if err != nil {
			return nil, fmt.Errorf("skill %q target %q apply: %w", name, t.Agent, err)
		}
		results = append(results, InstallResult{
			Skill: name, Target: t.Agent, Mode: mode, Action: action,
			ResolvedAt: dest, Hash: hash,
		})
	}
	return results, nil
}

// applyMode materializes skill_dir → dest using the given mode.
func applyMode(skillDir, dest, mode string, verbose bool) (string, error) {
	// Ensure parent dir exists.
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("mkdir parent: %w", err)
	}

	// If dest exists: check whether it's a symlink we manage, or stale content.
	if info, err := os.Lstat(dest); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			// Existing symlink: remove it and re-apply.
			target, _ := os.Readlink(dest)
			if verbose {
				fmt.Printf("  replacing symlink %s → %s\n", dest, target)
			}
			if err := os.Remove(dest); err != nil {
				return "", fmt.Errorf("remove existing symlink: %w", err)
			}
		} else if mode == ModeSelf {
			// self mode just verifies SKILL.md exists; nothing to do here
			return "verified", nil
		} else {
			// Existing non-symlink file/dir — refuse to overwrite.
			return "", fmt.Errorf("destination already exists (not a symlink): %s", dest)
		}
	}

	switch mode {
	case ModeSymlink:
		if err := os.Symlink(skillDir, dest); err != nil {
			return "", fmt.Errorf("symlink: %w", err)
		}
		if verbose {
			fmt.Printf("  linked %s → %s\n", dest, skillDir)
		}
		return "linked", nil
	case ModeCopy:
		if err := copyDir(skillDir, dest); err != nil {
			return "", fmt.Errorf("copy: %w", err)
		}
		if verbose {
			fmt.Printf("  copied %s ← %s\n", dest, skillDir)
		}
		return "copied", nil
	case ModeSelf:
		// verify SKILL.md exists at dest
		if _, err := os.Stat(filepath.Join(dest, "SKILL.md")); err != nil {
			return "", fmt.Errorf("self mode: SKILL.md not found at %s", dest)
		}
		return "verified", nil
	default:
		return "", fmt.Errorf("unknown mode %q", mode)
	}
}

// copyDir recursively copies src to dst using io.Copy for file contents.
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sf.Close()
	info, err := sf.Stat()
	if err != nil {
		return err
	}
	df, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	if _, err := io.Copy(df, sf); err != nil {
		_ = df.Close()
		return err
	}
	return df.Close()
}

// expandLocalPath expands ~ and returns absolute path.
func expandLocalPath(p string) (string, error) {
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if p == "~" {
			p = home
		} else if strings.HasPrefix(p, "~/") {
			p = filepath.Join(home, p[2:])
		}
	}
	return filepath.Abs(p)
}