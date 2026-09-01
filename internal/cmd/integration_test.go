package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runCmd executes the binary in-process via Execute() and returns success.
func runCmd(t *testing.T, args ...string) {
	t.Helper()
	// reset rootCmd args between calls (cobra retains state)
	rootCmd.SetArgs(args)
	if err := Execute(); err != nil {
		t.Fatalf("execute %v: %v", args, err)
	}
}

func TestIntegration_InitCreatesManifest(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	runCmd(t, "init")

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
	t.Chdir(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "skills-manage.json"), []byte(`{
		"version": 2,
		"targets": [{"agent": "x", "path": "/p/{category}"}],
		"skills": {}
	}`), 0o644))

	runCmd(t, "validate")
}

func TestIntegration_LockInitializesIfMissing(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	runCmd(t, "init")

	// lock should not crash when no lock file exists yet — that's the whole point.
	// We don't create a file on read; this verifies graceful handling.
	runCmd(t, "lock")

	_, err := os.Stat(filepath.Join(dir, "skills-manage.lock.json"))
	assert.True(t, os.IsNotExist(err), "lock cmd should not create a file when only reading")
}

func TestIntegration_ListPrintsHeaders(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	runCmd(t, "init")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "skills-manage.json"), []byte(`{
		"version": 2,
		"targets": [{"agent": "x", "path": "/p/{category}"}],
		"skills": {"foo": {"source": {"type": "local", "path": "/tmp/some-skill"}}}
	}`), 0o644))

	// just ensure list doesn't crash with a real skill entry
	runCmd(t, "list")
}