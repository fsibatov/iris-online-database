# Iris Online Database — CONTINUE NEXT RUN (iter187)

**UTC:** 2026-10-28 07:21:53 UTC. **Status:** PARTIAL, NOT A RELEASE. Exactly one local `ai-development` commit only.

**Authentic input:** newest complete Library/conversation iter186 ZIP SHA-256 `dabda1d31954e22175e412a83381db4d40a72c77a4b959fe126e9abc105b1c7b`, 22,302,437 bytes, 1152 files, one root; CRC 1152/1152, full extraction and byte identity PASS. CRC table SHA-256 `8acb29069b3d8fe56c37af024347f624fe1b28b7adbcf1dff9b42b0247f674c2`. Latest/immutable iter186 checkpoints independently materialized and byte-identical. Named methodology not located in accessible Library.

**Input Git:** `ai-development` HEAD `6da693c65a1a4dd3d16ce813b77064965234b5c9`, parent `d001bcd8737b8ec0ac6c557e6d8a18133d38411d`, clean, fsck PASS. VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`; architecture/build/release docs, IrisTools.ps1 and AI log present. `main` and `origin/main` unchanged.

**One reproduced defect:** release verifier accepted embedded Windows developer home paths with forward/mixed separators. **2 RED before**, backslash/clean controls PASS; minimal regex fix; **2 GREEN after**, controls PASS. Python verifier, one regression test, AI log and continuation only.

**Actual QA:** Linux Python 3.13.5 **315 unittest PASS**, compileall PASS, data audit `fatal=[]`, repository audit `WARN: none` PASS; Node 22.16.0 JS syntax PASS; gofmt/git diff --check PASS; offline Chromium/Playwright monster-drop navigation PASS. Workflow static PASS, overall **NOT PASS [WINDOWS_POWERSHELL]**. Full Go test/vet/race **BLOCKED** (offline Linux Go 1.23.2 < go.mod 1.25.0); isolated Go NOT RUN; Windows Wails/WebView2/Quality Gate/PowerShell/race NOT RUN; frontend TCP smoke NOT RUN; Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.

**Output gate:** one complete ZIP with `.git`, local clean ai-development commit, CRC, full extraction, SHA-256, source↔ZIP↔extracted byte identity, Git/fsck, independent Library ZIP/immutable checkpoint readback before latest pointer promotion. Final ZIP SHA and HEAD recorded externally in verified checkpoint. No release/push/main/remote/tag changes.

**Critical Windows divergence:** archived `.go-version=1.26.6` NOT reconciled with working Windows copy after merge and Go/security updates. Never replace Windows copy or downgrade Go/security. Original/Kiss Kiss separate; `%APPDATA%\IrisOnlineDatabase\UserData\profile.json`, UI, offline/privacy/no telemetry/no TCP unchanged.

**Exactly one next action:** obtain an authentic read-only snapshot of the working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
