# skill-manager

> Declarative skill manager for AI agents — think npm for skills.

`skill-manager` clones skill repos from git, computes a SHA-256 hash of each
skill folder, and links them into agent directories like `.opencode/skills/`.
It uses a manifest (`skills-manage.json`) to declare which skills to install
and where, then produces a lock file (`skills-manage.lock.json`) recording the
exact contents installed.

## Install

```bash
go install github.com/yxc023/skill-manager@v0.3.0
```

Or build from source:

```bash
git clone https://github.com/yxc023/skill-manager
cd skill-manager
make install   # installs to $GOBIN
```

Requires Go 1.21+ and a `git` binary on `PATH`.

## Quick start

```bash
# 1. Create a starter manifest in your project
cd my-project
skill-manager init

# 2. Edit skills-manage.json to declare skills you want
cat skills-manage.json
# {
#   "version": 2,
#   "targets": [
#     {"agent": "opencode", "path": ".opencode/skills/{category}", "mode": "symlink"}
#   ],
#   "skills": {
#     "my-skill": {
#       "source": {"type": "github", "repo": "anthropics/skills"},
#       "category": "tools/my-skill"
#     }
#   }
# }

# 3. Validate the manifest
skill-manager validate

# 4. Install (clone + link + write lock)
skill-manager install

# 5. Verify everything is in place
skill-manager verify
```

## Manifest schema

`skills-manage.json` (v2) — see [`SCHEMA.md`](SCHEMA.md) for full reference.

```json
{
  "version": 2,
  "targets": [
    {"agent": "opencode", "path": ".opencode/skills/{category}", "mode": "symlink"},
    {"agent": "claude",   "path": ".claude/skills/{category}",     "mode": "copy"}
  ],
  "skills": {
    "my-skill": {
      "source": {"type": "github", "repo": "owner/repo", "ref": "v1.0"},
      "category": "tools/my-skill",
      "enabled": true,
      "targets": [
        {"agent": "opencode", "path": "/custom/{category}", "mode": "symlink"}
      ]
    },
    "local-skill": {
      "source": {"type": "local", "path": "~/work/my-local-skill"}
    },
    "raw-git": {
      "source": {"type": "git", "url": "git@github.com:o/r.git", "ref": "main", "subpath": "skills/foo"}
    }
  }
}
```

**Source types:**

| Type    | Fields                                            | Notes                                          |
|---------|---------------------------------------------------|------------------------------------------------|
| github  | `repo` (required), `host`, `ref`, `subpath`       | Defaults to `github.com`, ref `main`           |
| gitlab  | `repo` (required), `host`, `ref`, `subpath`       | Defaults to `gitlab.com`, ref `main`           |
| git     | `url` (required), `ref`, `subpath`                | Any git URL (https or ssh)                     |
| local   | `path` (required)                                 | No clone, no hash                              |

**Target modes:**

| Mode     | Action                                                  |
|----------|---------------------------------------------------------|
| symlink  | `ln -s <skill_dir> <dest>` (default)                    |
| copy     | `cp -r <skill_dir> <dest>`                              |
| self     | verify `<dest>/SKILL.md` exists; no link/copy           |

## Commands

```
skill-manager init          Create a starter skills-manage.json
skill-manager install       Clone/fetch + link/copy + write lock
skill-manager install -v    Verbose: print shell commands
skill-manager update        Alias for `install --update` (force fresh clones)
skill-manager validate      Check manifest schema, sources, target paths
skill-manager verify        Check SKILL.md + hash + target existence
skill-manager list          Tabular view: name, category, enabled, source, hash
skill-manager outdated      Show skills with new upstream commits
skill-manager clean         Remove symlinks/dirs at all targets (cache preserved)
skill-manager lock          Print skills-manage.lock.json
```

## Cache

Cloned repos live in `~/.skills-manage/<host>/<owner>/<repo>/`. The cache is
shallow (`--depth 1`) and reused across installs — `skill-manager install`
fetches and hard-resets to the configured ref without re-cloning.

## Development

```bash
make test       # go test -race -cover ./...
make lint       # golangci-lint run
make build      # go build -ldflags "-X ...Version=$(git describe)" -o bin/skill-manager
make install    # go install
```

## Architecture

```
cmd/skill-manager/main.go      # entry point
internal/cmd/                  # cobra subcommands
internal/skillmanager/         # core domain (Source, Target, Manifest, Install, GitRunner, ComputeFolderHash)
internal/ui/                   # output helpers
```

Git operations shell out to the system `git` binary so SSH keys and user
config are reused. No `go-git` dependency.

## License

MIT — see [`LICENSE`](LICENSE).

## Migration from Python `sm` (v0.2.2)

Manifest schema is wire-compatible except for one field: `version` is now
integer `2` (was string `"2"`). Search-and-replace `"version": "2"` →
`"version": 2` in any existing `skills-manage.json`.

Lock file is **not** wire-compatible — Go writes only `source`, `category`,
`skillFolderHash`. Old Python lock files with `installedAt`/`updatedAt`/
`sourceType`/etc. will still load (extras are ignored), but the next `install`
will rewrite the file in the new minimal format.

Other breaking items: cache directory renamed `~/.sm/cache/` (Python) →
`~/.skills-manage/` (Go); Python binary `sm` → `skill-manager`.