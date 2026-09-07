---
name: skill-manage-skill
description: Use the `skill-manager` CLI to install, update, validate, and audit agent skills. Invoke when the user asks to install a skill from a git repo, manage `skills-manage.json`, fix a broken skill, check which skills are out of date, or set up a new project that uses agent skills. Do not use for non-skill CLI tooling, package management, or skill authoring (that's a different concern).
---

# skill-manager

Declarative skill manager for AI agents — think `npm` for skills. Clones skill repos from git, computes content hashes, and links them into agent directories (`~/.config/opencode/skills/`, `.claude/skills/`, etc.).

The binary is `skill-manager`. There is no daemon, no service, no API — every action is a one-shot CLI invocation that reads `skills-manage.json` and (for writes) updates `skills-manage.lock.json`.

## When to use this skill

Trigger on user phrases like:

- "install this skill" / "add this skill to my agent" / "set up skills for this project"
- "what skills do I have installed" / "list installed skills"
- "which skills are out of date" / "check for skill updates"
- "update my skills" / "bump skill versions" / "refresh skills"
- "validate my skills-manage.json" / "check my manifest"
- "verify my skills" / "are my skills intact" / "check skill hashes"
- "clean stale skills" / "remove unused skills"
- "init skill manager" / "set up skill-manager in this repo"

Do **not** trigger on:

- Authoring a new skill (writing a SKILL.md) — use the skill-authoring toolchain for that.
- Installing npm/pip/cargo packages — different ecosystems.
- General "git clone" requests without the word "skill" or a `skills-manage.json` reference.

## Quick reference: subcommands

| Command | Purpose | When to run |
|---|---|---|
| `skill-manager init` | Create a starter `skills-manage.json` in cwd | First time in a project |
| `skill-manager install` | Clone/fetch + link/copy + write lock | After manifest changes, on clone, on new machine |
| `skill-manager update` | Alias for `install --update` (force fresh clones) | When `outdated` reports changes |
| `skill-manager validate` | Schema-check manifest, sources, target paths | Before committing a manifest change |
| `skill-manager verify` | Check SKILL.md + hash + target existence | CI, post-install sanity check |
| `skill-manager list` | Tabular view: name, category, enabled, source, hash | Quick audit |
| `skill-manager outdated` | Show which skills have new commits upstream vs local cache | Periodic check, pre-update |
| `skill-manager clean` | Remove skills no longer in manifest | After deleting entries from manifest |
| `skill-manager lock` | Re-derive lock file from current cache state | After manual cache edits |

All commands accept `--manifest <path>` (default `skills-manage.json`) and `--verbose` / `-v`.

## Standard workflow (most projects)

```bash
# 1. First time in a project — create the manifest
skill-manager init

# 2. Add skills to skills-manage.json (hand-edit), then install
skill-manager install

# 3. Periodic check
skill-manager outdated      # any output? → run install
skill-manager verify        # any failures? → run install or investigate

# 4. After bumping versions in the manifest
skill-manager install

# 5. After removing entries from the manifest
skill-manager clean
```

## Manifest format (`skills-manage.json`)

JSON, schema v2 (integer). See `SCHEMA.md` for the full reference — the canonical source.

```json
{
  "version": 2,
  "targets": [
    {"agent": "opencode", "path": ".opencode/skills/{category}", "mode": "symlink"}
  ],
  "skills": {
    "pdf": {
      "source": {"type": "github", "repo": "anthropics/skills", "subpath": "skills/pdf"},
      "category": "tools/document"
    },
    "my-local-skill": {
      "source": {"type": "local", "path": "~/work/my-local-skill"},
      "category": "personal"
    }
  }
}
```

**Source types:**

| `type` | Required fields | Notes |
|---|---|---|
| `github` | `repo` ("owner/name") | Auto-constructs `https://github.com/<repo>.git` |
| `gitlab` | `repo` | Only `gitlab.com`; self-hosted → use `git` |
| `git` | `url` | Any host (SSH, self-hosted, intranet) |
| `local` | `path` | Filesystem path, supports `~`; no cache |

**Install modes (`targets[].mode`):**

| Mode | Behavior | Use when |
|---|---|---|
| `symlink` (default) | `<dest>` is a symlink → central cache dir | Local dev, want live edits to cache |
| `copy` | `<dest>` is a copy of cache dir | Production, containers, no-symlink envs |
| `self` | Skip link/copy, just verify `dest/SKILL.md` exists | Skill already lives at the target path |

For every mode, `dest` is computed as `expand(target.path) + /<skill-name>`, where `{category}` in `target.path` is substituted with the skill's `category` (or removed if empty).

## Lock file (`skills-manage.lock.json`)

**Auto-generated. Do not hand-edit.** Committed optionally (the repo's `.gitignore` excludes it by default — regenerate per-machine via `install`).

Schema v2 contains, per skill:

```json
{
  "source": {"type": "github", "repo": "anthropics/skills", "subpath": "skills/pdf", "ref": "main"},
  "category": "tools/document",
  "skillFolderHash": "ad4f350be137206c..."
}
```

`skillFolderHash` is a deterministic local SHA-256 (sorted paths + `relpath\0mode\0content`), used by `verify` to detect local cache tampering. **Not** a GitHub Tree SHA — it's local-only.

## Common pitfalls

- **`version` is integer `2`, not string `"2"`.** Older guides show `"2"`. Wrong type → `validate` errors.
- **`skillFolderHash` includes file mode.** A `chmod` inside the cache invalidates the hash. Re-run `install` to recompute.
- **`self` mode still appends `/<skill-name>`.** Don't pre-pend the name to `target.path` — let the resolver do it.
- **Local sources have no cache.** `~/.skills-manage/` doesn't get a local entry; the source path *is* the source.
- **`update` rewrites the cache.** It does a fresh clone, so any local edits to cache contents are lost.
- **The manifest is JSON5-ish-ish but it's just JSON.** No comments allowed. Trailing commas fail.

## Self-installation (dogfooding)

This repo (`yxc023/skill-manager`) ships its own skill at `skills/skill-manage-skill/`. To install it into your own agent:

```bash
# In your target project
cat > skills-manage.json <<'EOF'
{
  "version": 2,
  "targets": [{"agent": "opencode", "path": ".opencode/skills/{category}", "mode": "symlink"}],
  "skills": {
    "skill-manage-skill": {
      "source": {"type": "github", "repo": "yxc023/skill-manager", "subpath": "skills/skill-manage-skill"},
      "category": "tools/meta"
    }
  }
}
EOF
skill-manager install
```

## Debugging failures

- `validate` fails → read the error, fix the manifest (most failures are schema-level).
- `verify` fails on hash mismatch → cache contents drifted; run `install` to refresh.
- `install` hangs on a `github` source → check `~/.gitconfig`, SSH keys, network.
- `outdated` reports "fetch error" → remote ref may have been force-pushed; run `install` to reset cache.

For the canonical schema and field semantics, defer to `SCHEMA.md` in this repo — do not duplicate it here.