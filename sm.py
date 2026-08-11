"""
sm — declarative skill manager for AI agents (v0.2.2, rolled back from npx-skills wrapper)

Layout philosophy: every skill can be placed in any subdirectory via `category`.
Unlike npx skills (flat `.agents/skills/<name>/`), sm supports categorized layouts
like `.agents/skills/tools/document/pdf/`.

Physical flow (per skill):
  github/gitlab/git:
    1. git clone --depth 1 (or fetch + reset) → ~/.sm/cache/<host>/<owner>/<repo>/
    2. resolve subpath → skill_dir
    3. compute SHA-256 of skill_dir contents
    4. for each target, expand `{category}` template, append /<name>, then:
       - mode=symlink: ln -s skill_dir <dest>
       - mode=copy:    cp -r skill_dir <dest>
       - mode=self:    verify <path> exists + has SKILL.md, no link/copy
  local:
    1. use source.path directly (no cache, no hash needed)
    2. smart-skip: if resolved target == source path, don't create link
    3. mode=self → just verify
"""

__version__ = "0.2.2"

import argparse
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional

CONFIG_FILE = "skills-manage.json"
LOCK_FILE = "skills-manage.lock.json"
DEFAULT_CACHE = Path.home() / ".sm" / "cache"


# ---- io helpers ----

def die(msg, code=1):
    print(f"sm: {msg}", file=sys.stderr)
    sys.exit(code)


def info(msg):
    print(f"sm: {msg}")


def load_json(path: Path, default=None):
    if not path.exists():
        if default is not None:
            return default
        die(f"missing: {path}")
    try:
        with open(path) as f:
            return json.load(f)
    except json.JSONDecodeError as e:
        die(f"invalid JSON in {path}: {e}")


def save_json(path: Path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    with open(path, "w") as f:
        json.dump(data, f, indent=2, ensure_ascii=False)
        f.write("\n")


def run(cmd: list, **kw) -> subprocess.CompletedProcess:
    """Run cmd, raise on failure."""
    result = subprocess.run(cmd, capture_output=True, text=True, **kw)
    if result.returncode != 0:
        die(f"command failed: {' '.join(cmd)}\nstderr: {result.stderr.strip()}")
    return result


# ---- git ops ----

def git_short_sha(repo_dir: Path) -> str:
    return run(["git", "-C", str(repo_dir), "rev-parse", "--short=8", "HEAD"]).stdout.strip()


def git_remote_sha(repo_dir: Path, ref: str) -> Optional[str]:
    """Compare local git HEAD vs remote HEAD at <ref>. None if unreachable."""
    try:
        subprocess.run(
            ["git", "-C", str(repo_dir), "fetch", "--depth", "1"],
            check=False, capture_output=True, text=True, timeout=10,
        )
        r = subprocess.run(
            ["git", "-C", str(repo_dir), "ls-remote", "--heads", "origin", ref],
            capture_output=True, text=True, check=False, timeout=10,
        )
        # Filter to exact ref match: `refs/heads/<ref>` ends the line
        if r.returncode == 0:
            for line in r.stdout.strip().splitlines():
                sha, ref_name = line.split("\t", 1)
                if ref_name == f"refs/heads/{ref}":
                    return sha[:8]
    except (subprocess.TimeoutExpired, Exception):
        pass
    return None


def git_fetch_reset(repo_dir: Path, ref: str):
    """Fetch + reset hard to origin/<ref>."""
    run(["git", "-C", str(repo_dir), "fetch", "--depth", "1", "origin", ref])
    run(["git", "-C", str(repo_dir), "reset", "--hard", f"origin/{ref}"])


# ---- folder hash ----

def compute_folder_hash(folder: Path) -> str:
    """Stable SHA-256 of all files in folder, sorted paths."""
    h = hashlib.sha256()
    for f in sorted(folder.rglob("*")):
        if not f.is_file():
            continue
        rel = f.relative_to(folder).as_posix()
        h.update(rel.encode())
        h.update(b"\0")
        with open(f, "rb") as fh:
            while chunk := fh.read(65536):
                h.update(chunk)
    return h.hexdigest()


# ---- URL parsing ----

def parse_git_url(url: str) -> tuple[str, str, str]:
    """Return (host, owner, repo) from a git URL.

    Supports:
      - https://host/owner/repo[.git]
      - ssh://git@host/owner/repo[.git]
      - git@host:owner/repo[.git]
      - git@host:/path/to/repo (rare; treat /path as owner)
    """
    # Scheme form first (to avoid SCP regex eating https URLs)
    m = re.match(r"^(?:https?|ssh|git)://(?:[^@/]+@)?([^/]+)/(.+?)(?:\.git)?/?$", url)
    if m:
        host, path = m.group(1), m.group(2)
        parts = path.strip("/").split("/")
        if len(parts) >= 2:
            return host, "/".join(parts[:-1]), parts[-1]
        die(f"can't parse URL {url!r}: need owner/repo")

    # SCP form: git@host:owner/repo[.git]
    m = re.match(r"^git@([^:]+):(?!//)(.+?)(?:\.git)?/?$", url)
    if m:
        host, path = m.group(1), m.group(2)
        parts = path.strip("/").split("/")
        if len(parts) >= 2:
            return host, "/".join(parts[:-1]), parts[-1]
        die(f"can't parse URL {url!r}: need owner/repo")

    die(f"can't parse URL {url!r}: not https/ssh/scp")


# ---- path expansion ----

def expand_target(template: str, category: str, name: str,
                 project_root: Path, mode: str) -> Path:
    """Expand {category} placeholders and append /<name> (unless mode=self).

    Returns the symlink path (NOT resolved). The caller may resolve() if needed.
    """
    # expand ~ and {category}
    expanded = template.replace("{category}", category or "")
    if expanded.startswith("~"):
        expanded = os.path.expanduser(expanded)
    p = Path(expanded)
    if not p.is_absolute():
        p = project_root / p
    # mode=self uses the path as-is
    if mode != "self":
        p = p / name
    return p


# ---- skill discovery ----

def find_repo_root(start: Path) -> Optional[Path]:
    """Find the repo root (top-level dir without .git inside)."""
    cur = start.resolve()
    while cur != cur.parent:
        if (cur / ".git").exists():
            return cur
        cur = cur.parent
    return None


def find_skill_dir(repo_root: Path, subpath: Optional[str]) -> Path:
    """Resolve subpath within repo_root. Auto-discover if no subpath given."""
    if subpath:
        # ensure SKILL.md exists at subpath
        candidate = repo_root / subpath
        if not (candidate / "SKILL.md").exists():
            die(f"subpath {subpath!r} has no SKILL.md in {repo_root}")
        return candidate
    # Auto-discover paths (priority order):
    # 1. <repo_root>/SKILL.md
    # 2. <repo_root>/skills/SKILL.md  (catalog)
    # 3. <repo_root>/skills/<name>/SKILL.md  (one subskill)
    # 4. <repo_root>/skills/*/SKILL.md  (auto-pick if exactly one)
    if (repo_root / "SKILL.md").exists():
        return repo_root
    skills_dir = repo_root / "skills"
    if (skills_dir / "SKILL.md").exists():
        return skills_dir
    candidates = list((p.parent) for p in skills_dir.rglob("SKILL.md")) if skills_dir.exists() else []
    if len(candidates) == 1:
        return candidates[0]
    if len(candidates) > 1:
        names = ", ".join(c.relative_to(skills_dir).as_posix() for c in candidates)
        die(f"multiple SKILL.md found under skills/: {names} — set subpath explicitly")
    die(f"no SKILL.md found in {repo_root}")


# ---- data classes ----

@dataclass
class Target:
    agent: str
    path: str
    mode: str = "symlink"

    @classmethod
    def from_dict(cls, d: dict) -> "Target":
        return cls(
            agent=d["agent"],
            path=d["path"],
            mode=d.get("mode", "symlink"),
        )


@dataclass
class SkillSource:
    type: str        # github | gitlab | git | local
    repo: Optional[str] = None     # github/gitlab: "owner/name"; git: also used with host
    host: Optional[str] = None     # gitlab/git: override default host
    url:  Optional[str] = None     # git: full URL (any host, any protocol)
    path: Optional[Path] = None    # local (legacy; now optional — inferred from target)
    subpath: Optional[str] = None
    ref:    Optional[str] = None

    def _resolved_url(self) -> str:
        """Build the clone URL from type-specific fields."""
        if self.type == "github":
            return f"https://github.com/{self.repo}.git"
        if self.type == "gitlab":
            host = self.host or "gitlab.com"
            return f"https://{host}/{self.repo}.git"
        if self.type == "git":
            # Two forms supported:
            #   1. full URL: "url" present
            #   2. host + repo: build "https://{host}/{repo}.git"
            if self.url:
                return self.url
            if not self.host or not self.repo:
                die("git source needs either 'url' OR 'host' + 'repo'")
            return f"https://{self.host}/{self.repo}.git"
        die(f"local sources have no fetch_url")

    def cache_dir(self, cache_root: Path) -> Path:
        """Where this source lives in the central cache (or absolute for local)."""
        if self.type == "local":
            return Path(os.path.expanduser(str(self.path))).resolve()
        if self.type == "github":
            host = "github.com"
            owner_repo = self.repo
        elif self.type == "gitlab":
            host = self.host or "gitlab.com"
            owner_repo = self.repo
        elif self.type == "git":
            url = self.url or f"https://{self.host}/{self.repo}.git"
            host, owner, repo = parse_git_url(url)
            owner_repo = f"{owner}/{repo}"
        else:
            die(f"unknown source type {self.type!r}")
        return cache_root / host / owner_repo

    def fetch_url(self) -> str:
        return self._resolved_url()

    def source_label(self) -> str:
        """Short label for lockfile."""
        if self.type == "local":
            if self.path is None:
                return "local"
            return str(self.path)
        if self.type == "github":
            return self.repo
        if self.type == "gitlab":
            host = self.host or "gitlab.com"
            return f"{host}/{self.repo}"
        if self.type == "git":
            return self.repo or self.url
        return self.url

    def source_url_field(self) -> str:
        """Full URL for lockfile."""
        if self.type == "local":
            return "local"
        return self.fetch_url()

    @classmethod
    def from_dict(cls, d: dict) -> "SkillSource":
        path_str = d.get("path")
        return cls(
            type=d["type"],
            repo=d.get("repo"),
            host=d.get("host"),
            url=d.get("url"),
            path=Path(path_str) if path_str else None,
            subpath=d.get("subpath"),
            ref=d.get("ref", "main"),
        )


@dataclass
class SkillDef:
    name: str
    source: SkillSource
    category: str = ""
    description: str = ""
    targets: list[Target] = field(default_factory=list)
    enabled: bool = True

    @classmethod
    def from_dict(cls, name: str, d: dict) -> "SkillDef":
        if "source" not in d:
            die(f"{name}: v1 manifest no longer supported (missing 'source' object)")
        return cls(
            name=name,
            source=SkillSource.from_dict(d["source"]),
            category=d.get("category", ""),
            description=d.get("description", ""),
            targets=[Target.from_dict(t) for t in d.get("targets", [])],
            enabled=d.get("enabled", True),
        )


def effective_targets(skill: SkillDef, default: list[Target]) -> list[Target]:
    return skill.targets if skill.targets else default


# ---- core install ----

def install_skill(skill: SkillDef, default_targets: list[Target],
                  lock: dict, cache_root: Path, project_root: Path):
    """Install one skill: fetch → resolve → link/copy → update lock."""
    if not skill.enabled:
        info(f"[{skill.name}] disabled, skipping")
        return

    targets = effective_targets(skill, default_targets)
    if not targets:
        die(f"skill '{skill.name}': no targets (root 'targets' empty and no per-skill override)")

    # 1. Fetch source
    src = skill.source
    if src.type == "local":
        # For local, source IS target (no fetch). Path is inferred from
        # the first non-self target. If src.path is set explicitly (legacy),
        # use it; otherwise infer.
        if src.path is not None:
            skill_dir = Path(os.path.expanduser(str(src.path))).resolve()
        else:
            # Infer from first target
            for t in targets:
                if t.mode != "self":
                    skill_dir = expand_target(
                        t.path, skill.category, skill.name, project_root, "symlink"
                    )
                    break
            else:
                # All targets are mode=self
                skill_dir = expand_target(
                    targets[0].path, skill.category, skill.name, project_root, "self"
                )
        if not (skill_dir / "SKILL.md").exists():
            # Missing local source → warn + skip (don't die). User can populate
            # the source dir later, then re-run `sm install --only <name>`.
            info(f"[{skill.name}] SKIP — local source has no SKILL.md: {skill_dir}")
            return
    else:
        cache = src.cache_dir(cache_root)
        url = src.fetch_url()
        ref = src.ref or "main"
        refresh = getattr(skill, '_refresh', False)  # set by cmd_install
        if cache.exists():
            if refresh:
                # `sm update` or `sm install --refresh` — pull upstream
                info(f"[{skill.name}] fetching {url} ref={ref}")
                try:
                    git_fetch_reset(cache, ref)
                except SystemExit:
                    info(f"[{skill.name}] fetch failed, using existing cache")
            else:
                # `sm install` (default) — use cache as-is, just link
                info(f"[{skill.name}] using cached {ref} (no fetch)")
        else:
            # cache missing — clone (no choice)
            info(f"[{skill.name}] cloning {url} ref={ref} → {cache}")
            cache.parent.mkdir(parents=True, exist_ok=True)
            run(["git", "clone", "--depth", "1", "-b", ref, url, str(cache)])
        # resolve subpath inside cached repo
        skill_dir = find_skill_dir(cache, src.subpath)

    # 2. Compute hash (github/gitlab/git only)
    folder_hash = compute_folder_hash(skill_dir) if src.type != "local" else ""

    # 3. Smart-detect: local source == target → don't create link
    # Compare RESOLVED paths so that dest via any symlink chain
    # is detected as the same as skill_dir
    is_self_per_target = {}
    for target in targets:
        dest = expand_target(target.path, skill.category, skill.name, project_root, target.mode)
        is_self = False
        if src.type == "local":
            try:
                is_self = (dest.resolve() == skill_dir)
            except (OSError, RuntimeError):
                is_self = (dest == skill_dir)
        is_self_per_target[target.agent] = is_self

    # 4. Process each target
    for target in targets:
        dest = expand_target(target.path, skill.category, skill.name, project_root, target.mode)
        is_self = is_self_per_target[target.agent]

        if target.mode == "self":
            if not dest.exists():
                die(f"  [{target.agent}] mode=self but {dest} does not exist")
            if not (dest / "SKILL.md").exists():
                die(f"  [{target.agent}] mode=self but no SKILL.md at {dest}")
            info(f"  [{target.agent}] mode=self at {dest} (no link)")
            continue

        # symlink or copy
        if is_self:
            info(f"  [{target.agent}] source IS target ({dest}) — no symlink needed")
            continue

        # Already exists?
        # IMPORTANT: compare dest against skill_dir via resolved paths,
        # because target paths may live under a symlink chain.
        # In that case dest is NOT a symlink itself but resolves to skill_dir.
        if dest.exists() or dest.is_symlink():
            try:
                dest_resolved = dest.resolve()
            except (OSError, RuntimeError):
                dest_resolved = dest
            if dest_resolved == skill_dir:
                # Either dest IS a symlink to skill_dir, OR dest resolves to skill_dir
                # through a parent symlink.
                # In both cases, no action needed.
                info(f"  [{target.agent}] already at target (resolved): {dest}")
                continue
            if dest.is_symlink():
                info(f"  [{target.agent}] stale sm link, updating -> {dest}")
                dest.unlink()
            elif (dest / "SKILL.md").exists():
                # dest is a real dir but NOT our skill_dir — bail (don't touch)
                info(f"  [{target.agent}] {dest} exists with SKILL.md (not sm), skipping")
                continue
            else:
                info(f"  [{target.agent}] {dest} exists (not sm), skipping")
                continue
        # Ensure parent exists
        dest.parent.mkdir(parents=True, exist_ok=True)
        if target.mode == "copy":
            shutil.copytree(skill_dir, dest)
            info(f"  [{target.agent}] copied -> {dest}")
        else:  # symlink
            os.symlink(skill_dir, dest)
            info(f"  [{target.agent}] symlink -> {dest}")

    # 5. Update lockfile entry
    now = subprocess.run(["date", "+%Y-%m-%dT%H:%M:%S%z"],
                         capture_output=True, text=True).stdout.strip()
    skill_path = str(skill_dir.relative_to(skill_dir.parents[len(skill_dir.parents) - (3 if src.type != "local" else 1)])) if False else (
        # skill_path = path within repo (for github/git) or absolute (for local)
        str(skill_dir.relative_to(skill_dir.parents[-2] if src.type != "local" else skill_dir.parent))
        if src.type != "local" else skill_dir.name
    )
    # Simpler: store relative to cache root for github, absolute for local
    if src.type != "local":
        # cache_root / host / owner_repo / ... / skill_dir
        # the path within the repo (relative to repo root)
        cache = src.cache_dir(cache_root)
        try:
            rel = skill_dir.relative_to(cache)
            stored_skill_path = rel.as_posix()
        except ValueError:
            stored_skill_path = skill_dir.name
    else:
        stored_skill_path = skill_dir.name

    entry = lock["skills"].setdefault(skill.name, {})
    is_first_install = "installedAt" not in entry
    entry.update({
        "source": src.source_label(),
        "sourceType": src.type,
        "sourceUrl": src.source_url_field(),
        "ref": src.ref or "main",
        "skillPath": stored_skill_path,
        "category": skill.category,
    })
    if src.type != "local":
        entry["skillFolderHash"] = folder_hash
    else:
        entry["localPath"] = str(skill_dir)
    entry["updatedAt"] = now
    if is_first_install:
        entry["installedAt"] = now


# ---- verify ----

def verify_skill(skill: SkillDef, default_targets: list[Target],
                 lock_entry: dict, cache_root: Path, project_root: Path) -> list[str]:
    issues = []
    src = skill.source
    targets = effective_targets(skill, default_targets)

    # 1. Resolve skill_dir
    if src.type == "local":
        if src.path is not None:
            skill_dir = Path(os.path.expanduser(str(src.path))).resolve()
        else:
            # Infer from first target (same logic as install_skill)
            for t in targets:
                if t.mode != "self":
                    skill_dir = expand_target(
                        t.path, skill.category, skill.name, project_root, "symlink"
                    )
                    break
            else:
                skill_dir = expand_target(
                    targets[0].path, skill.category, skill.name, project_root, "self"
                )
    else:
        cache = src.cache_dir(cache_root)
        if not cache.exists():
            issues.append("cache missing")
            skill_dir = None
        else:
            try:
                skill_dir = find_skill_dir(cache, src.subpath)
            except SystemExit as e:
                issues.append(str(e))
                skill_dir = None

    # 2. SKILL.md at skill_dir
    if skill_dir and not (skill_dir / "SKILL.md").exists():
        issues.append(f"no SKILL.md at {skill_dir}")

    # 3. hash check (for non-local)
    if skill_dir and src.type != "local":
        actual_hash = compute_folder_hash(skill_dir)
        locked_hash = lock_entry.get("skillFolderHash")
        if locked_hash and actual_hash != locked_hash:
            issues.append(
                f"hash mismatch\n"
                f"        locked:  {locked_hash[:16]}\n"
                f"        actual:  {actual_hash[:16]}\n"
                f"        location: {skill_dir}"
            )

    # 4. each target exists
    for target in effective_targets(skill, default_targets):
        dest = expand_target(target.path, skill.category, skill.name, project_root, target.mode)
        if not dest.exists() and not dest.is_symlink():
            issues.append(f"target {target.agent}: missing at {dest}")

    return issues


# ---- subcommands ----

def cmd_init(args):
    target = Path(CONFIG_FILE)
    if target.exists() and not args.force:
        die(f"{target} already exists (use --force to overwrite)")
    example = {
        "$schema": "https://github.com/yxc023/skill-manager/blob/main/SCHEMA.md",
        "version": "2",
        "targets": [
            {"agent": "agents", "path": ".agents/skills/{category}", "mode": "symlink"}
        ],
        "skills": {
            "skill-creator": {
                "source": {"type": "github", "repo": "anthropics/skills",
                           "subpath": "skills/skill-creator", "ref": "main"},
                "category": "tools/meta",
                "description": "Anthropic 元 skill - 创建/改进 skill",
            },
            "weekly-notes": {
                "source": {"type": "local"},
                "category": "workspace/notes",
                "description": "周报 workflow",
            }
        }
    }
    save_json(target, example)
    info(f"created {target}")


def cmd_validate(args):
    cfg = load_json(Path(CONFIG_FILE), {"skills": {}, "targets": []})
    errors = []
    if cfg.get("version") == 1:
        errors.append("manifest is v1 (flat) — must migrate to v2 (source{} object)")
    if "version" not in cfg:
        errors.append("missing 'version' field")
    if not isinstance(cfg.get("skills"), dict):
        errors.append("'skills' must be an object")
    skills = cfg.get("skills", {})
    for name, sdef in skills.items():
        try:
            SkillDef.from_dict(name, sdef)
        except SystemExit as e:
            errors.append(f"{name}: {e}")
    if errors:
        for e in errors:
            print(f"  {e}", file=sys.stderr)
        die(f"{CONFIG_FILE} has {len(errors)} error(s)")
    info(f"{CONFIG_FILE} is valid ({len(skills)} skills, {len(cfg.get('targets', []))} targets)")


def cmd_install(args):
    cfg = load_json(Path(CONFIG_FILE))
    lock_path = Path(LOCK_FILE)
    lock = load_json(lock_path, {"version": "2", "skills": {}})
    if lock.get("version") != "2":
        lock = {"version": "2", "skills": {}}

    skills_dict = cfg.get("skills", {})
    default_targets = [Target.from_dict(t) for t in cfg.get("targets", [])]
    if not default_targets:
        die("no targets defined (add 'targets' at root or per-skill)")

    only = set(args.only.split(",")) if args.only else None
    cache_root = Path(args.cache_dir).expanduser().resolve() if args.cache_dir else DEFAULT_CACHE
    project_root = Path.cwd()
    refresh = getattr(args, 'refresh', False)

    for name, sdef in skills_dict.items():
        if only and name not in only:
            continue
        try:
            sd = SkillDef.from_dict(name, sdef)
        except SystemExit as e:
            die(str(e))
        sd._refresh = refresh  # pass through to install_skill
        install_skill(sd, default_targets, lock, cache_root, project_root)

    save_json(lock_path, lock)
    info(f"wrote {LOCK_FILE}")


def cmd_list(args):
    cfg = load_json(Path(CONFIG_FILE))
    lock_path = Path(LOCK_FILE)
    lock = load_json(lock_path, {"version": "2", "skills": {}}) if lock_path.exists() else {"skills": {}}
    default_targets = [Target.from_dict(t) for t in cfg.get("targets", [])]
    skills_dict = cfg.get("skills", {})

    print(f"{'NAME':<35} {'SOURCE':<35} {'CATEGORY':<25} {'HASH'}")
    print(f"{'-'*35} {'-'*35} {'-'*25} {'-'*12}")
    for name, sdef in skills_dict.items():
        try:
            sd = SkillDef.from_dict(name, sdef)
        except SystemExit:
            continue
        e = lock.get("skills", {}).get(name, {})
        src_label = sd.source.source_label()
        if len(src_label) > 33:
            src_label = src_label[:30] + "..."
        cat = sd.category or "(none)"
        if len(cat) > 23:
            cat = cat[:20] + "..."
        h = e.get("skillFolderHash", e.get("localPath", "—"))[:12]
        print(f"{name:<35} {src_label:<35} {cat:<25} {h}")


def cmd_outdated(args):
    """Compare local git HEAD vs remote git HEAD (NOT content hash).

    Two checks:
      - 'local HEAD' = git SHA currently checked out in cache
      - 'remote HEAD' = git SHA at <ref> on remote
    Different SHAs ⇒ upstream has new commits (run `sm install` to fetch).
    For content-level integrity, use `sm verify`.
    """
    cfg = load_json(Path(CONFIG_FILE))
    cache_root = Path(args.cache_dir).expanduser().resolve() if args.cache_dir else DEFAULT_CACHE
    any_network = False
    for name, sdef in cfg.get("skills", {}).items():
        try:
            sd = SkillDef.from_dict(name, sdef)
        except SystemExit:
            continue
        if sd.source.type == "local":
            continue
        cache = sd.source.cache_dir(cache_root)
        if not cache.exists():
            info(f"{name}\tno cache yet")
            continue
        ref = sd.source.ref or "main"
        try:
            local_sha = git_short_sha(cache)
        except SystemExit:
            local_sha = "?"
        remote_sha = git_remote_sha(cache, ref)
        if remote_sha is None:
            info(f"{name}\tlocal={local_sha}\tremote=?\toffline?")
            continue
        any_network = True
        status = "BEHIND" if remote_sha != local_sha else "up-to-date"
        info(f"{name}\tlocal={local_sha}\tremote={remote_sha}\t{status}")
    if not any_network:
        info("(offline or no remote reachable — set up network to compare)")


def cmd_verify(args):
    cfg = load_json(Path(CONFIG_FILE))
    lock_path = Path(LOCK_FILE)
    lock = load_json(lock_path, {"version": "2", "skills": {}}) if lock_path.exists() else {"skills": {}}
    default_targets = [Target.from_dict(t) for t in cfg.get("targets", [])]
    cache_root = Path(args.cache_dir).expanduser().resolve() if args.cache_dir else DEFAULT_CACHE

    only = set(args.only.split(",")) if args.only else None
    issues = 0
    for name, sdef in cfg.get("skills", {}).items():
        if only and name not in only:
            continue
        try:
            sd = SkillDef.from_dict(name, sdef)
        except SystemExit:
            continue
        errs = verify_skill(sd, default_targets, lock.get("skills", {}).get(name, {}),
                            cache_root, Path.cwd())
        if errs:
            print(f"✗ {name}")
            for e in errs:
                print(f"    {e}")
            issues += 1
        else:
            h = lock.get("skills", {}).get(name, {}).get("skillFolderHash", "(local)")
            print(f"✓ {name:<35} {h[:12] if isinstance(h, str) and len(h) >= 12 else h}")
    if issues:
        die(f"{issues} skill(s) with issues")
    checked = len(only) if only else len(cfg.get("skills", {}))
    info(f"all {checked} verified")


def cmd_clean(args):
    """Remove sm-managed symlinks. Cache preserved at ~/.sm/cache/."""
    cfg = load_json(Path(CONFIG_FILE))
    lock_path = Path(LOCK_FILE)
    lock = load_json(lock_path, {"version": "2", "skills": {}}) if lock_path.exists() else {"skills": {}}
    default_targets = [Target.from_dict(t) for t in cfg.get("targets", [])]
    cache_root = Path(args.cache_dir).expanduser().resolve() if args.cache_dir else DEFAULT_CACHE
    project_root = Path.cwd()

    removed = 0
    for name, sdef in cfg.get("skills", {}).items():
        try:
            sd = SkillDef.from_dict(name, sdef)
        except SystemExit:
            continue
        for target in effective_targets(sd, default_targets):
            dest = expand_target(target.path, sd.category, sd.name, project_root, target.mode)
            if target.mode == "self":
                continue  # don't touch
            if dest.is_symlink():
                # only remove if it points into our cache (i.e. sm-managed)
                try:
                    target_path = dest.resolve()
                    cache = sd.source.cache_dir(cache_root) if sd.source.type != "local" else None
                    if cache and target_path.is_relative_to(cache):
                        dest.unlink()
                        info(f"removed {dest}")
                        removed += 1
                except (OSError, ValueError):
                    pass
            elif dest.is_dir() and target.mode == "copy":
                shutil.rmtree(dest)
                info(f"removed {dest} (copy)")
                removed += 1
    info(f"removed {removed} sm-managed link(s) (cache preserved at {cache_root})")


def cmd_lock(args):
    lock_path = Path(LOCK_FILE)
    if not lock_path.exists():
        die(f"no {LOCK_FILE}")
    print(lock_path.read_text())


def cmd_update(args):
    """Refresh cache from upstream + relink. (Like `npm update`.)"""
    args.refresh = True  # force refresh in cmd_install
    if args.only:
        info(f"updating subset: {args.only}")
    else:
        info("updating all installed skills (fetching upstream)")
    cmd_install(args)


# ---- main ----

def build_parser():
    ap = argparse.ArgumentParser(
        prog="sm",
        description=f"sm — declarative skill manager for AI agents (v{__version__})",
    )
    ap.add_argument("--version", action="version", version=f"sm {__version__}")
    ap.add_argument("-C", "--config-dir", default=".",
                    help=f"directory containing {CONFIG_FILE}")
    ap.add_argument("--cache-dir", default=None,
                    help=f"override cache (default: {DEFAULT_CACHE})")
    sub = ap.add_subparsers(dest="cmd", required=True)

    p = sub.add_parser("init", help=f"create starter {CONFIG_FILE}")
    p.add_argument("-f", "--force", action="store_true")

    sub.add_parser("validate", help=f"validate {CONFIG_FILE} schema")

    p = sub.add_parser("install", help="install skills (link only, no fetch if cache exists)")
    p.add_argument("--only", help="comma-separated skill names")
    p.add_argument("--refresh", action="store_true",
                   help="also fetch upstream before linking (same as `sm update`)")

    sub.add_parser("list", help="list configured skills + lock hashes")

    sub.add_parser("outdated", help="show which skills have upstream commits")

    p = sub.add_parser("verify", help="verify each skill's SKILL.md + hash + targets")
    p.add_argument("--only", help="comma-separated skill names")

    p = sub.add_parser("update", help="alias for install (hard reset to remote)")
    p.add_argument("--only", help="comma-separated skill names")

    sub.add_parser("clean", help="remove sm-managed symlinks (cache preserved)")

    sub.add_parser("lock", help=f"show {LOCK_FILE}")

    return ap


def main():
    args = build_parser().parse_args()
    os.chdir(args.config_dir)
    handlers = {
        "init": cmd_init,
        "validate": cmd_validate,
        "install": cmd_install,
        "list": cmd_list,
        "outdated": cmd_outdated,
        "verify": cmd_verify,
        "update": cmd_update,
        "clean": cmd_clean,
        "lock": cmd_lock,
    }
    handlers[args.cmd](args)


if __name__ == "__main__":
    main()