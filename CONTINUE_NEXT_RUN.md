# Iris Online Database — CONTINUE NEXT RUN (iter199)

**UTC:** 2026-10-29 06:14:57 UTC. **Status:** PARTIAL, NOT A RELEASE; exactly one local `ai-development` commit; no main/remote/tag changes.

**Authentic input:** Library `IrisOnlineDB_AI_2026-10-29_iter198.zip`, SHA-256 `f6e675401db5822c34f4d4a540091ace9d508c77c3518d7f60ffe88d3e76e231`, 25,316,822 bytes, 1260 ZIP files, sole root; CRC 1260/1260, safe full extraction and ZIP↔extracted byte identity PASS; sorted path/CRC32/size manifest SHA-256 `bdf9ea6aa6388f6669e26a059f08d331d7cf61bfebb03a56a20b31a9208aa5fb`. Conversation ZIP byte-identical; Library latest checkpoint matched input. Named maintenance standard not found in accessible Library; explicit engineering contract followed.

**Input Git:** `ai-development` HEAD `455dff9e94f1621b2bb54864018bb2f5fa3530a4`, parent `a4bcbe3ac286924cfd4a3a6d99f2973f4744f4aa`, clean, fsck PASS (two pre-existing dangling commits). VERSION `2.0.5`, `.go-version=1.26.6`, go.mod `1.25.0`; required docs/IrisTools.ps1 present. `main`/`origin/main` `f17d835e7443eec6d0dd5884ad3ddb53867ad214`, nine tags unchanged.

**One fix:** release EXE privacy scan accepted absolute developer paths in `file:///` URIs (including UTF-16LE). Three RED before → three GREEN after; macOS URI negative and system URI control PASS. Minimal anchored regex boundary change. Only Python verifier, regression test and docs changed; no Go/runtime/UI/game data/profile/security pins/network/telemetry/TCP changes.

**Actual QA:** Linux Python 3.13.5: 327 unittest PASS, compileall PASS, data audit `fatal=[]`, repository audit `WARN: none`, Node 22.16.0 syntax PASS, gofmt PASS, offline Chromium/Playwright monster-drop PASS, git diff --check/fsck PASS. Workflow static PASS, overall NOT PASS [WINDOWS_POWERSHELL]. Full Go test/vet/race BLOCKED (offline Go 1.23.2 < go.mod 1.25.0); isolated Go NOT RUN. Windows Wails/WebView2/Quality Gate/PowerShell/race NOT RUN; main TCP frontend smoke NOT RUN; Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.

**Output gate:** full `.git` ZIP, CRC/full extraction/source↔ZIP↔extracted byte identity/SHA/Git/fsck and independent Library ZIP/checkpoint readback before latest pointer promotion. Final ZIP SHA/HEAD in external checkpoint to avoid circular self-hash. No publication or release.

**Critical divergence:** archived `.go-version=1.26.6` NOT reconciled with working Windows copy after ai-development→main merge and Go/security updates. Never replace Windows copy or downgrade Go/security. The Original/Iris Kiss Kiss separate; `%APPDATA%\IrisOnlineDatabase\UserData\profile.json`; offline/privacy/no telemetry/no separate TCP; UI unchanged.

**Exactly one next action:** obtain an authentic read-only snapshot of the working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
