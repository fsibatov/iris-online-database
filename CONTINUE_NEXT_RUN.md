# Iris Online Database — CONTINUE NEXT RUN (iter119)

**UTC:** 2026-10-24 15:13:51 UTC. **Status:** one CodeQL job-level error-propagation defect reproduced and patched; Linux offline QA partial. Not a release.

## Input attestation
- Source: Library/conversation `IrisOnlineDB_AI_2026-10-24_iter118.zip`, SHA-256 `96149564fd85f26f621c64f6fb107e4949611dd6b47b156dacb4a92a89d4d92e`, size 8,369,989, 534 files, one root, CRC/full extraction/byte identity PASS. CRC manifest SHA-256 `a8c99361910f81dd89659be918d6fcca02d85dce5f2fd65a5937ea19478f8ce3`. Checkpoint iter118/latest SHA-256 `2b67699ac6c15634ade50878b09f3b5e57599536aa603adf0b90b2670a2f8f1b`, identical.
- Git input `ai-development` HEAD `fbdfe68864a5de69e1b81565bb801c06303383be`, parent `fe476bcdd55f978f3b7688ec54ee475f3df114f5`, clean, fsck PASS (one pre-existing dangling commit). VERSION `2.0.5`; `.go-version` `1.26.6`; go.mod `1.25.0`. Required architecture/build/release docs, IrisTools.ps1, AI log present. Named OpenAI maintenance standard unavailable in accessible own Library.

## One iteration
- Defect: `jobs.analyze.continue-on-error` masks failed Go/Python CodeQL scans while workflow policy reported 0 failures. Regression 3 RED before patch, 3 GREEN after patch; unchanged workflow control PASS. Minimal YAML-structure check added; no workflow/application/game-data changes.
- Actual QA: 248 Python unittest PASS, compileall PASS, data audit fatal=[], repository audit PASS, Node syntax PASS, gofmt PASS, offline Chromium/Playwright navigation PASS, Git diff/fsck PASS. Go test/vet/race BLOCKED (Linux Go 1.23.2 < go.mod 1.25.0); isolated Go contour NOT RUN. Workflow static PASS, Windows PowerShell NOT RUN; overall workflow NOT PASS. Windows Wails/WebView2/Quality Gate/race NOT RUN; security tools Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN. No separate TCP server or telemetry.
- Output must be verified after local commit, then full ZIP with `.git`, Library readback and separate immutable/latest checkpoint readback. ZIP SHA and output Git HEAD belong in external checkpoint. Do not advance pointer until every check passes.

## Critical blocker / next action
Archived `.go-version=1.26.6` is NOT reconciled with the running Windows working copy after merge/security upgrades; do not replace Windows copy, downgrade security/Go, publish or call archive a release.
**Exactly one next action:** obtain authentic read-only snapshot of Windows working copy and compare Git HEAD, `.go-version`, and security toolchain.
