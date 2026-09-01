package skillmanager

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// ComputeFolderHash walks dir and returns a deterministic SHA-256 hex digest of
// its contents. The hash covers every file's relative path, mode bits, and bytes.
// It is sensitive to: content changes, filename changes, mode changes.
// It is insensitive to: directory mtime, traversal order (paths are sorted).
//
// dir must be a directory. Empty directories hash to SHA-256 of the empty string.
func ComputeFolderHash(dir string) (string, error) {
	h := sha256.New()

	// Collect paths first so we can sort them — gives a stable hash regardless
	// of filesystem walk order.
	var paths []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk %s: %w", dir, err)
	}
	sort.Strings(paths)

	for _, rel := range paths {
		full := filepath.Join(dir, rel)
		info, err := os.Lstat(full)
		if err != nil {
			return "", fmt.Errorf("lstat %s: %w", full, err)
		}
		// Include relative path (with forward-slash separator for cross-platform stability).
		h.Write([]byte(filepath.ToSlash(rel)))
		h.Write([]byte{0})
		// Include mode as string so permission/exitency changes affect the hash.
		h.Write([]byte(info.Mode().String()))
		h.Write([]byte{0})
		// Include file contents.
		f, err := os.Open(full)
		if err != nil {
			return "", fmt.Errorf("open %s: %w", full, err)
		}
		if _, err := io.Copy(h, f); err != nil {
			_ = f.Close()
			return "", fmt.Errorf("read %s: %w", full, err)
		}
		_ = f.Close()
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
