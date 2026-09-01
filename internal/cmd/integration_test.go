package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runCmdInDir executes the binary in-process with --manifest pointing to <root>/skills-manage.json.
// We avoid t.Chdir (Go 1.24+) so tests work with go 1.21+. Uses absolute manifest paths via --manifest flag.
func runCmdInDir(t *testing.T, dir string, args ...string) {
	t.Helper()
	rootCmd.SetArgs(append([]string{"--manifest", filepath.Join(dir, "skills-manage.json")}, args...))
	if err := Execute(); err != nil {
		t.Fatalf("execute %v: %v", args, err)
	}
}

func TestIntegration_InitCreatesManifest(t *testing.T) {
	dir := t.TempDir()
	runCmdInDir(t, dir, "init")

	data, err := os.ReadFile(filepath.Join(dir, "skills-manage.json"))
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	assert.EqualValues(t, 2, m["version"])
	assert.Contains(t, m, "skills")
	assert.Contains(t, m, "targets")
}

func TestIntegration_ValidateOnEmptyManifest(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "skills-manage.json")
	require.NoError(t, os.WriteFile(manifest, []byte(`{
		"version": 2,
		"targets": [{"agent": "x", "path": "/p/{category}"}],
		"skills": {}
	}`), 0o644))

	runCmdInDir(t, dir, "validate")
}

func TestIntegration_LockInitializesIfMissing(t *testing.T) {
	dir := t.TempDir()
	runCmdInDir(t, dir, "init")

	// lock should not crash when no lock file exists yet — that's the whole point.
	// We don't create a file on read; this verifies graceful handling.
	runCmdInDir(t, dir, "lock")

	_, err := os.Stat(filepath.Join(dir, "skills-manage.lock.json"))
	assert.True(t, os.IsNotExist(err), "lock cmd should not create a file when only reading")
}

func TestIntegration_ListPrintsHeaders(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "skills-manage.json")
	require.NoError(t, os.WriteFile(manifest, []byte(`{
		"version": 2,
		"targets": [{"agent": "x", "path": "/p/{category}"}],
		"skills": {"foo": {"source": {"type": "local", "path": "/tmp/some-skill"}}}
	}`), 0o644))

	// just ensure list doesn't crash with a real skill entry
	runCmdInDir(t, dir, "list")
}
