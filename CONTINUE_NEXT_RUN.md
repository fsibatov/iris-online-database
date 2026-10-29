# Iris Online Database — CONTINUE NEXT RUN (iter193)

**UTC:** 2026-10-29 00:13:42 UTC. **Status:** PARTIAL, NOT A RELEASE; exactly one local `ai-development` commit, no main/remote/tag changes.

**Authentic input:** latest complete Library `IrisOnlineDB_AI_2026-10-28_iter192.zip` SHA-256 `1b4b5103be3b2a7f98f0b58a0c630b5446825be1057fdd927c178c9d31683a96`, 23,786,293 bytes, 1206 ZIP files, one root `iris-online-database`, CRC 1206/1206 PASS, full extraction and byte identity PASS; sorted path/CRC32/size manifest SHA-256 `7105486349142219d5c9acf04f863c4b5e8075aa64f86a892a75a4b7782a1f3c`. Latest/immutable iter192 checkpoints byte-identical SHA-256 `9a53f2b6ed47a0806b315bf2b3a37a5cf5941a86c32a3865cc229819be669383`. Named maintenance standard not found in accessible Library; supplied engineering contract applied.

**Input Git:** `ai-development` HEAD `e72eda851f980c422aab70c27be3c059551ab8df`, parent `d728f2357973881360fd7f22df0467775a383f2a`, clean, fsck PASS (two existing dangling commits); VERSION `2.0.5`, `.go-version=1.26.6`, go.mod `1.25.0`; ARCHITECTURE/BUILD/RELEASE docs, IrisTools.ps1, AI log, continuation present. `main`/`origin/main` `f17d835e7443eec6d0dd5884ad3ddb53867ad214`, nine tags unchanged.

**One fix:** release manifest verifier accepted malformed DPI fallback while primary mode was `permonitorv2`. Three negative cases **3 RED before → 3 GREEN after**, canonical control PASS. Minimal strict two-mode validation and one regression test; low risk, unchanged canonical manifest, runtime/UI/game data/profile/security pins/network unchanged.

**Actual QA:** Linux Python 3.13.5: 321 unittest PASS, compileall PASS, data audit `fatal=[]`, repository audit `WARN: none`, Node 22.16.0 JS syntax PASS, gofmt PASS, offline Chromium/Playwright monster-drop PASS, git diff --check/fsck PASS. Workflow static PASS, overall NOT PASS [WINDOWS_POWERSHELL]. Full Go test/vet/race BLOCKED (offline Go 1.23.2 < go.mod 1.25.0); isolated Go NOT RUN. Windows Wails/WebView2/Quality Gate/PowerShell/race NOT RUN; frontend TCP smoke NOT RUN; Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.

**Output gate:** one local commit, clean tree, complete `.git` ZIP, CRC/extraction/byte identity/SHA/Git/fsck and independent Library ZIP/checkpoint readback required before latest promotion. Final output ZIP SHA/HEAD recorded in separate verified checkpoint to avoid self-reference. No publication or release.

**Critical divergence:** archived `.go-version=1.26.6` NOT reconciled with running Windows copy after ai-development→main merge and Go/security updates. Do not replace Windows copy or downgrade Go/security. The Original/Iris Kiss Kiss separate; `%APPDATA%\IrisOnlineDatabase\UserData\profile.json`; offline/privacy/no telemetry/no separate TCP; UI unchanged.

**Exactly one next action:** obtain an authentic read-only snapshot of the working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
