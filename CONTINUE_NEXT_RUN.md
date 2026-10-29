# Iris Online Database — CONTINUE NEXT RUN (iter198)

**UTC:** 2026-10-29 05:22:26 UTC. **Status:** PARTIAL, NOT A RELEASE; exactly one local `ai-development` commit; no main/remote/tag changes.

**Authentic input:** Library `IrisOnlineDB_AI_2026-10-29_iter197.zip`, SHA-256 `c51db8d68af3e53bf5d2d3e533640a6edf449ed0ec7bb35b677e99798034e8fc`, 25,061,084 bytes, 1251 ZIP files, sole root; CRC 1251/1251, safe full extraction and ZIP↔extracted byte identity PASS; sorted path/CRC32/size manifest SHA-256 `cc631bf664aba367ff2c2bd23ef8a0d5126575944b6f2d34552d062494df9ee2`. Conversation ZIP byte-identical; Library latest checkpoint matched input. Named maintenance standard not found in accessible Library; explicit contract followed.

**Input Git:** `ai-development` HEAD `a4bcbe3ac286924cfd4a3a6d99f2973f4744f4aa`, parent `d78c8c531331dda3a22ad2671ce2043545401984`, clean, fsck PASS (two pre-existing dangling commits). VERSION `2.0.5`, `.go-version=1.26.6`, go.mod `1.25.0`; required docs/IrisTools.ps1 present. `main`/`origin/main` `f17d835e7443eec6d0dd5884ad3ddb53867ad214`, nine tags unchanged.

**One fix:** UTF-16LE encoded developer paths in Windows release EXE escaped privacy scan; 3 RED before → 3 GREEN after, one clean system-path control PASS. Minimal two-alignment wide-string scan reusing original path patterns. Only Python verifier, regression test and docs changed; no Go/runtime/UI/game data/profile/security pins/network/telemetry/TCP changes.

**Actual QA:** Linux Python 3.13.5: 326 unittest PASS, compileall PASS, data audit `fatal=[]`, repository audit `WARN: none`, Node 22.16.0 syntax PASS, gofmt PASS, offline Chromium/Playwright monster-drop PASS, git diff --check/fsck PASS. Workflow static PASS, overall NOT PASS [WINDOWS_POWERSHELL]. Full Go test/vet/race BLOCKED (offline Go 1.23.2 < go.mod 1.25.0); isolated Go NOT RUN. Windows Wails/WebView2/Quality Gate/PowerShell/race NOT RUN; main TCP frontend smoke NOT RUN; Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.

**Output gate:** full `.git` ZIP, CRC/full extraction/source↔ZIP↔extracted byte identity/SHA/Git/fsck and independent Library ZIP/checkpoint readback before latest pointer promotion. Final ZIP SHA/HEAD in external checkpoint to avoid circular self-hash. No publication or release.

**Critical divergence:** archived `.go-version=1.26.6` NOT reconciled with working Windows copy after ai-development→main merge and Go/security updates. Never replace Windows copy or downgrade Go/security. The Original/Iris Kiss Kiss separate; `%APPDATA%\IrisOnlineDatabase\UserData\profile.json`; offline/privacy/no telemetry/no separate TCP; UI unchanged.

**Exactly one next action:** obtain an authentic read-only snapshot of the working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
