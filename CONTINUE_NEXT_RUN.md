# Iris Online Database — CONTINUE NEXT RUN (iter195)

**UTC:** 2026-10-29 02:19:52 UTC. **Status:** PARTIAL, NOT A RELEASE; one local `ai-development` commit pending final artifact gate; no main/remote/tag changes.

**Authentic input:** Library/conversation iter194 ZIP byte-identical, SHA-256 `6a63e9f348e302ef688ff02cc425a3d2a7cf7469b6fad8a7189c355671ef1241`, 24,296,625 bytes, 1224 ZIP files, one root, CRC 1224/1224, full extraction and byte identity PASS; sorted path/CRC32/size manifest SHA-256 `8f7b252eca1ffce8119b20cc119271fd9b262b5d38d0787ba9a2a83813c2734b`. Latest/immutable iter194 checkpoints byte-identical SHA-256 `7a34430747d9259c1a804d9521ac82eb0ed0afb12c8192071fb5fdff3c609e3b`. Named maintenance standard not located; supplied engineering contract applied.

**Input Git:** `ai-development` HEAD `7422306e6ce00449d6cf95848798bb58afb4f7a4`, parent `01e55cd615e3a14801232d5732ea0ea981346533`, clean, fsck PASS (two pre-existing dangling commits); VERSION `2.0.5`, `.go-version=1.26.6`, go.mod `1.25.0`; required docs, IrisTools.ps1, AI log and continuation present. `main`/`origin/main` `f17d835e7443eec6d0dd5884ad3ddb53867ad214`, nine tags unchanged.

**One fix:** Windows release version verifier previously accepted malformed `dwFileFlagsMask` values 0, 0x03, 0x3E, 0x7F. Four negative cases **4 RED before → 4 GREEN after**, canonical 0x3F PASS. Minimal mask guard and focused regression; low risk, no Go/UI/game data/profile/security pins/network changes.

**Actual QA:** Linux Python 3.13.5: 323 unittest PASS, compileall PASS, data audit `fatal=[]`, repository audit `WARN: none`, Node 22.16.0 JS syntax PASS, gofmt PASS, offline Chromium/Playwright monster-drop PASS, git diff --check/fsck PASS. Workflow static PASS, overall NOT PASS [WINDOWS_POWERSHELL]. Full Go test/vet/race BLOCKED (offline Go 1.23.2 < go.mod 1.25.0); isolated Go NOT RUN. Windows Wails/WebView2/Quality Gate/PowerShell/race NOT RUN; frontend TCP smoke NOT RUN; Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.

**Output gate:** one local commit, clean tree, complete `.git` ZIP, CRC/extraction/byte identity/SHA/Git/fsck and independent Library ZIP/checkpoint readback required before latest promotion. Final output ZIP SHA/HEAD recorded in separate verified checkpoint to avoid self-reference. No publication or release.

**Critical divergence:** archived `.go-version=1.26.6` NOT reconciled with running Windows copy after ai-development→main merge and Go/security updates. Do not replace Windows copy or downgrade Go/security. The Original/Iris Kiss Kiss separate; `%APPDATA%\IrisOnlineDatabase\UserData\profile.json`; offline/privacy/no telemetry/no separate TCP; UI unchanged.

**Exactly one next action:** obtain an authentic read-only snapshot of the working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
