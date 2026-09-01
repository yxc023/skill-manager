// Package skillmanager is the core domain for skill-manager: parsing git URLs,
// loading/saving manifests, computing folder hashes, and installing skills.
package skillmanager

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Source describes where a skill is fetched from.
//
//   - Type=github: Repo=owner/name, Host optional (default github.com), Ref/Subpath optional.
//   - Type=gitlab: same as github but default Host=gitlab.com.
//   - Type=git:    URL=raw git URL (https or ssh form).
//   - Type=local:  Path=local filesystem path.
type Source struct {
	Type    string `json:"type"`
	Repo    string `json:"repo,omitempty"`
	Host    string `json:"host,omitempty"`
	URL     string `json:"url,omitempty"`
	Path    string `json:"path,omitempty"`
	Ref     string `json:"ref,omitempty"`
	Subpath string `json:"subpath,omitempty"`
}

// scpRegex matches git@host:owner/repo (SCP-style). The user@ prefix is required
// so https URLs with port (e.g. https://github.com:443/x/y) are not mistaken for SCP.
var scpRegex = regexp.MustCompile(`^([^@]+)@([^:]+):(.+)$`)

// ParseGitURL splits a git URL into (host, owner, repo).
//
// Supports:
//   - https://host/owner/repo[.git]
//   - ssh://[user@]host/owner/repo[.git]
//   - user@host:owner/repo[.git] (SCP-style)
//
// For nested paths (e.g. gitlab subgroups), owner is "group/sub".
func ParseGitURL(rawURL string) (host, owner, repo string, err error) {
	if m := scpRegex.FindStringSubmatch(rawURL); m != nil {
		host = m[2]
		return splitPath(host, m[3], rawURL)
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid git URL %q: %w", rawURL, err)
	}
	if u.Host == "" {
		return "", "", "", fmt.Errorf("invalid git URL %q (no host)", rawURL)
	}
	// Use Hostname (no port) so that https://host:443/path still extracts host correctly.
	return splitPath(u.Hostname(), strings.TrimPrefix(u.Path, "/"), rawURL)
}

func splitPath(host, path, rawURL string) (string, string, string, error) {
	path = strings.TrimSuffix(path, ".git")
	path = strings.Trim(path, "/")
	if path == "" {
		return "", "", "", fmt.Errorf("invalid git URL %q (empty path)", rawURL)
	}
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("invalid git URL %q (path has no owner)", rawURL)
	}
	repo := parts[len(parts)-1]
	owner := strings.Join(parts[:len(parts)-1], "/")
	return host, owner, repo, nil
}

// FetchURL returns the canonical clone URL for the source.
//
//   - github/gitlab: derived from Repo + Host (defaults to github.com/gitlab.com).
//   - git:           returns URL as-is (caller may pass any form).
//   - local:         returns empty string (no clone needed).
func (s *Source) FetchURL() (string, error) {
	switch s.Type {
	case "github":
		host := s.Host
		if host == "" {
			host = "github.com"
		}
		return fmt.Sprintf("https://%s/%s.git", host, s.Repo), nil
	case "gitlab":
		host := s.Host
		if host == "" {
			host = "gitlab.com"
		}
		return fmt.Sprintf("https://%s/%s.git", host, s.Repo), nil
	case "git":
		if s.URL == "" {
			return "", fmt.Errorf("git source has empty url")
		}
		return s.URL, nil
	case "local":
		return "", nil
	default:
		return "", fmt.Errorf("unknown source type %q", s.Type)
	}
}

// CachePath returns the on-disk cache directory for non-local sources.
// Format: <root>/<host>/<owner>/<repo>
func (s *Source) CachePath(root string) (string, error) {
	if s.Type == "local" {
		return "", fmt.Errorf("local source has no cache path")
	}
	host, owner, repo, err := s.Parsed()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s/%s/%s", root, host, owner, repo), nil
}

// Parsed returns (host, owner, repo) for non-local sources by calling ParseGitURL
// on the resolved fetch URL. Returns an error for local sources.
func (s *Source) Parsed() (string, string, string, error) {
	fetchURL, err := s.FetchURL()
	if err != nil {
		return "", "", "", err
	}
	return ParseGitURL(fetchURL)
}
