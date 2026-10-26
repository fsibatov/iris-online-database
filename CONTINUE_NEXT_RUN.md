# Iris Online Database — CONTINUE NEXT RUN (iter157)

UTC: 2026-10-26 05:12–05:14 UTC. Exactly one safe local `ai-development` iteration. **NOT a release.** Output ZIP SHA-256, final HEAD and Library readback are recorded in independently verified `IrisOnlineDB_AI_LATEST_CHECKPOINT_ITER157.md` (avoids circular ZIP self-hash).

## Verified authentic input

- Enumerated conversation iter147–156 ZIPs and Library iter151–156 ZIP/checkpoints. Newest complete Library iter156 `IrisOnlineDB_AI_2026-10-26_iter156.zip`: SHA-256 `cc464ce4f37d12d48ab5d37c0e1e4f41048d6e5f69b0204918009bf29c6ac9fe`, 15,304,661 bytes, 877 ZIP files, sole root `iris-online-database`; CRC 877/877, full extraction and byte identity 877/877 PASS; sorted filename/tab/lowercase-CRC32/newline manifest SHA-256 `6025f1c3f7ac6fb3348fd335f0c7b4a680c6ea0e688a267b05492abc2fc27c0b`. Library ZIP byte-identical to conversation iter156. Latest and immutable iter156 checkpoints byte-identical. Named `Engineering_Maintenance_Standard_OpenAI_2026-10-22.md` not found in accessible Library; explicit engineering constraints applied.
- Input Git `ai-development` HEAD `06c4118709164ff3b5afc12dfc43f685732accba`, parent `714b94e5f9130f0c53596dbb0bc82bf6cbb0f3c7`, clean, fsck PASS (pre-existing dangling commit `95bfa0933292843980a63d6bb4ce621909363a47`). VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`; docs/ARCHITECTURE.md, docs/BUILD.md, docs/RELEASE.md, IrisTools.ps1, docs/AI_DEVELOPMENT_LOG.md and this continuation present. `main`/`origin/main` `f17d835e7443eec6d0dd5884ad3ddb53867ad214`; nine tags unchanged.

## Exactly one defect and measured regression

- `tools/verify_windows_resources.py::directory_entries` accepted unsorted PE resource names, risking missed Windows resource lookup. Added `test_resource_directory_named_entries_must_be_sorted`: **4 RED before** (ASCII, mixed, Unicode, prefix), four positive controls PASS; minimal ascending UTF-16LE code-unit name-order check; **8/8 PASS after**. Existing duplicate/bounds/numeric-ID validation unchanged. Benefit fail-closed PE validation; low risk for conforming PE. Only verifier, test, docs/AI_DEVELOPMENT_LOG.md and this continuation modified. No application, UI, game data, Go/security pins, workflows, profile, network, telemetry, separate TCP server or Windows working copy modified.

## Actual offline QA and limitations

- Linux Python 3.13.5, `PYTHONPATH=tools`, external `PYTHONPYCACHEPREFIX`, `PYTHONDONTWRITEBYTECODE=1`: **285 unittest PASS**, compileall PASS; data_presentation_audit `fatal=[]` PASS; repository_audit `WARN: none` PASS; Node 22.16.0 `node --check web/*.js` PASS; gofmt PASS; offline Chromium/Playwright monster-drop navigation PASS; git diff --check/fsck PASS.
- Workflow static PASS, native Windows PowerShell parser **NOT RUN [WINDOWS_POWERSHELL]**, overall workflow **NOT PASS** exit 2. Full Go test/vet/race **BLOCKED**: offline Go 1.23.2 vs go.mod >=1.25.0 (`GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off`). Isolated Go **NOT RUN** (Python-only change). Windows Wails/WebView2/Quality Gate/PowerShell/race **NOT RUN**; main frontend TCP smoke **NOT RUN** (separate server forbidden); Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks **NOT RUN** (unavailable).
- Output acceptance: exactly one local `ai-development` commit and clean tree; full `.git` ZIP with CRC/full extraction/source↔ZIP↔extracted byte identity/Git/fsck/SHA-256; Library upload/readback and immutable checkpoint materialization/byte verification before latest pointer promotion. Output SHA/HEAD in external immutable checkpoint only. Main, remote, tags, Releases untouched; no publication.

## Critical Windows divergence and one next action

- Archived `.go-version=1.26.6` **NOT reconciled** with working Windows copy after `ai-development→main` merge and Go/security updates. Never replace Windows copy, downgrade Go/security or call ZIP a release. The Original/Iris Kiss Kiss remain separate; `%APPDATA%\IrisOnlineDatabase\UserData\profile.json`, UI/terminology, offline/privacy/no-telemetry/no-TCP invariants unchanged.
- **Exactly one next action:** obtain authentic read-only snapshot of working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
