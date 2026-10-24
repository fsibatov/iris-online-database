# Iris Online Database — continuation after iter118

**UTC:** 2026-10-24 14:13:38 UTC. **Status:** one CodeQL runner-binding defect RED→GREEN. NOT a release. Full Go BLOCKED, Windows QA NOT RUN, workflow NOT PASS.

**Verified input:** latest Library/conversation iter117 ZIP SHA-256 `bcd87320ddb6380000ceef677a6a01f5b34b66ae4b18d7ceb9e5dbac1ffefb37`, 8,213,810 bytes, 525 entries, sole root `iris-online-database`, CRC 525/525, full extraction and byte identity 525/525 PASS, CRC manifest SHA-256 `3bfc6a1215af15a210ba6b115672d85ccd47a0addbfed334d232622db44b15ab`. Latest/immutable iter117 checkpoint byte-identical, SHA-256 `c315de99cc5066f3d6e27ebcd56e4aceb735e2316f1adbc5d78b331744c3367c`. Input Git `ai-development` HEAD `fe476bcdd55f978f3b7688ec54ee475f3df114f5`, parent `b02b608da0043dd0531b983b728f4c74b139290e`, clean, fsck PASS (pre-existing dangling commit). VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`; required docs and IrisTools.ps1 present. Named Engineering Maintenance Standard not found in accessible own Library; explicit constraints applied.

**One fix:** `tools/validate_workflows.py` now verifies that CodeQL `jobs.analyze` itself uses `windows-2025`; an unrelated pinned job can no longer mask the analyzer running on `windows-2022`. One regression **RED before** (policy falsely zero), **GREEN after**; unchanged CodeQL control PASS. Minimal Python-only change, low risk; no workflow, game data, UI, profile, Go/security pins, network or telemetry modified.

**Actual QA:** Linux Python 3.13.5, `PYTHONPATH=tools`, pycache outside source: 247 unittest PASS, compileall PASS, data audit `fatal=[]` (13927 items, 1342 monsters, 467 sets), repository audit PASS; Node 22.16.0 `node --check web/app.js` PASS, gofmt PASS, offline Chromium Playwright monster-drop PASS, Git fsck/diff check PASS. Workflow static PASS, overall NOT PASS (Windows PowerShell NOT RUN, exit 2). Full Go test/vet/race BLOCKED (Go 1.23.2 < go.mod 1.25.0, offline); isolated Go contour NOT RUN (Python-only patch). Windows Wails/WebView2/Quality Gate/race/PowerShell NOT RUN; frontend TCP smoke NOT RUN (separate server prohibited). Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN (unavailable).

**Storage gate:** exactly one local ai-development commit, clean tree, one complete ZIP with `.git`; verify SHA/CRC/full extraction/byte identity/Git/fsck; upload and independently materialize/compare ZIP and immutable checkpoint; only then advance/readback latest. Record output ZIP SHA/HEAD in external checkpoint to avoid ZIP self-reference. No main/remote/tag/Release changes. NOT a release.

**Critical Windows divergence:** archived `.go-version=1.26.6` not reconciled with working Windows copy after merge and Go/security updates. Never replace Windows copy or downgrade Go/security.

**Exactly one next action:** obtain authentic read-only snapshot of working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
