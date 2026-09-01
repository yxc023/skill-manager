# Changelog

All notable changes to `skill-manager` are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/).

## [0.3.0] — 2026-09-01

### Changed
- **BREAKING:** Complete rewrite from Python to Go. Binary renamed from `sm` to `skill-manager`.
- **BREAKING:** Cache directory renamed from `~/.sm/cache/` to `~/.skills-manage/`. No migration path.
- Manifest JSON format (`skills-manage.json` v2) is unchanged — existing files work as-is.
- Lock file format (`skills-manage.lock.json`) is unchanged.

### Added
- Cobra-based CLI with 9 subcommands: `init`, `install`, `validate`, `verify`, `list`, `outdated`, `update`, `clean`, `lock`.
- Global flags: `--manifest` (default `skills-manage.json`), `--verbose`.
- Shell-out to system `git` (no `go-git` dependency) — reuses user SSH keys and git config.
- Deterministic SHA-256 folder hash (`internal/skillmanager/hash.go`).
- Three install modes: `symlink` (default), `copy`, `self`.
- Smart-skip for local sources when target resolves to source path.
- Test suite: 27 tests across `internal/skillmanager` and `internal/cmd`.
- GitHub Actions CI matrix: Go 1.21 / 1.22 / 1.23 × ubuntu-latest / macos-latest.
- `golangci-lint` config (errcheck, govet, ineffassign, staticcheck, unused, gofmt, goimports).
- Makefile: `test`, `lint`, `build`, `install`, `tidy`, `clean` with `-ldflags` version injection.

### Removed
- Python source (`sm.py`, `src/skill_manager/`).
- `pyproject.toml` (Python packaging).
- `WORK_LOG.md` (replaced by this changelog).

## [0.2.2] — 2026-07-29

Last Python release. Single-file `sm.py` with PEP 621 metadata, console script `sm`,
`uv tool install .` workflow. Pure stdlib, no runtime deps. Pre-PyPI.