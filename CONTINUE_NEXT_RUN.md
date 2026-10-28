# Iris Online Database — CONTINUE NEXT RUN (iter190)

**UTC:** 2026-10-28 20:12:57 UTC. **Status:** PARTIAL, NOT A RELEASE. Exactly one local `ai-development` commit only.

**Authentic input:** newest complete Library/conversation iter189 ZIP SHA-256 `32e11b5e1acfecf0e752cbafff8f7b9f6c27c769197654d8d89b4f698aeeb6fc`, 23,037,979 bytes, 1179 files, one root; CRC 1179/1179, full extraction and byte identity PASS. CRC manifest SHA-256 `82c92f81fb7c8c10d565edf84dbbd0e639a58288db860a42e454ccae79a7eb6e`. Library latest checkpoint matches input SHA. Named methodology absent from accessible Library.

**Input Git:** `ai-development` HEAD `c8269efa201909ba11acfea562d5bcd603ae547d`, parent `5c1a1e9013f7bd6abb2789bb209496e80bc8d3b0`, clean, fsck PASS (two pre-existing dangling commits). VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`; architecture/build/release docs, IrisTools.ps1 and AI log present. `main`, remote refs and nine tags unchanged.

**One reproduced defect:** Wails `go version -m` dependency replacement `=>` bypassed pinned Wails version check; **3 RED before, 3 GREEN after**, valid and unrelated replacement controls PASS. Minimal fail-closed replacement detection, Python-only, no runtime/UI/game data/security pin changes.

**Actual QA:** Linux Python 3.13.5 **318 unittest PASS**, compileall PASS, data audit `fatal=[]`, repository audit `WARN: none` PASS; Node 22.16.0 JS syntax PASS; gofmt/git diff --check/fsck PASS; offline Chromium/Playwright monster-drop navigation PASS. Workflow static PASS, overall **NOT PASS [WINDOWS_POWERSHELL]**. Full Go test/vet/race **BLOCKED** (offline Linux Go 1.23.2 < go.mod 1.25.0); isolated Go NOT RUN. Windows Wails/WebView2/Quality Gate/PowerShell/race NOT RUN; frontend TCP smoke NOT RUN; Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.

**Output gate:** one complete ZIP with `.git`, local clean ai-development commit, CRC, full extraction, SHA-256, source↔ZIP↔extracted byte identity, Git/fsck, independent Library ZIP/immutable checkpoint readback before latest pointer promotion. Final ZIP SHA and HEAD recorded externally in verified checkpoint. No release/push/main/remote/tag changes.

**Critical Windows divergence:** archived `.go-version=1.26.6` NOT reconciled with working Windows copy after merge and Go/security updates. Never replace Windows copy or downgrade Go/security. Original/Kiss Kiss separate; `%APPDATA%\IrisOnlineDatabase\UserData\profile.json`, UI, offline/privacy/no telemetry/no TCP unchanged.

**Exactly one next action:** obtain an authentic read-only snapshot of the working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
