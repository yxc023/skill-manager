# Changelog

All notable changes to `skill-manager` are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/).

## [0.4.0](https://github.com/yxc023/skill-manager/compare/skill-manager-v0.3.0...skill-manager-v0.4.0) (2026-09-08)


### Added

* skill-manager v0.3.0 — Go rewrite ([#1](https://github.com/yxc023/skill-manager/issues/1)) ([7b20490](https://github.com/yxc023/skill-manager/commit/7b20490b9ac8f5bdf1683af72b1658ec5052f503))
* sm v0.2.2 — declarative skill manager for AI agents ([f0c3aba](https://github.com/yxc023/skill-manager/commit/f0c3abab6a3a3947bcce06a02b11b0c3af0c8a7e))


### Fixed

* **release:** use kebab-case keys in release-please-config.json ([0b2f18d](https://github.com/yxc023/skill-manager/commit/0b2f18deac5e996afe43fc86ff051cd4845f24de))


### Documentation

* add skill-manage-skill — dogfooding the CLI to document itself ([79bcfb2](https://github.com/yxc023/skill-manager/commit/79bcfb27d1fc94afeab389a850cf91d147de7626))
* align post-release notes with actual Go implementation ([4f7ee91](https://github.com/yxc023/skill-manager/commit/4f7ee91281f4b2a7c394b58c19471c049df6cf32))

## [0.3.0] — 2026-09-01

### Changed
- **BREAKING:** Complete rewrite from Python to Go. Binary renamed from `sm` to `skill-manager`.
- **BREAKING:** Cache directory renamed from `~/.sm/cache/` to `~/.skills-manage/`. No migration path.
- **BREAKING:** Manifest `version` field is now integer `2` (was string `"2"` in Python). One-character fix per existing manifest.
- **BREAKING:** Lock file format (`skills-manage.lock.json`) simplified — only `source`, `category`, `skillFolderHash` are written. Removed (still read-tolerated): `sourceType`, `sourceUrl`, `ref`, `skillPath`, `installedAt`, `updatedAt`, `localPath`.

### Added
- Cobra-based CLI with 9 subcommands: `init`, `install`, `validate`, `verify`, `list`, `outdated`, `update`, `clean`, `lock`.
- Global flags: `--manifest` (default `skills-manage.json`), `--verbose`.
- Shell-out to system `git` (no `go-git` dependency) — reuses user SSH keys and git config.
- Deterministic SHA-256 folder hash (`internal/skillmanager/hash.go`).
- Three install modes: `symlink` (default), `copy`, `self`.
- Smart-skip for local sources when target resolves to source path.
- Test suite: 27 tests across `internal/skillmanager` and `internal/cmd`.
- GitHub Actions CI matrix: Go 1.23 / 1.24 × ubuntu-latest / macos-latest (dropped 1.21/1.22 — see `.github/workflows/ci.yml` for rationale).
- `golangci-lint` config (errcheck, govet, ineffassign, staticcheck, unused, gofmt, goimports).
- Makefile: `test`, `lint`, `build`, `install`, `tidy`, `clean` with `-ldflags` version injection.

### Removed
- Python source (`sm.py`, `src/skill_manager/`).
- `pyproject.toml` (Python packaging).
- `WORK_LOG.md` (replaced by this changelog).

## [0.2.2] — 2026-07-29

Last Python release. Single-file `sm.py` with PEP 621 metadata, console script `sm`,
`uv tool install .` workflow. Pure stdlib, no runtime deps. Pre-PyPI.
