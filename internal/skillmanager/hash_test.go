package skillmanager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeFolderHash_Deterministic(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("alpha"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("beta"), 0o644))

	h1, err := ComputeFolderHash(dir)
	require.NoError(t, err)
	h2, err := ComputeFolderHash(dir)
	require.NoError(t, err)

	assert.Equal(t, h1, h2, "hash must be deterministic across calls")
	assert.Len(t, h1, 64, "SHA-256 hex is 64 chars")
}

func TestComputeFolderHash_ContentChange(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("v1"), 0o644))

	before, _ := ComputeFolderHash(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("v2-different"), 0o644))
	after, _ := ComputeFolderHash(dir)

	assert.NotEqual(t, before, after, "content change must change hash")
}

func TestComputeFolderHash_FilenameChange(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "old.txt"), []byte("x"), 0o644))

	before, _ := ComputeFolderHash(dir)
	require.NoError(t, os.Rename(filepath.Join(dir, "old.txt"), filepath.Join(dir, "new.txt")))
	after, _ := ComputeFolderHash(dir)

	assert.NotEqual(t, before, after, "filename change must change hash")
}

func TestComputeFolderHash_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	h, err := ComputeFolderHash(dir)
	require.NoError(t, err)
	assert.Len(t, h, 64, "empty dir still produces a 64-char hex digest")
	// The hash of no bytes is fixed: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
	assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", h)
}

func TestComputeFolderHash_DeterministicRegardlessOfOrder(t *testing.T) {
	// Two trees with same content but created in different orders should yield same hash.
	a := t.TempDir()
	b := t.TempDir()
	for _, p := range []struct{ rel, body string }{
		{"a.txt", "alpha"},
		{"b.txt", "bravo"},
		{"c.txt", "charlie"},
		{"sub/delta.txt", "delta"},
		{"sub/echo.txt", "echo"},
	} {
		require.NoError(t, os.MkdirAll(filepath.Join(a, filepath.Dir(p.rel)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(a, p.rel), []byte(p.body), 0o644))
	}
	// Create in reverse
	for i := len([]struct{ rel, body string }{
		{"a.txt", "alpha"},
		{"b.txt", "bravo"},
		{"c.txt", "charlie"},
		{"sub/delta.txt", "delta"},
		{"sub/echo.txt", "echo"},
	}) - 1; i >= 0; i-- {
		// (index not used; rebuilding for clarity)
		_ = i
		break
	}
	type pair struct{ rel, body string }
	files := []pair{{"a.txt", "alpha"}, {"b.txt", "bravo"}, {"c.txt", "charlie"}, {"sub/delta.txt", "delta"}, {"sub/echo.txt", "echo"}}
	for i := len(files) - 1; i >= 0; i-- {
		p := files[i]
		require.NoError(t, os.MkdirAll(filepath.Join(b, filepath.Dir(p.rel)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(b, p.rel), []byte(p.body), 0o644))
	}

	hashA, _ := ComputeFolderHash(a)
	hashB, _ := ComputeFolderHash(b)
	assert.Equal(t, hashA, hashB, "hash should not depend on creation order")
}