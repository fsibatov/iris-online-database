# Iris Online Database — CONTINUE NEXT RUN (iter197)

**UTC:** 2026-10-29 04:16:40 UTC. **Status:** PARTIAL, NOT A RELEASE; one local `ai-development` commit; no main/remote/tag changes.

**Authentic input:** Library `IrisOnlineDB_AI_2026-10-29_iter196.zip`, SHA-256 `4f478824fe2b822d168025829fdf098c02b20673b2ebfb47f957988e0fc0c964`, 24,806,485 bytes, 1242 ZIP files, sole root; CRC 1242/1242, safe full extraction and byte identity PASS; sorted path/CRC32/size manifest SHA-256 `2ee66fd0c3eaeffaf6360ef5d91657f7cb6b3f51304404d445157ddf20eca4d4`. Conversation and Library ZIP bytes identical; latest and immutable iter196 checkpoints identical (5148 bytes). Named maintenance standard not found in accessible Library; explicit engineering contract applied.

**Input Git:** `ai-development` HEAD `d78c8c531331dda3a22ad2671ce2043545401984`, parent `6a88bf68d4a2eb6b99470804df1fea1321d2c183`, clean, fsck PASS (two pre-existing dangling commits). VERSION `2.0.5`, `.go-version=1.26.6`, go.mod `1.25.0`; required docs/IrisTools.ps1 present. `main`/`origin/main` `f17d835e7443eec6d0dd5884ad3ddb53867ad214`, nine tags unchanged.

**One fix:** `tools/verify_executables.py` privacy scanner accepted Unix absolute developer paths after tab, quote or `=`. Three negative cases **3 RED before → 3 GREEN after**, two clean controls PASS. Minimal delimiter-class update and one regression test; no runtime, game data, profile, security pins, UI, network, telemetry or TCP changes.

**Actual QA:** Linux Python 3.13.5: 325 unittest PASS, compileall PASS, data audit `fatal=[]`, repository audit `WARN: none`, Node 22.16.0 syntax PASS, gofmt PASS, offline Chromium/Playwright monster-drop PASS, git diff --check/fsck PASS. Workflow static PASS, overall NOT PASS [WINDOWS_POWERSHELL]. Full Go test/vet/race BLOCKED (offline Go 1.23.2 < go.mod 1.25.0); isolated Go NOT RUN. Windows Wails/WebView2/Quality Gate/PowerShell/race NOT RUN; main TCP frontend smoke NOT RUN; Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.

**Output gate:** complete `.git` ZIP, CRC/full extraction/source↔ZIP↔extracted byte identity/SHA/Git/fsck and independent Library ZIP/checkpoint readback required before latest pointer promotion. Final ZIP SHA/HEAD in external checkpoint to avoid circular self-hash. No publication or release.

**Critical divergence:** archived `.go-version=1.26.6` NOT reconciled with running Windows copy after ai-development→main merge and Go/security updates. Never replace Windows copy or downgrade Go/security. The Original/Iris Kiss Kiss separate; `%APPDATA%\IrisOnlineDatabase\UserData\profile.json`; offline/privacy/no telemetry/no separate TCP; UI unchanged.

**Exactly one next action:** obtain an authentic read-only snapshot of the working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
