# Iris Online Database — CONTINUE NEXT RUN (iter159)

UTC: 2026-10-26 07:27 UTC. Exactly one local `ai-development` engineering iteration. **NOT a release.** Output ZIP SHA/HEAD and Library readback are in independently verified `IrisOnlineDB_AI_LATEST_CHECKPOINT_ITER159.md` (avoid circular ZIP hash).

## Authentic input and provenance

- Enumerated conversation iter149–158 ZIPs and Library Engineering_Candidates ZIP/checkpoints. Selected newest complete Library iter158: SHA-256 `4172f2e98210092c6c77ba5dd1b6f38ba235ff162ca592f1ffd303f5e912b518`, 15,900,733 bytes, 900 ZIP files, sole root `iris-online-database`; CRC 900/900, complete extraction, byte identity 900/900, Library↔conversation cmp PASS. CRC-manifest SHA-256 `1386ced89344c0d8f237326ce53a4e8d63524859363f52d2f6686818582afd2b`. Latest and immutable iter158 checkpoints byte-identical SHA-256 `4bbd5e9cfdd7c65aa0a3a6b5a38dc539e807feea7e7472a09f94a1239bed9702`. Named engineering standard absent from accessible Library; explicit constraints applied.
- Input Git `ai-development` HEAD `8bc5cc9c8f75eeb3b01d06f8ffafca999fbcb289`, parent `97ad31324bd75860ed875004e4cc51ee549170c6`, clean, fsck PASS (two pre-existing dangling commits). VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`; required docs/ARCHITECTURE.md, docs/BUILD.md, docs/RELEASE.md, IrisTools.ps1, docs/AI_DEVELOPMENT_LOG.md present. `main`/`origin/main` `f17d835e7443eec6d0dd5884ad3ddb53867ad214`, nine tags unchanged.

## Exactly one narrow defect

- `tools/verify_windows_resources.py` accepted invalid RT_GROUP_ICON bytes if type and RT_ICON images existed. Regression `test_windows_release_rejects_invalid_icon_group_directory`: **6 RED before** (empty, wrong type, zero count, truncated entry, zero icon ID, zero size), valid control PASS; **7/7 GREEN after**. Minimal GRPICONDIR structural validation; no actual cross-reference or icon payload decoding attempted. Only verifier, regression test, docs/AI_DEVELOPMENT_LOG.md and this continuation changed. No app, UI, game data, Go/security pins, workflow, profile, network, telemetry or Windows working copy touched.

## Actual offline QA

- Linux Python 3.13.5, `PYTHONPATH=tools`, external `PYTHONPYCACHEPREFIX`, `PYTHONDONTWRITEBYTECODE=1`: **287 unittest PASS**, compileall PASS, data_presentation_audit `fatal=[]` PASS, repository_audit `WARN: none` PASS; Node 22.16.0 `node --check web/*.js` PASS; gofmt PASS; offline Chromium/Playwright monster-drop navigation PASS on bounded retry; git diff --check/fsck PASS. One initial QA run had an unrelated pre-existing `tools/__pycache__` created by preliminary script invocation; deleted generated cache and reran all tests and repository audit successfully.
- Workflow static PASS, native Windows PowerShell parser **NOT RUN [WINDOWS_POWERSHELL]**, workflow **NOT PASS** exit 2. Full Go test/vet/race **BLOCKED**: offline Linux Go 1.23.2 vs go.mod >=1.25.0 (`GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off`). Isolated Go **NOT RUN** (Python-only patch). Windows Wails/WebView2/Quality Gate/PowerShell/race **NOT RUN**; frontend TCP smoke **NOT RUN** (separate server forbidden); Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks **NOT RUN** (unavailable).

## Output acceptance gate

- Exactly one local `ai-development` commit and clean tree; full `.git` ZIP with CRC/full extraction/source↔ZIP↔extracted byte identity/Git/fsck/SHA; Library upload/readback and immutable checkpoint readback before latest pointer promotion. Main/remote/tags/Releases unchanged, no push or publication. ZIP is not a release.

## Critical Windows divergence and exactly one next action

- Archived `.go-version=1.26.6` **NOT reconciled** with working Windows copy after `ai-development→main` merge and Go/security updates. Never replace Windows copy, downgrade Go/security or label ZIP a release. The Original/Iris Kiss Kiss separate; `%APPDATA%\IrisOnlineDatabase\UserData\profile.json`, UI/terminology, offline/privacy/no-telemetry/no-TCP invariants unchanged.
- **Exactly one next action:** obtain authentic read-only snapshot of working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
