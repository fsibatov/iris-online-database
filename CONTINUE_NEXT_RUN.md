# Iris Online Database — CONTINUE NEXT RUN (iter200)

**UTC:** 2026-10-29 07:14:42 UTC. **Status:** PARTIAL, NOT A RELEASE; exactly one local `ai-development` commit; main/remote/tags unchanged.

**Authentic input:** Library `IrisOnlineDB_AI_2026-10-29_iter199.zip` (independently materialized), SHA-256 `821778cbf89c655b4faae683a3e5917710f59b5826c09eb3798b6ec2c66ca06e`, 25,573,357 bytes, 1269 ZIP files, sole root, CRC 1269/1269, full extraction and ZIP↔extracted byte identity PASS, sorted path/CRC32/size manifest SHA-256 `ca86a11a657ced160dd54c04b17f135c8802cb6405e7e5ad7d4087f1e56b6379`. Conversation ZIP SHA matched; latest checkpoint materialized. Maintenance standard not located in accessible Library.

**Input Git:** `ai-development` HEAD `49055b93342e577cce62bb66695ec72e5c98fc20`, parent `455dff9e94f1621b2bb54864018bb2f5fa3530a4`, clean, fsck PASS (two pre-existing dangling commits). VERSION `2.0.5`, `.go-version=1.26.6`, go.mod `1.25.0`; required docs/IrisTools.ps1 present. `main`/`origin/main` `f17d835e7443eec6d0dd5884ad3ddb53867ad214`, nine tags unchanged.

**One fix:** release EXE privacy gate accepted private UNC Windows user homes in ASCII/UTF-8 and UTF-16LE. Four negative fixtures 4 RED before → 4 GREEN after; two controls PASS. One anchored UNC regex; only Python verifier, regression test and docs changed. No Go/runtime/UI/game data/profile/security pins/network/telemetry/TCP changes.

**Actual QA:** Linux Python 3.13.5: 328 unittest PASS; compileall PASS; data audit `fatal=[]`; repository audit `WARN: none`; Node 22.16.0 syntax PASS; gofmt PASS; offline Chromium/Playwright monster-drop PASS; git diff --check/fsck PASS. Workflow static PASS, overall NOT RUN [WINDOWS_POWERSHELL]. Full Go test/vet/race BLOCKED (offline Go 1.23.2 < go.mod 1.25.0); isolated Go NOT RUN. Windows Wails/WebView2/Quality Gate/PowerShell/race NOT RUN; main TCP frontend smoke NOT RUN; Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.

**Output gate:** full `.git` ZIP, CRC/full extraction/source↔ZIP↔extracted byte identity/SHA/Git/fsck and independent Library ZIP/checkpoint readback before latest pointer promotion. Final ZIP SHA/HEAD in external checkpoint to avoid circular self-hash. No publication/release.

**Critical divergence:** archived `.go-version=1.26.6` NOT reconciled with working Windows copy after ai-development→main merge and Go/security updates. Never replace Windows copy or downgrade Go/security. The Original/Iris Kiss Kiss separate; `%APPDATA%\IrisOnlineDatabase\UserData\profile.json`; offline/privacy/no telemetry/no separate TCP; UI unchanged.

**Exactly one next action:** obtain authentic read-only snapshot of working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
