# skill-manager Go rewrite — Design

> **For Claude:** Validated design from brainstorming session 2026-08-13.

## Goal

Replace Python `sm` (v0.2.2) with a Go rewrite of `skill-manager` v0.3.0. Full feature parity: all 9 subcommands, same `skills-manage.json` v2 manifest format, same lockfile, same on-disk cache layout (renamed from `~/.sm/cache/` to `~/.skills-manage/`).

## Decisions

| Topic | Decision |
|-------|----------|
| Scope | Full parity with Python v0.2.2 (all 9 subcommands) |
| Initial version | v0.3.0 |
| Binary name | `skill-manager` (not `sm`) |
| Module path | `github.com/yxc023/skill-manager` |
| CLI framework | Cobra |
| Git operations | Shell out to `git` CLI (no go-git) |
| Cache path | `~/.skills-manage/` (was `~/.sm/cache/`) — breaking change, no migration |
| Distribution | `go install github.com/yxc023/skill-manager@vX.Y.Z` only — no GitHub Releases binaries |
| Test framework | stdlib `testing` + `testify/assert` |
| Linter | `golangci-lint` (errcheck, govet, ineffassign, staticcheck, unused, gofmt, goimports) |
| CI | GitHub Actions matrix Go 1.21/1.22/1.23 × ubuntu/macos |
| License | MIT |
| Branch | Stay on `yxc023/publish-v0.2.3`, delete Python source in-place |

## Project Structure

```
skill-manager/
├── cmd/
│   └── skill-manager/
│       └── main.go              # entry point + version var
├── internal/
│   ├── skillmanager/            # core domain
│   │   ├── manifest.go          # load/save skills-manage.json (v2 schema)
│   │   ├── lock.go              # load/save skills-manage.lock.json
│   │   ├── source.go            # SkillSource, ParseGitURL, fetch_url()
│   │   ├── target.go            # Target, mode (symlink/copy/self)
│   │   ├── skill.go             # SkillDef, effective_targets
│   │   ├── git.go               # shell out to git (clone, fetch, reset)
│   │   ├── hash.go              # ComputeFolderHash (SHA-256)
│   │   ├── install.go           # install/link/copy/self logic
│   │   ├── verify.go            # hash + SKILL.md verification
│   │   └── valid.go             # validate manifest schema
│   ├── cmd/                     # cobra subcommands
│   │   ├── root.go              # root cmd, --version, --verbose
│   │   ├── init.go              # create starter manifest
│   │   ├── install.go
│   │   ├── validate.go
│   │   ├── verify.go
│   │   ├── list.go
│   │   ├── outdated.go
│   │   ├── update.go
│   │   ├── clean.go
│   │   └── lock.go
│   └── ui/                      # user-facing output (stdout/stderr formatting)
│       └── ui.go
├── testdata/                    # fixtures (sample manifest, sample skills)
├── go.mod
├── go.sum
├── Makefile                     # make test, make lint, make build
├── .golangci.yml
├── .github/workflows/ci.yml
├── README.md
├── LICENSE (MIT)
└── CHANGELOG.md
```

## Cobra Command Tree

```
skill-manager --version          # print 0.3.0
skill-manager --help
skill-manager init               # create starter skills-manage.json
skill-manager install            # clone/fetch + link/copy/self
skill-manager validate           # schema + reference check
skill-manager verify             # hash + SKILL.md + target check
skill-manager list               # configured skills + lock hashes
skill-manager outdated           # upstream commits vs lock
skill-manager update             # alias for install (hard reset)
skill-manager clean              # remove sm-managed symlinks (cache preserved)
skill-manager lock               # print lock file
```

Global flags: `--manifest` (default `skills-manage.json`), `--verbose` (print shell commands).

## Core Types

```go
// Source — github | gitlab | git | local
type Source struct {
    Type    string `json:"type"`              // "github" | "gitlab" | "git" | "local"
    Repo    string `json:"repo,omitempty"`    // owner/name for github/gitlab
    Host    string `json:"host,omitempty"`    // gitlab host
    URL     string `json:"url,omitempty"`     // raw git URL
    Path    string `json:"path,omitempty"`    // local source path
    Ref     string `json:"ref,omitempty"`     // default "main"
    Subpath string `json:"subpath,omitempty"` // skills/foo
}

// Target — agent + path + mode
type Target struct {
    Agent string `json:"agent"`
    Path  string `json:"path"`
    Mode  string `json:"mode,omitempty"` // default "symlink"
}

// SkillDef — one skill definition
type SkillDef struct {
    Category    string   `json:"category,omitempty"`
    Description string   `json:"description,omitempty"`
    Enabled     bool     `json:"enabled"` // default true
    Source      Source   `json:"source"`
    Targets     []Target `json:"targets,omitempty"`
}

// Manifest — top-level skills-manage.json (v2)
type Manifest struct {
    Version int                  `json:"version"` // 2
    Targets []Target             `json:"targets"`
    Skills  map[string]*SkillDef `json:"skills"`
}

// LockEntry — one skill's lock
type LockEntry struct {
    Source          Source `json:"source"`
    Category        string `json:"category"`
    SkillFolderHash string `json:"skillFolderHash"`
}

// Lock — top-level skills-manage.lock.json
type Lock struct {
    Version int                   `json:"version"`
    Skills  map[string]*LockEntry `json:"skills"`
}
```

JSON fields are **camelCase** to maintain on-disk compatibility with Python v2 manifest.

## On-disk Format (parity with Python v0.2.2)

`skills-manage.json` (v2):

```json
{
  "version": 2,
  "targets": [
    {"agent": "opencode", "path": ".opencode/skills/{category}", "mode": "symlink"}
  ],
  "skills": {
    "foo": {
      "source": {"type": "github", "repo": "anthropics/skills"},
      "category": "tools/foo",
      "enabled": true,
      "targets": []
    }
  }
}
```

`skills-manage.lock.json`:

```json
{
  "version": 2,
  "skills": {
    "foo": {
      "source": {"type": "github", "repo": "anthropics/skills"},
      "category": "tools/foo",
      "skillFolderHash": "<sha256-hex-64-chars>"
    }
  }
}
```

Cache: `~/.skills-manage/<host>/<owner>/<repo>/` (shallow clones).

## Physical Flow (per skill)

```
github/gitlab/git:
  1. git clone --depth 1 (or fetch + reset) → ~/.skills-manage/<host>/<owner>/<repo>/
  2. resolve subpath → skill_dir
  3. compute SHA-256 of skill_dir contents
  4. for each target, expand {category} → append /<name> → apply mode

local:
  1. use source.path directly (no cache, no hash)
  2. smart-skip: if resolved target == source.path, skip
  3. mode=self → verify only
```

## Modes

| Mode    | Action                              | Error condition                  |
|---------|-------------------------------------|----------------------------------|
| symlink | `os.Symlink(skill_dir, dest)`       | dest exists and is not a symlink |
| copy    | `cp -r` (manual walk or `os.CopyFS`) | dest exists                     |
| self    | verify `dest/SKILL.md` exists       | dest missing or no SKILL.md      |

## Implementation Phases

### Phase 1 — Project scaffolding (3 tasks)
- T1: Delete Python source (src/, pyproject.toml, WORK_LOG.md, tests/README.md)
- T2: `go.mod` + directory skeleton + `.gitignore` updates
- T3: Cobra root cmd + `--version`

### Phase 2 — Domain types + manifest I/O (3 tasks)
- T4: `Source` + `ParseGitURL` + `fetch_url()`
- T5: `Target` + 3 modes
- T6: `Manifest` + `Lock` + JSON load/save (v2 schema)

### Phase 3 — Git + hash + install (3 tasks)
- T7: `runGit()` shell-out helper + Clone/Fetch/Reset
- T8: `ComputeFolderHash` SHA-256
- T9: `Install` core (resolve target, apply mode, smart-skip)

### Phase 4 — Subcommands (5 tasks)
- T10: `init` + `validate`
- T11: `install`
- T12: `verify`
- T13: `list` + `outdated` + `lock`
- T14: `update` + `clean`

### Phase 5 — Tests (3 tasks)
- T15: `source_test.go`
- T16: `hash_test.go` + `manifest_test.go` + `install_test.go`
- T17: `cmd_integration_test.go`

### Phase 6 — CI + lint (2 tasks)
- T18: `.golangci.yml` + `.github/workflows/ci.yml`
- T19: `Makefile` + CI lint fixes

### Phase 7 — Docs + cleanup (2 tasks)
- T20: `README.md` rewrite
- T21: `CHANGELOG.md` (start at v0.3.0) + cleanup

Total: 21 tasks across 7 phases. Each phase ends with `make test` + `make lint`.

## Dependencies (go.mod)

```go
require (
    github.com/spf13/cobra v1.8.x
    github.com/stretchr/testify v1.9.x
)
```

No other direct dependencies — stdlib covers git shell-out, SHA-256, JSON, symlinks.