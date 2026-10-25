# Iris Online Database — CONTINUE NEXT RUN (iter144)

**UTC:** 2026-10-25 16:16 UTC. **Status:** one local `ai-development` engineering iteration; full Go BLOCKED, Windows QA NOT RUN, workflow NOT PASS. **NOT a release.**

## Authentic input

- Enumerated conversation iter134–143 and Library Engineering_Candidates. Latest complete Library iter143 ZIP is byte-identical to conversation. Input SHA-256 `b95c680f22796418514209b371ff9ee52f7a7e4dfd24d9d3ea2a0f6d33fd4429`, 12,744,378 bytes, 760 ZIP files, sole root `iris-online-database`; CRC/full extraction/byte identity 760/760 PASS. Sorted filename/CRC32 manifest SHA-256 `04a071ff58efa722092cd16b0683b9e6712f37ee8ac15a25b5ba510eb1e970c7`. Latest and immutable iter143 checkpoints byte-identical SHA-256 `ec473179325f36db5a11c68d8f89aee6325983dc16e55323cc29a82a7270f565`.
- Input Git `ai-development` HEAD `ebfe6243a4312c7767440d526e2d335df1ecc70f`, parent `4813291df686ded799b6bddbe662ffcf25f0ed8e`, clean, fsck PASS (pre-existing dangling commit). VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`; ARCHITECTURE/BUILD/RELEASE docs, IrisTools.ps1, AI log and continuation present. Main/origin-main `f17d835e7443eec6d0dd5884ad3ddb53867ad214`; nine tags. Named engineering standard not located in accessible Library; explicit constraints applied.

## One narrow fix, RED→GREEN, actual QA

- `tools/validate_workflows.py` previously accepted stale status captures and early exit guards for Windows quality, race, and Go-test gates. **Six RED before → six GREEN after**, unchanged control PASS. Minimal command → immediate `$LASTEXITCODE` capture → later guard ordering requirement; regression in `tests/test_workflow_platform_gate.py`. Low-risk Python-only static validator; no workflow YAML/app/UI/game data/security pin/profile/telemetry/network/working Windows copy changed.
- Linux Python 3.13.5 with `PYTHONPATH=tools`, external pycache: **272 unittest PASS**, compileall PASS, data audit `fatal=[]` PASS (13,927 items; 1,342 monsters), repository audit `WARN: none` PASS, Node 22.16.0 JS syntax PASS, gofmt PASS, offline Chromium/Playwright monster-drop navigation PASS, Git diff --check/fsck PASS. Workflow static PASS, native Windows PowerShell parser **NOT RUN [WINDOWS_POWERSHELL]**, workflow **NOT PASS** (exit 2). Full Go test/vet/race **BLOCKED** (offline Linux Go 1.23.2 vs go.mod >=1.25); isolated Go NOT RUN (Python-only). Windows Wails/WebView2/Quality Gate/PowerShell/race NOT RUN; frontend TCP smoke NOT RUN; Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.

## Storage and critical Windows divergence

- Exactly one local `ai-development` commit, clean tree; complete `.git` ZIP SHA/CRC/full extraction/source↔ZIP↔extracted byte identity/Git/fsck and Library upload/readback required before latest pointer promotion. Immutable checkpoint readback before latest pointer. Output SHA and HEAD stored in external checkpoint, not inside ZIP. No main/remote/tags/Releases changes. NOT a release.
- **CRITICAL:** archived `.go-version=1.26.6` NOT reconciled with working Windows copy after `ai-development→main` merge and Go/security updates. Never replace Windows copy, downgrade Go/security or publish.
- **Exactly one next action:** obtain authentic read-only snapshot of working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
