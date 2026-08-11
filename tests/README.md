# Tests for sm (skill manager)

Currently no automated tests — see `WORK_LOG.md` for manual test history.

## Run `sm` locally without installing

```bash
python3 /path/to/skill-manager/sm.py install
```

## Run `sm` globally via uv

```bash
uv tool install /path/to/skill-manager --force
sm install
```

## Suggested future tests

- `test_smoke.py` — run `sm validate` against a fixture manifest, expect exit 0
- `test_install.py` — sandboxed tmpdir, init manifest + cache, install, verify
- `test_url_parsing.py` — unit-test `parse_git_url` against 5 URL forms
- `test_local_smartskip.py` — local source == target detection (the resolved-path fix)