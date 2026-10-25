# Iris Online Database — CONTINUE NEXT RUN (iter146)

**UTC:** 2026-10-25 18:16 UTC. **Status:** one local `ai-development` engineering iteration; full Go BLOCKED, Windows QA NOT RUN, workflow NOT PASS. **NOT a release.**

## Authentic input and Git

- Enumerated recent conversation and Library ZIP/checkpoints; selected newest complete Library iter145, byte-identical to conversation ZIP. Input SHA-256 `2b8d373024f0f4cc55324c312dfd245d12c17f927bccf6ebc8aa353d80a2c3ef`, 13,127,576 bytes, 778 ZIP files, sole root `iris-online-database`; CRC/full extraction/byte identity 778/778 PASS. Sorted filename/tab/lowercase-CRC32/newline manifest SHA-256 `4785a5ed77d50604c1c95438662cbb811e784235aeeeaf66a01c08097acaf83a`. Library iter145 latest checkpoint materialized. Input Git `ai-development` HEAD `9dac7d487573e5d400dc5d733e23bf481e73b83d`, parent `3d741b6823bd0c75b820ad577e76543a01f54623`, clean, fsck PASS (pre-existing dangling commit). VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`; required docs and release scripts tracked. Named engineering standard not located in accessible Library; explicit constraints applied.

## Exactly one fix and actual QA

- `tools/verify_executables.py` failed to check compiler version in the first `go version -m` line against pinned `.go-version`. With all other markers valid, mismatched/absent compiler headers produced **3 RED, 1 control PASS before**; now **5 PASS after** (including fake second-line case). Minimal Python-only release verification fix; no app/UI/game data/Go pin/security downgrade/Windows copy changed.
- Linux Python 3.13.5 `PYTHONPATH=tools` and external pycache: **274 unittest PASS**, compileall PASS, data audit `fatal=[]` PASS, repository audit PASS, Node 22.16.0 syntax PASS, gofmt PASS, offline Chromium/Playwright navigation PASS, Git diff --check/fsck PASS. Workflow static PASS; Windows PowerShell parser **NOT RUN**, workflow overall **NOT PASS** (exit 2). Full Go test/vet/race **BLOCKED** (offline Linux Go 1.23.2 vs go.mod >=1.25); isolated Go NOT RUN (Python-only patch). Windows Wails/WebView2/Quality Gate/PowerShell/race NOT RUN; frontend TCP smoke NOT RUN; Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.

## Storage and Windows divergence

- One local `ai-development` commit, clean tree, full `.git` ZIP SHA/CRC/full extraction/source↔ZIP↔extracted byte identity/Git/fsck and Library upload/readback mandatory before latest pointer promotion. Immutable checkpoint readback before latest pointer. Output SHA/HEAD stored in external checkpoint, not inside ZIP. No main/remote/tags/Releases changes. **NOT a release.**
- **CRITICAL:** archived `.go-version=1.26.6` NOT reconciled with working Windows copy after `ai-development→main` merge and Go/security updates. Never replace Windows copy, downgrade Go/security or publish.
- **Exactly one next action:** obtain authentic read-only snapshot of working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
