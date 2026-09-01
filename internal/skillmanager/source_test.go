package skillmanager

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGitURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantHost string
		wantOwn  string
		wantRepo string
		wantErr  bool
	}{
		{
			name:     "https-basic",
			url:      "https://github.com/anthropics/skills.git",
			wantHost: "github.com", wantOwn: "anthropics", wantRepo: "skills",
		},
		{
			name:     "https-no-dot-git",
			url:      "https://gitlab.com/team/main",
			wantHost: "gitlab.com", wantOwn: "team", wantRepo: "main",
		},
		{
			name:     "https-nested-owner",
			url:      "https://gitlab.com/group/sub/skills.git",
			wantHost: "gitlab.com", wantOwn: "group/sub", wantRepo: "skills",
		},
		{
			name:     "https-with-port-not-scp",
			url:      "https://github.com:443/anthropics/skills.git",
			wantHost: "github.com", wantOwn: "anthropics", wantRepo: "skills",
		},
		{
			name:     "ssh-protocol",
			url:      "ssh://git@gitlab.inner.com/team/skills.git",
			wantHost: "gitlab.inner.com", wantOwn: "team", wantRepo: "skills",
		},
		{
			name:     "scp-github",
			url:      "git@github.com:anthropics/skills.git",
			wantHost: "github.com", wantOwn: "anthropics", wantRepo: "skills",
		},
		{
			name:     "scp-self-hosted",
			url:      "git@gitlab.inner.com:team/skills.git",
			wantHost: "gitlab.inner.com", wantOwn: "team", wantRepo: "skills",
		},
		{
			name:    "empty-url",
			url:     "",
			wantErr: true,
		},
		{
			name:    "url-with-no-path",
			url:     "https://github.com/",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, own, repo, err := ParseGitURL(tt.url)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantHost, host, "host")
			assert.Equal(t, tt.wantOwn, own, "owner")
			assert.Equal(t, tt.wantRepo, repo, "repo")
		})
	}
}

func TestSource_FetchURL(t *testing.T) {
	tests := []struct {
		name   string
		source Source
		want   string
		wantErr bool
	}{
		{
			name:   "github-default-host",
			source: Source{Type: "github", Repo: "owner/name"},
			want:   "https://github.com/owner/name.git",
		},
		{
			name:   "github-custom-host",
			source: Source{Type: "github", Repo: "o/r", Host: "gh.example.com"},
			want:   "https://gh.example.com/o/r.git",
		},
		{
			name:   "gitlab-default-host",
			source: Source{Type: "gitlab", Repo: "team/skills"},
			want:   "https://gitlab.com/team/skills.git",
		},
		{
			name:   "gitlab-custom-host",
			source: Source{Type: "gitlab", Repo: "team/skills", Host: "gitlab.inner.com"},
			want:   "https://gitlab.inner.com/team/skills.git",
		},
		{
			name:   "git-passthrough",
			source: Source{Type: "git", URL: "git@github.com:o/r.git"},
			want:   "git@github.com:o/r.git",
		},
		{
			name:   "local-empty",
			source: Source{Type: "local", Path: "~/work/x"},
			want:   "",
		},
		{
			name:    "unknown-type",
			source:  Source{Type: "ftp"},
			wantErr: true,
		},
		{
			name:    "git-empty-url",
			source:  Source{Type: "git"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.source.FetchURL()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSource_CachePath(t *testing.T) {
	tests := []struct {
		name   string
		source Source
		want   string
		wantErr bool
	}{
		{
			name:   "github-default",
			source: Source{Type: "github", Repo: "o/r"},
			want:   "/root/github.com/o/r",
		},
		{
			name:   "gitlab-custom-host",
			source: Source{Type: "gitlab", Repo: "team/skills", Host: "gitlab.inner.com"},
			want:   "/root/gitlab.inner.com/team/skills",
		},
		{
			name:   "git-from-scp",
			source: Source{Type: "git", URL: "git@github.com:o/r.git"},
			want:   "/root/github.com/o/r",
		},
		{
			name:    "local-error",
			source:  Source{Type: "local", Path: "~/x"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.source.CachePath("/root")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}