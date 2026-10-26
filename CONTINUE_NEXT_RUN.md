# Iris Online Database — CONTINUE NEXT RUN (iter161)

UTC: 2026-10-26 09:26 UTC. One local engineering iteration on `ai-development`. **NOT a release**. Output ZIP SHA/HEAD and Library readback belong to external immutable `IrisOnlineDB_AI_LATEST_CHECKPOINT_ITER161.md` to avoid a cyclic ZIP hash.

## Authentic source and input integrity

- First enumerated current conversation ZIP iter151–160 and Library Engineering_Candidates ZIP/checkpoints; selected newest complete Library iter160, SHA-256 `70ef0d8e4ec75f0b4c709a400820428d1b1bd119ba8822fd90594cd3cce1ee1c`, 16,323,832 bytes, 918 files, sole root `iris-online-database`; CRC 918/918, full extraction and byte identity 918/918 PASS; conversation↔Library `cmp` PASS. CRC manifest SHA-256 `3bb89e844ecf014d8d97da9f14e59f548e9ae26d777652c4d245d0e3b5a16aef`; latest/immutable iter160 checkpoints byte-identical SHA-256 `00e86495742c68c22cccca81a33fb2fa1cbf0bcfc48090bb63bbae88099e543c`. Named methodology file not found in accessible Library.
- Input Git `ai-development` HEAD `4956d040dae91f3eee8269ec572577e5d022db74`, parent `5a6fe2c9764f18d8dafe41310c2469c1a3a048f6`, clean, fsck PASS (two pre-existing dangling commits). VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`. Required docs/ARCHITECTURE.md, docs/BUILD.md, docs/RELEASE.md, scripts/windows/IrisTools.ps1, docs/AI_DEVELOPMENT_LOG.md present. `main`/`origin/main=f17d835e7443eec6d0dd5884ad3ddb53867ad214`, nine tags unchanged.

## One narrow fix, RED→GREEN

- `tools/verify_windows_resources.py::resource_payloads` previously accepted `IMAGE_RESOURCE_DATA_ENTRY.Reserved != 0`. New synthetic PE directory test: two negative cases RED before (`1`, `0xFFFFFFFF`), valid zero-reserved control PASS; minimal fail-closed check, all three GREEN after. Low risk for conforming PE resources. Only verifier, test, log, continuation changed. No runtime, game data, UI, security pins, workflow or Windows copy changes.

## Actual QA

- Python 3.13.5 `PYTHONPATH=tools`, external pycache: **289 unittest PASS**, compileall PASS, data audit `fatal=[]` PASS, repository audit `WARN: none` PASS; Node 22.16.0 `node --check web/app.js` PASS; gofmt PASS; offline Chromium/Playwright monster-drop navigation PASS on retry (initial command timed out); git diff --check/fsck PASS.
- Workflow static PASS, Windows PowerShell parser NOT RUN, workflow overall NOT PASS exit 2. Full Go test/vet/race BLOCKED: offline Linux Go 1.23.2 < go.mod 1.25.0; isolated Go NOT RUN (Python-only). Windows Wails/WebView2/Quality Gate/PowerShell/race NOT RUN; frontend TCP smoke NOT RUN (no separate TCP server). Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN (unavailable).

## Preservation and next action

- Commit exactly once locally to `ai-development`; clean tree, complete `.git` ZIP, CRC/full extraction/source↔ZIP↔extracted byte identity/Git/fsck/SHA; upload and independently read back ZIP and immutable checkpoint before updating latest pointer. Not a release; no changes to main, remote, tags, Releases. The Original/Iris Kiss Kiss remain separate. `%APPDATA%\IrisOnlineDatabase\UserData\profile.json`, offline/privacy/no telemetry/no TCP unchanged.
- Critical divergence: archive `.go-version=1.26.6` NOT reconciled with working Windows copy after ai-development→main merge and Go/security updates; never replace working Windows copy, downgrade Go/security or label ZIP release.
- **Exactly one next action:** obtain authentic read-only snapshot of working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
