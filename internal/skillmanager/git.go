package skillmanager

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// GitRunner shells out to the system `git` binary. Reusing the user's installed
// git keeps SSH keys, GPG signing, and user.email/user.name config intact.
type GitRunner struct {
	verbose bool
}

// NewGitRunner returns a runner. verbose=true prints each command before execution.
func NewGitRunner(verbose bool) *GitRunner {
	return &GitRunner{verbose: verbose}
}

// run executes git with args. dir is the working directory (use "" for default).
// Returns combined stdout+stderr. Returns an error if exit status != 0.
func (g *GitRunner) run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if g.verbose {
		prefix := ""
		if dir != "" {
			prefix = "(cd " + dir + ")"
		}
		fmt.Printf("%s git %s\n", prefix, strings.Join(args, " "))
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// Clone performs: git clone --depth 1 <url> <dir>
// dir must not already exist; if it does, returns an error.
func (g *GitRunner) Clone(url, dir string) error {
	_, err := g.run("", "clone", "--depth", "1", url, dir)
	return err
}

// FetchIn performs: git -C <dir> fetch --depth 1 origin <ref>
func (g *GitRunner) FetchIn(dir, ref string) error {
	_, err := g.run(dir, "fetch", "--depth", "1", "origin", ref)
	return err
}

// ResetHardIn performs: git -C <dir> reset --hard origin/<ref>
func (g *GitRunner) ResetHardIn(dir, ref string) error {
	_, err := g.run(dir, "reset", "--hard", "origin/"+ref)
	return err
}

// ensureFresh clones the repo at url into dir if dir is missing, otherwise
// fetches and hard-resets to ref. Returns nil on success.
func (g *GitRunner) ensureFresh(url, dir, ref string) error {
	if !dirExists(dir) {
		return g.Clone(url, dir)
	}
	if err := g.FetchIn(dir, ref); err != nil {
		// If ref is not in shallow history (e.g. tag), fall back to unshallow fetch.
		if _, unshallowErr := g.run(dir, "fetch", "--unshallow", "origin"); unshallowErr != nil {
			return fmt.Errorf("fetch failed (%v) and unshallow fallback failed (%v)", err, unshallowErr)
		}
	}
	return g.ResetHardIn(dir, ref)
}

// RevParseHead returns the commit SHA at HEAD.
func (g *GitRunner) RevParseHead(dir string) (string, error) {
	out, err := g.run(dir, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// RevParseRemoteRef returns the commit SHA at origin/<ref>.
func (g *GitRunner) RevParseRemoteRef(dir, ref string) (string, error) {
	out, err := g.run(dir, "rev-parse", "origin/"+ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func dirExists(p string) bool {
	// kept inline to avoid pulling os import into a separate util file
	info, err := statDir(p)
	return err == nil && info.IsDir()
}