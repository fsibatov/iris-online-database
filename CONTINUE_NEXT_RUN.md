# Iris Online Database — CONTINUE NEXT RUN (iter201)

**UTC:** 2026-10-29 08:22:51 UTC. **Status:** PARTIAL, NOT A RELEASE; exactly one local `ai-development` commit; main/remote/tags unchanged.

**Authentic input:** Library `IrisOnlineDB_AI_2026-10-29_iter200.zip` independently materialized, SHA-256 `22790ebd727a5e3a9d183144a95a871367ad3ffcadb1f74dbafcdb84162300ee`, 25,834,545 bytes, 1278 ZIP files, one root, CRC 1278/1278, full extraction and ZIP↔extracted byte identity PASS, sorted path/CRC32/size manifest SHA-256 `ef08f8e00a593f2d62d0b73ca5bf40f6ed739d8ab0e5717c0ba7af26051d824e`. Conversation ZIP byte-identical; latest checkpoint independently materialized. Maintenance standard not located in accessible Library.

**Input Git:** `ai-development` HEAD `b3568ec728d7ab65266e48224090eb4e4aa39d6e`, parent `49055b93342e577cce62bb66695ec72e5c98fc20`, clean, fsck PASS (two pre-existing dangling commits). VERSION `2.0.5`, `.go-version=1.26.6`, go.mod `1.25.0`; required docs/IrisTools.ps1 present. `main`/`origin/main` `f17d835e7443eec6d0dd5884ad3ddb53867ad214`, nine tags unchanged.

**One fix:** release EXE privacy gate accepted Windows legacy `Documents and Settings` user profiles in drive/UNC/UTF-16LE paths. Four negative fixtures 4 RED before → 4 GREEN after; two controls PASS. Two anchored regex patterns, only Python verifier, regression test and docs changed. No Go/runtime/UI/game data/profile/security pins/network/telemetry/TCP changes.

**Actual QA:** Linux Python 3.13.5: 329 unittest PASS; compileall PASS; data audit `fatal=[]`; repository audit `WARN: none`; Node 22.16.0 syntax PASS; gofmt PASS; offline Chromium/Playwright monster-drop PASS; git diff --check/fsck PASS. Workflow static PASS, overall NOT PASS [WINDOWS_POWERSHELL]. Full Go test/vet/race BLOCKED (offline Go 1.23.2 < go.mod 1.25.0); isolated Go NOT RUN. Windows Wails/WebView2/Quality Gate/PowerShell/race NOT RUN; main TCP frontend smoke NOT RUN; Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.

**Output gate:** full `.git` ZIP, CRC/full extraction/source↔ZIP↔extracted byte identity/SHA/Git/fsck and independent Library ZIP/checkpoint readback before latest pointer promotion. Final ZIP SHA/HEAD in external checkpoint to avoid circular self-hash. No publication/release.

**Critical divergence:** archived `.go-version=1.26.6` NOT reconciled with working Windows copy after ai-development→main merge and Go/security updates. Never replace Windows copy or downgrade Go/security. The Original/Iris Kiss Kiss separate; `%APPDATA%\IrisOnlineDatabase\UserData\profile.json`; offline/privacy/no telemetry/no separate TCP; UI unchanged.

**Exactly one next action:** obtain authentic read-only snapshot of working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
