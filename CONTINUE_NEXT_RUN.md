# Iris Online Database — CONTINUE NEXT RUN (iter189)

**UTC:** 2026-10-28 19:23:11 UTC. **Status:** PARTIAL, NOT A RELEASE. Exactly one local `ai-development` commit only.

**Authentic input:** newest complete Library/conversation iter188 ZIP SHA-256 `29311ddeb2fde4aa8fd2f792f3a4801a702ca90a0c5c2c569afd00049d2ac5ab`, 22,791,664 bytes, 1170 files, one root; CRC 1170/1170, full extraction and byte identity PASS. CRC manifest SHA-256 `566aac5e151289a51c12b6967ec0757b260c1512bf5dca44e702393cf09a2c2c`. Library latest checkpoint matches input SHA. Named methodology absent from accessible Library.

**Input Git:** `ai-development` HEAD `5c1a1e9013f7bd6abb2789bb209496e80bc8d3b0`, parent `cef49f5ecf6a73def5fc6d546bb7e76afba5709e`, clean, fsck PASS (two pre-existing dangling commits). VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`; architecture/build/release docs, IrisTools.ps1 and AI log present. `main`, remote refs and tags unchanged.

**One reproduced defect:** conflicting/duplicate Go build metadata and Wails dependency records masked by one valid record; seven subcases **7 RED before**, **7 GREEN after**, clean control PASS. Minimal exact-one-record validation, Python-only, no runtime/UI/game data/security pin changes.

**Actual QA:** Linux Python 3.13.5 **317 unittest PASS**, compileall PASS, data audit `fatal=[]`, repository audit `WARN: none` PASS; Node 22.16.0 JS syntax PASS; gofmt/git diff --check/fsck PASS; offline Chromium/Playwright monster-drop navigation PASS. Workflow static PASS, overall **NOT PASS [WINDOWS_POWERSHELL]**. Full Go test/vet/race **BLOCKED** (offline Linux Go 1.23.2 < go.mod 1.25.0); isolated Go NOT RUN. Windows Wails/WebView2/Quality Gate/PowerShell/race NOT RUN; frontend TCP smoke NOT RUN; Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.

**Output gate:** one complete ZIP with `.git`, local clean ai-development commit, CRC, full extraction, SHA-256, source↔ZIP↔extracted byte identity, Git/fsck, independent Library ZIP/immutable checkpoint readback before latest pointer promotion. Final ZIP SHA and HEAD recorded externally in verified checkpoint. No release/push/main/remote/tag changes.

**Critical Windows divergence:** archived `.go-version=1.26.6` NOT reconciled with working Windows copy after merge and Go/security updates. Never replace Windows copy or downgrade Go/security. Original/Kiss Kiss separate; `%APPDATA%\IrisOnlineDatabase\UserData\profile.json`, UI, offline/privacy/no telemetry/no TCP unchanged.

**Exactly one next action:** obtain an authentic read-only snapshot of the working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
