package skillmanager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeSampleSkillDir creates a directory with SKILL.md for use in install tests.
func makeSampleSkillDir(t *testing.T, root string) string {
	t.Helper()
	dir := filepath.Join(root, "fake-skill")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Test\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.py"), []byte("print('hi')\n"), 0o644))
	return dir
}

func TestInstall_LocalSourceSymlink(t *testing.T) {
	root := t.TempDir()
	skillDir := makeSampleSkillDir(t, root)

	targets := []Target{{Agent: "test", Path: filepath.Join(root, "dest/{category}"), Mode: ModeSymlink}}
	sd := &SkillDef{
		Category: "x",
		Source:   Source{Type: "local", Path: skillDir},
	}

	results, err := Install("fake-skill", sd, targets, InstallOpts{CacheRoot: filepath.Join(root, "cache")})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "linked", results[0].Action)
	assert.Equal(t, ModeSymlink, results[0].Mode)

	dest := filepath.Join(root, "dest", "x", "fake-skill")
	info, err := os.Lstat(dest)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeSymlink, "dest should be a symlink")
}

func TestInstall_LocalSourceCopy(t *testing.T) {
	root := t.TempDir()
	skillDir := makeSampleSkillDir(t, root)

	targets := []Target{{Agent: "test", Path: filepath.Join(root, "dest/{category}"), Mode: ModeCopy}}
	sd := &SkillDef{
		Category: "x",
		Source:   Source{Type: "local", Path: skillDir},
	}

	results, err := Install("fake-skill", sd, targets, InstallOpts{CacheRoot: filepath.Join(root, "cache")})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "copied", results[0].Action)

	dest := filepath.Join(root, "dest", "x", "fake-skill")
	info, err := os.Lstat(dest)
	require.NoError(t, err)
	assert.Zero(t, info.Mode()&os.ModeSymlink, "copy mode should NOT be a symlink")
	assert.True(t, info.IsDir())
	_, err = os.Stat(filepath.Join(dest, "SKILL.md"))
	assert.NoError(t, err)
}

func TestInstall_LocalSourceSelfMode(t *testing.T) {
	root := t.TempDir()
	// For self mode: target.Path is the parent; ResolvedPath appends /<name>.
	// After append, dest should already contain SKILL.md.
	skillDir := filepath.Join(root, "fake-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test\n"), 0o644))

	targets := []Target{{Agent: "test", Path: root, Mode: ModeSelf}}
	sd := &SkillDef{
		Source: Source{Type: "local", Path: skillDir},
	}

	results, err := Install("fake-skill", sd, targets, InstallOpts{CacheRoot: filepath.Join(root, "cache")})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "verified", results[0].Action)
}

func TestInstall_SmartSkipWhenTargetEqualsSource(t *testing.T) {
	root := t.TempDir()
	skillDir := makeSampleSkillDir(t, root)

	// target.Path = root (parent of skillDir). After ResolvedPath(category="", name="fake-skill")
	// we get root/fake-skill = skillDir → smart-skip fires.
	targets := []Target{{Agent: "test", Path: root, Mode: ModeSymlink}}
	sd := &SkillDef{
		Source: Source{Type: "local", Path: skillDir},
	}

	results, err := Install("fake-skill", sd, targets, InstallOpts{CacheRoot: filepath.Join(root, "cache")})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "skipped", results[0].Action)
}

func TestInstall_DisabledSkillSkipped(t *testing.T) {
	root := t.TempDir()
	skillDir := makeSampleSkillDir(t, root)

	targets := []Target{{Agent: "test", Path: filepath.Join(root, "dest/{category}"), Mode: ModeSymlink}}
	d := false
	sd := &SkillDef{
		Category: "x",
		Enabled:  &d,
		Source:   Source{Type: "local", Path: skillDir},
	}

	results, err := Install("fake-skill", sd, targets, InstallOpts{CacheRoot: filepath.Join(root, "cache")})
	require.NoError(t, err)
	assert.Empty(t, results, "disabled skill should produce no install results")
}

func TestInstall_DestinationAlreadyExistsFails(t *testing.T) {
	root := t.TempDir()
	skillDir := makeSampleSkillDir(t, root)

	dest := filepath.Join(root, "dest", "x", "fake-skill")
	require.NoError(t, os.MkdirAll(filepath.Dir(dest), 0o755))
	require.NoError(t, os.WriteFile(dest, []byte("blocker"), 0o644))

	targets := []Target{{Agent: "test", Path: filepath.Join(root, "dest/{category}"), Mode: ModeSymlink}}
	sd := &SkillDef{
		Category: "x",
		Source:   Source{Type: "local", Path: skillDir},
	}

	_, err := Install("fake-skill", sd, targets, InstallOpts{CacheRoot: filepath.Join(root, "cache")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "destination already exists")
}

func TestInstall_MissingSkillMDInLocal(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "no-skill-md")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "readme.txt"), []byte("x"), 0o644))

	targets := []Target{{Agent: "test", Path: filepath.Join(root, "dest/{category}"), Mode: ModeSymlink}}
	sd := &SkillDef{
		Category: "x",
		Source:   Source{Type: "local", Path: skillDir},
	}

	_, err := Install("fake-skill", sd, targets, InstallOpts{CacheRoot: filepath.Join(root, "cache")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SKILL.md")
}

func TestTarget_ResolvedPath(t *testing.T) {
	t.Run("expands {category}", func(t *testing.T) {
		target := &Target{Agent: "x", Path: "/p/{category}", Mode: ModeSymlink}
		got, err := target.ResolvedPath("tools/bar", "foo")
		require.NoError(t, err)
		assert.Equal(t, "/p/tools/bar/foo", got)
	})

	t.Run("relative-path-becomes-abs", func(t *testing.T) {
		target := &Target{Agent: "x", Path: "rel/{category}", Mode: ModeSymlink}
		got, err := target.ResolvedPath("c", "n")
		require.NoError(t, err)
		assert.True(t, filepath.IsAbs(got), "expected absolute path, got %s", got)
	})
}