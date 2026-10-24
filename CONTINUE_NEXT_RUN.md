# Iris Online Database — CONTINUE NEXT RUN (iter121)

**UTC:** 2026-10-24 17:19:50 UTC. **Status:** one PR-event-filter policy defect RED→GREEN; Linux offline QA partial. Not a release.

## Input attestation
- Newest confirmed full Library/conversation `IrisOnlineDB_AI_2026-10-24_iter120.zip`, SHA-256 `d2ba10e0db336eb54096d9b1b1314c9fb841b729d843cbcef5dcb74ac81e35a6`, 8,689,622 bytes, 552 files, sole root, CRC/full extraction/byte identity PASS; sorted CRC manifest SHA-256 `f9e46332c5dae3dd97735bbb698cbd112bbb9cf0d2e6c88707ea2415208817a0`. Immutable/latest checkpoint SHA-256 `b7a5586c5d60afa2c2ef34d74d7995bdbf8b22b491e6aafff496ca08f36e889e`, byte-identical.
- Git input `ai-development` HEAD `950f40f72f5bb7b1499e36759e33b7e733ba0260`, parent `05ad90b2cd0b5a2aa4b6dd9b4446d5024f1a4063`, clean, fsck exit 0 (pre-existing dangling commit). VERSION `2.0.5`; `.go-version` `1.26.6`; go.mod `1.25.0`. Required docs/IrisTools.ps1/log present. Named engineering standard absent from accessible Library.

## One iteration
- `tools/validate_workflows.py::ci_trigger_failures` accepted PR event `types` filters that skip required CI/CodeQL on updated commits. Six negative subcases RED before patch, all GREEN after minimal guard; unchanged workflow controls PASS. Only Python validator, one regression test, log and continuation changed. No Go/UI/game data/security pins/network/profile modified.
- Actual QA: 250 Python unittest PASS, compileall PASS, data audit fatal=[], repository audit PASS, Node syntax PASS, gofmt PASS, offline Chromium/Playwright navigation PASS, Git diff/fsck PASS. Full Go test/vet/race BLOCKED (Linux Go 1.23.2 < go.mod 1.25.0); isolated Go NOT RUN. Workflow static PASS, native Windows PowerShell NOT RUN, overall workflow NOT PASS. Windows Wails/WebView2/Quality Gate/race NOT RUN; Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN. No separate TCP server/telemetry.
- Commit/ZIP/Library/checkpoint integrity must be verified before pointer update. Output ZIP SHA and HEAD recorded in external checkpoint to avoid circular ZIP hash.

## Critical blocker / exactly one next action
Archived `.go-version=1.26.6` is NOT reconciled with working Windows copy after merge/security upgrades. Never overwrite Windows working copy, downgrade Go/security, publish or call archive a release.
**Exactly one next action:** obtain authentic read-only snapshot of working Windows copy and compare Git HEAD, `.go-version`, security toolchain without replacing files.
