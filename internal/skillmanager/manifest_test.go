package skillmanager

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifest_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skills-manage.json")

	original := &Manifest{
		Version: ManifestVersion,
		Targets: []Target{{Agent: "opencode", Path: ".opencode/skills/{category}", Mode: ModeSymlink}},
		Skills: map[string]*SkillDef{
			"foo": {
				Category: "tools/foo",
				Source:   Source{Type: "github", Repo: "o/r"},
			},
		},
	}
	require.NoError(t, SaveManifest(original, path))

	// Inspect raw bytes to ensure camelCase keys
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"version"`)
	assert.Contains(t, string(raw), `"targets"`)
	assert.Contains(t, string(raw), `"skills"`)
	assert.Contains(t, string(raw), `"category"`)
	assert.Contains(t, string(raw), `"source"`)

	loaded, err := LoadManifest(path)
	require.NoError(t, err)
	assert.Equal(t, ManifestVersion, loaded.Version)
	assert.Equal(t, "opencode", loaded.Targets[0].Agent)
	assert.True(t, loaded.Skills["foo"].IsEnabled())
}

func TestManifest_LoadRejectsBadVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version":1,"targets":[],"skills":{}}`), 0o644))

	_, err := LoadManifest(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version")
}

func TestManifest_EnabledDefault(t *testing.T) {
	sd := &SkillDef{Source: Source{Type: "github", Repo: "o/r"}}
	assert.True(t, sd.IsEnabled(), "missing enabled should default to true")

	d := new(bool)
	*d = false
	sd.Enabled = d
	assert.False(t, sd.IsEnabled(), "explicit false should be honored")
}

func TestManifest_EffectiveTargets(t *testing.T) {
	sd := &SkillDef{Source: Source{Type: "github", Repo: "o/r"}}
	defaults := []Target{{Agent: "default", Path: "/d/{category}"}}
	assert.Equal(t, defaults, sd.EffectiveTargets(defaults), "no per-skill targets → use default")

	sd.Targets = []Target{{Agent: "override", Path: "/o/{category}"}}
	assert.Equal(t, "override", sd.EffectiveTargets(defaults)[0].Agent, "per-skill overrides default")
}

func TestLock_MissingFileIsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skills-manage.lock.json")

	lock, err := LoadLock(path)
	require.NoError(t, err)
	assert.Equal(t, ManifestVersion, lock.Version)
	assert.NotNil(t, lock.Skills)
	assert.Empty(t, lock.Skills)
}

func TestLock_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lock.json")

	original := &Lock{
		Version: ManifestVersion,
		Skills: map[string]*LockEntry{
			"foo": {
				Source:          Source{Type: "github", Repo: "o/r"},
				Category:        "tools/foo",
				SkillFolderHash: "abc123",
			},
		},
	}
	require.NoError(t, SaveLock(original, path))
	loaded, err := LoadLock(path)
	require.NoError(t, err)
	assert.Equal(t, "abc123", loaded.Skills["foo"].SkillFolderHash)
	assert.Equal(t, "tools/foo", loaded.Skills["foo"].Category)
}

func TestLock_JSONFormat(t *testing.T) {
	// Ensure camelCase keys for skillFolderHash
	data, err := json.MarshalIndent(&Lock{
		Version: ManifestVersion,
		Skills:  map[string]*LockEntry{"k": {SkillFolderHash: "h"}},
	}, "", "  ")
	require.NoError(t, err)
	assert.Contains(t, string(data), `"skillFolderHash": "h"`)
}