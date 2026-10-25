# Iris Online Database — CONTINUE NEXT RUN (iter140)

**UTC:** 2026-10-25 12:14:31 UTC. **Status:** one local `ai-development` engineering iteration; full Go BLOCKED, Windows QA NOT RUN, workflow NOT PASS. **NOT a release.**

## Authentic source and integrity

- Enumerated conversation iter130–139 and Library ZIP/checkpoints first. Selected latest complete iter139 Library/conversation ZIP, byte-identical SHA-256 `20a919c274c34ccb400b43f0494abb92e6f868c969d4a16d8974b714c98aa061`, 11,988,434 bytes, 723 files, sole `iris-online-database` root, CRC 723/723, full extraction/byte identity 723/723 PASS. Sorted filename/tab/CRC32 manifest SHA-256 `ab49451fbcdc56e70e876347e2d73abf97b1cfcba2debf99ee3e8d06138725f0`. Latest/immutable iter139 checkpoints byte-identical SHA-256 `4ea449c0b48984afd06eb60db7bafc84e7de18f2a341a1691dfa8a0e65cc6ace`. Named methodology file not found in accessible Library; explicit engineering requirements followed.
- Input Git `ai-development` HEAD `02500f39ef2e44716fdc9915036b1e9c0c4bfea2`, parent `b8c13c733a247c61ed5907664aecee5f035dc4e6`, clean, fsck PASS (pre-existing dangling commit). VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`; ARCHITECTURE/BUILD/RELEASE, IrisTools.ps1, AI log and continuation present. Main/origin-main `f17d835e7443eec6d0dd5884ad3ddb53867ad214`, nine tags preserved.

## Exactly one fix and actual QA

- CodeQL release policy incorrectly accepted `$LASTEXITCODE` captured before Go test/build, producing stale status and potential false-green scan. **2 RED before → 2 GREEN after**, unchanged workflow control PASS; minimal adjacent active-line check added in `tools/validate_workflows.py` and parameterized regression in `tests/test_workflow_platform_gate.py`. Low risk, no workflow YAML or app/UI/game data/Go/security changes. Static heuristic is not native PowerShell execution.
- Offline Linux Python 3.13.5 with `PYTHONPATH=tools`, pycache outside source: **268 unittest PASS**, compileall PASS, data audit `fatal=[]` PASS, repository audit `WARN: none` PASS, Node 22.16.0 `node --check web/*.js` PASS, gofmt zero unformatted, offline Chromium/Playwright monster-drop navigation PASS, Git diff --check/fsck PASS. Workflow static PASS; Windows parser **NOT RUN [WINDOWS_POWERSHELL]**, overall workflow **NOT PASS** (exit 2). Full Go test BLOCKED: Linux Go 1.23.2 < go.mod 1.25.0, offline; vet/race NOT RUN; isolated Go NOT RUN (Python-only patch). Windows Wails/WebView2/Quality Gate/PowerShell/race NOT RUN; frontend TCP smoke NOT RUN. Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN (unavailable).

## Storage gate and critical divergence

- Exactly one local `ai-development` commit and clean tree required; create one complete `.git` ZIP with CRC/full extraction/source↔ZIP↔extracted byte identity/Git/fsck/SHA, upload/read back from Library, then upload/read back immutable checkpoint before advancing latest pointer. ZIP SHA and final HEAD belong in external checkpoint (no circular self-hash). Never modify main, remote refs, tags, Releases or publish. NOT a release.
- **CRITICAL:** archived `.go-version=1.26.6` not reconciled with working Windows copy after merge `ai-development→main` and Go/security updates. Do not replace Windows copy, downgrade Go/security or publish.
- **Exactly one next action:** obtain authentic read-only snapshot of working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
