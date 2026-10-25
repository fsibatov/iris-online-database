# Iris Online Database — CONTINUE NEXT RUN (iter143)

**UTC:** 2026-10-25 15:16 UTC. **Status:** one local `ai-development` engineering iteration; full Go BLOCKED, Windows QA NOT RUN, workflow NOT PASS. **NOT a release.**

## Authentic source and integrity

- Enumerated conversation iter133–142 and Library ZIP/checkpoints; selected latest complete Library iter142 ZIP, byte-identical to conversation. Input SHA-256 `cb587b298849677192a640adaff38d78ab2c612891c9d75cde6aed59d36e97ee`, 12,552,115 bytes, 750 entries, sole `iris-online-database` root; CRC 750/750, full extraction and byte identity 750/750 PASS; sorted filename/CRC32 manifest SHA-256 `a2a9ef67ede1fe09500f9ff2f9014136a4975df4838c0c1490215454e91dc90e`. Latest/immutable iter142 checkpoints identical SHA-256 `33f8538d8cc7763d165517f6d226162c44faf6616c2034ae172fbdf1aefb1885`. Named methodology file not found in accessible Library; explicit constraints followed.
- Input Git `ai-development` HEAD `4813291df686ded799b6bddbe662ffcf25f0ed8e`, parent `a5b2c9a4e2fb3fc50840e7bdaa17ab29cf5462bb`, clean, fsck PASS (pre-existing dangling commit). VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`; ARCHITECTURE/BUILD/RELEASE, IrisTools.ps1, AI log and continuation present/tracked. Main/origin-main `f17d835e7443eec6d0dd5884ad3ddb53867ad214`; nine tags unchanged.

## Exactly one fix and actual QA

- Static workflow policy previously accepted VK-news runner drift from `windows-2025` to `windows-2022`, `windows-latest` or invalid `windows-unknown`. **3 RED before → 3 GREEN after**, unchanged control PASS. Exact runner guard in `tools/validate_workflows.py`, new parameterized test in `tests/test_workflow_platform_gate.py`, updated existing static contract in `tests/test_quality_contract.py`. No actual workflow modified. Low-risk Python-only validation; not Windows execution. App/UI/game data/security pins/profile/network/telemetry/Windows working copy untouched.
- Linux Python 3.13.5, `PYTHONPATH=tools`, external pycache: **271 unittest PASS**, compileall PASS, data audit `fatal=[]` PASS, repository audit `WARN: none` PASS, Node 22.16.0 `node --check web/*.js` PASS, gofmt zero unformatted, offline Chromium/Playwright monster-drop navigation PASS, Git diff --check/fsck PASS. Workflow static PASS, Windows PowerShell parser **NOT RUN [WINDOWS_POWERSHELL]**, overall workflow **NOT PASS** (exit 2). Full Go test BLOCKED: Linux Go 1.23.2 < go.mod 1.25.0, offline; full vet/race NOT RUN; isolated Go NOT RUN (Python-only change). Windows Wails/WebView2/Quality Gate/PowerShell/race NOT RUN; frontend TCP smoke NOT RUN. Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN (unavailable).

## Storage gate and critical divergence

- Exactly one local `ai-development` commit, clean tree; complete `.git` ZIP SHA/CRC/full extraction/source↔ZIP↔extracted byte identity/Git/fsck and Library upload/readback required before pointer promotion. Immutable checkpoint readback before latest pointer. ZIP SHA/HEAD in external checkpoint to avoid circular self-hash. No main/remote/tags/Releases changes. NOT a release.
- **CRITICAL:** archived `.go-version=1.26.6` not reconciled with working Windows copy after `ai-development→main` merge and Go/security updates. Never replace Windows copy, downgrade Go/security or publish.
- **Exactly one next action:** obtain authentic read-only snapshot of working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
