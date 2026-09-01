package skillmanager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Mode constants for how a skill is materialized at a target path.
const (
	ModeSymlink = "symlink" // default
	ModeCopy    = "copy"
	ModeSelf    = "self"
)

// Target describes where a skill should be installed and how.
type Target struct {
	Agent string `json:"agent"`
	Path  string `json:"path"`
	Mode  string `json:"mode,omitempty"` // default symlink
}

// effectiveMode returns Mode, falling back to ModeSymlink.
func (t *Target) effectiveMode() string {
	if t.Mode == "" {
		return ModeSymlink
	}
	return t.Mode
}

// ValidateMode returns an error if mode is not one of the known constants.
// Returns nil for empty mode (which defaults to symlink).
func ValidateMode(mode string) error {
	switch mode {
	case "", ModeSymlink, ModeCopy, ModeSelf:
		return nil
	default:
		return fmt.Errorf("invalid mode %q (must be symlink|copy|self)", mode)
	}
}

// ResolvedPath expands {category} in t.Path, appends /<name>, and resolves
// any ~ to the user's home directory. Result is absolute.
func (t *Target) ResolvedPath(category, name string) (string, error) {
	p := strings.ReplaceAll(t.Path, "{category}", category)
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand ~: %w", err)
		}
		if p == "~" {
			p = home
		} else if strings.HasPrefix(p, "~/") {
			p = filepath.Join(home, p[2:])
		}
	}
	if !filepath.IsAbs(p) {
		abs, err := filepath.Abs(p)
		if err != nil {
			return "", fmt.Errorf("resolve path %q: %w", p, err)
		}
		p = abs
	}
	return filepath.Join(p, name), nil
}
