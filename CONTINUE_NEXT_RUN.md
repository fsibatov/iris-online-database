# Iris Online Database — CONTINUE NEXT RUN (iter196)

**UTC:** 2026-10-29 03:18:15 UTC. **Status:** PARTIAL, NOT A RELEASE; exactly one local `ai-development` commit pending final archive gate; no main/remote/tag changes.

**Authentic input:** Library iter195 ZIP SHA-256 `b64bee62e8d5d7f6206c14efb1adaed074e0bcb801ade519f5207c054efa960b`, 24,552,889 bytes, 1233 ZIP files, sole root; CRC 1233/1233, full extraction and byte identity PASS; sorted path/CRC32/size manifest SHA-256 `0cf75e38ebc085648e117c27da96b00dc710ff96102f845e116c29a395c9d549`. Latest and immutable iter195 checkpoints identical SHA-256 `d38fd3c22b00ffac97925d975d69dea6d339fecb2c6871adb643789f0828544c`. Named maintenance standard not located; supplied contract followed.

**Input Git:** `ai-development` HEAD `6a88bf68d4a2eb6b99470804df1fea1321d2c183`, parent `7422306e6ce00449d6cf95848798bb58afb4f7a4`, clean, fsck PASS (two existing dangling commits). VERSION `2.0.5`, `.go-version=1.26.6`, go.mod `1.25.0`; required docs/IrisTools.ps1 present. `main`/`origin/main` `f17d835e7443eec6d0dd5884ad3ddb53867ad214`, nine tags unchanged.

**One fix:** Linux `/root/` developer paths in Windows release EXE were accepted. Three negative cases **3 RED before → 3 GREEN after**, two clean controls PASS. Minimal anchored regex addition; no application runtime, game data, profile, security pin, network or UI changes.

**Actual QA:** Linux Python 3.13.5: 324 unittest PASS (clean rerun after removing transient generated pycache), compileall PASS, data audit `fatal=[]`, repository audit `WARN: none`, Node 22.16.0 JS syntax PASS, gofmt PASS, offline Chromium/Playwright monster-drop PASS, git diff --check/fsck PASS. Workflow static PASS, overall NOT PASS [WINDOWS_POWERSHELL]. Full Go test/vet/race BLOCKED (offline Go 1.23.2 < go.mod 1.25.0); isolated Go NOT RUN. Windows Wails/WebView2/Quality Gate/PowerShell/race NOT RUN; main TCP frontend smoke NOT RUN; Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.

**Output gate:** one local commit, clean tree, complete `.git` ZIP, CRC/full extraction/byte identity/SHA/Git/fsck and Library ZIP/checkpoint readback required before latest promotion. Final output SHA/HEAD in external checkpoint to avoid circular self-hash. No publication or release.

**Critical divergence:** archived `.go-version=1.26.6` NOT reconciled with running Windows copy after ai-development→main merge and Go/security updates. Never replace Windows copy or downgrade Go/security. The Original/Iris Kiss Kiss separate; `%APPDATA%\IrisOnlineDatabase\UserData\profile.json`; offline/privacy/no telemetry/no separate TCP; UI unchanged.

**Exactly one next action:** obtain an authentic read-only snapshot of the working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
