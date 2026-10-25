# Iris Online Database — CONTINUE NEXT RUN (iter145)

**UTC:** 2026-10-25 17:18 UTC. **Status:** one local `ai-development` engineering iteration; full Go BLOCKED, Windows QA NOT RUN, workflow NOT PASS. **NOT a release.**

## Authentic input

- Enumerated conversation iter135–144 and Library Engineering_Candidates ZIP/checkpoints; latest complete Library iter144 was byte-identical to conversation. Input SHA-256 `f8c1f9d39e49db932455ba16ff8673259d1b43f82f799d2f71d029b2c91c698f`, 12,938,839 bytes, 769 ZIP entries, sole root `iris-online-database`; CRC/full extraction/byte identity 769/769 PASS; sorted filename/tab/lowercase-CRC32/newline manifest SHA-256 `dd0d74579ae33848f81b4417631aef47d91b2af9ff01fbd85896f1ec3a1af1fd`. Latest and immutable iter144 checkpoints byte-identical SHA-256 `d72b77a6253041cb4e20e16541b5529dd4fd184a5134060c4ceb472dfe17eab4`.
- Input Git `ai-development` HEAD `3d741b6823bd0c75b820ad577e76543a01f54623`, parent `ebfe6243a4312c7767440d526e2d335df1ecc70f`, clean, fsck PASS (pre-existing dangling commit). VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`; required docs, IrisTools.ps1, AI log and continuation present/tracked. Main/origin-main `f17d835e7443eec6d0dd5884ad3ddb53867ad214`; nine tags. Named engineering standard not located in accessible Library; explicit constraints applied.

## One narrow fix, RED→GREEN, actual QA

- `tools/repository_audit.py` excluded itself from SEC001/SEC002 scans. Two negative subcases **RED before → GREEN after**; unchanged audit control PASS. Self-scan now checks credentials/developer paths; only self-referential code markers remain excluded from warnings. Equivalent path regex avoids self-match. Low-risk Python-only security audit fix + regression; no workflow YAML/app/UI/game data/security pin/profile/telemetry/network/working Windows copy changed.
- Linux Python 3.13.5 with `PYTHONPATH=tools`, external pycache: **273 unittest PASS**, compileall PASS, data audit `fatal=[]` PASS, repository audit `WARN: none` PASS, Node 22.16.0 JS syntax PASS, gofmt PASS, offline Chromium/Playwright monster-drop navigation PASS, Git diff --check/fsck PASS. Workflow static PASS, native Windows PowerShell parser **NOT RUN [WINDOWS_POWERSHELL]**, workflow **NOT PASS** (exit 2). Full Go test/vet/race **BLOCKED** (offline Linux Go 1.23.2 vs go.mod >=1.25); isolated Go NOT RUN (Python-only). Windows Wails/WebView2/Quality Gate/PowerShell/race NOT RUN; frontend TCP smoke NOT RUN; Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.

## Storage and critical Windows divergence

- Exactly one local `ai-development` commit, clean tree; complete `.git` ZIP SHA/CRC/full extraction/source↔ZIP↔extracted byte identity/Git/fsck and Library upload/readback required before latest pointer promotion. Immutable checkpoint readback before latest pointer. Output SHA and HEAD stored in external checkpoint, not inside ZIP. No main/remote/tags/Releases changes. NOT a release.
- **CRITICAL:** archived `.go-version=1.26.6` NOT reconciled with working Windows copy after `ai-development→main` merge and Go/security updates. Never replace Windows copy, downgrade Go/security or publish.
- **Exactly one next action:** obtain authentic read-only snapshot of working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
