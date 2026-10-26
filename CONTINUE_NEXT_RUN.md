# Iris Online Database — CONTINUE NEXT RUN (iter164)

UTC: 2026-10-26 12:21:40 UTC. Exactly one local engineering iteration on `ai-development`. **NOT a release.** Final ZIP SHA/HEAD and independent Library verification are in external immutable `IrisOnlineDB_AI_LATEST_CHECKPOINT_ITER164.md` (avoid circular ZIP hash).

## Authentic input

Library `/Iris Online Database/Engineering_Candidates/IrisOnlineDB_AI_2026-10-26_iter163.zip`, SHA-256 `e9ff1c2b012e4c4cdb6fd04726279153e40d5bf6574b38c9f2951e621e82f137`, 16,971,308 bytes, 945 files, sole root `iris-online-database`; CRC 945/945, complete extraction and ZIP↔source byte identity 945/945 PASS; conversation↔Library ZIP cmp PASS. Filename/tab/CRC32 manifest SHA-256 `16da336d76cb7778664d67af545ac24b1e2887d7238f4c5c37ea1420e24e3b5c`. Latest/immutable iter163 checkpoints byte-identical SHA-256 `e5907368856858eb5a66b2433032f29940a268ddca20ef8089317304c24edc76`.

Input Git `ai-development` HEAD `3bb561813fb3464bd0c3b0fe8a3e7dc55b855e4e`, parent `1a48a3e3dbd882ddb16093c4599490d7b4bfa7b9`, clean, fsck PASS (two pre-existing dangling commits); VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`. Required architecture/build/release docs, `scripts/windows/IrisTools.ps1`, AI log present/tracked. Main and origin/main `f17d835e7443eec6d0dd5884ad3ddb53867ad214`, nine tags and remote refs unchanged.

## One narrow fix, RED → GREEN

`RT_GROUP_ICON` previously accepted declared image sizes differing from actual `RT_ICON` resource data lengths. Synthetic PE regression **2 RED before**, valid control PASS; after new numeric ID→payload-size extraction and group size check **3 GREEN**, focused ×100 PASS. Cross-language variants accepted if any actual size matches; no product assets, runtime, UI, game data or security pins changed.

## Actual offline QA

Linux Python 3.13.5 (`PYTHONPATH=tools`, pycache external): **292 unittest PASS**, compileall PASS, data_presentation_audit `fatal=[]` PASS, repository_audit `WARN: none` PASS. Node 22.16.0 `node --check web/app.js` PASS; gofmt PASS; git diff --check/fsck PASS; offline Chromium/Playwright monster-drop navigation PASS. Workflow static PASS, full workflow **NOT PASS** [WINDOWS_POWERSHELL], exit 2. Full Go test/vet/race **BLOCKED**: Go 1.23.2 < go.mod 1.25.0; isolated Go **NOT RUN** (not full gate). Windows Wails/WebView2/Quality Gate/PowerShell **NOT RUN**; frontend TCP smoke **NOT RUN** (separate TCP server prohibited). Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks **NOT RUN** (unavailable).

## Preservation

Commit once locally on `ai-development`, clean tree; verify full `.git` ZIP SHA/CRC/full extraction/byte identity/Git/fsck; upload to Library, rematerialize and byte-compare ZIP and immutable checkpoint before latest-pointer promotion. Methodology file `Engineering_Maintenance_Standard_OpenAI_2026-10-22.md` not located in accessible Library; explicit constraints followed. Do not change main, remote, tags, Releases, Windows working copy, Go/security pins, The Original/Iris Kiss Kiss separation, offline/privacy/no-telemetry/no-TCP profile `%APPDATA%\IrisOnlineDatabase\UserData\profile.json`. Archived `.go-version=1.26.6` remains unreconciled with Windows working copy after merge and security updates.

**Exactly one next action:** obtain an authentic read-only snapshot of the working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
