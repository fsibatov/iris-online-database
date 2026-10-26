# Iris Online Database — CONTINUE NEXT RUN (iter152)

UTC: 2026-10-26 00:13:36 UTC. Status: one local `ai-development` engineering iteration; **not a Windows release**. Final output ZIP SHA/CRC/commit/Library readback must be recorded in external immutable `IrisOnlineDB_AI_LATEST_CHECKPOINT_ITER152.md` after verification, not embedded here (avoids circular hash).

## Authentic input

- Selected newest complete Library ZIP `/Iris Online Database/Engineering_Candidates/IrisOnlineDB_AI_2026-10-25_iter151.zip`, byte-identical to conversation iter151. SHA-256 `3cf2e0007e11645dddd9ee0e7802c0d62b6ab71c5639335a823edff9d7d2137f`, size 14,292,524; 832 ZIP entries; sole root `iris-online-database`; CRC 832/832, full extraction and ZIP↔source byte identity 832/832 PASS; sorted filename/tab/lowercase-CRC32/newline manifest SHA-256 `53618f36032ec291db907a4b9b59e4e78a184a1260eafc251123659e06d5fe32`. Latest and immutable iter151 checkpoint copies byte-identical SHA-256 `4a92578b540aa7efda00b6179612f9a4ad25ac9b0555e10d6b4ea1a398bdb898`. Named Library methodology not located; explicit engineering constraints applied.
- Input Git branch `ai-development` HEAD `54ce49a18cffc158d148c3d3282a81e9438216c0`, parent `fd6f57045f7cfdf6e3af04e6e5efd6d3563f8138`, clean, fsck PASS (pre-existing dangling commit `95bfa0933292843980a63d6bb4ce621909363a47`). VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`. Architecture/build/release docs, IrisTools.ps1, AI log, continuation present. Main/origin-main `f17d835e7443eec6d0dd5884ad3ddb53867ad214`, nine tags, remote refs untouched.

## Exactly one change and proof

- `directory_entries` accepted malformed named/ID slot assignments despite PE resource-directory header counts. Three synthetic negative cases **RED before**, **GREEN after** minimal fail-closed high-bit/ordering validation; three valid controls PASS before/after. Only verifier, regression test, log and continuation changed. No runtime/game data/security pins/workflows/UI/profile/telemetry/network/Windows working copy changed.

## Actual QA and blockers

- Python 3.13.5 unittest **280 PASS** (`PYTHONPATH=tools`, pycache outside source); compileall PASS; data audit `fatal=[]`; repository audit `WARN: none`; Node 22.16.0 check PASS; gofmt PASS; offline Chromium/Playwright monster-drop navigation PASS; Git fsck/diff-check PASS.
- Workflow static PASS; Windows PowerShell parser NOT RUN; overall workflow NOT PASS (exit 2).
- Full Go test/vet/race BLOCKED (offline Go 1.23.2 vs go.mod>=1.25.0); isolated Go NOT RUN (Python-only patch). Windows Wails/WebView2/Quality Gate/race NOT RUN. Main frontend TCP smoke NOT RUN (no separate TCP server). Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN (unavailable).
- Critical unresolved divergence: archive `.go-version=1.26.6` not compared with working Windows copy after ai-development→main merge and Go/security updates. Never replace working copy, downgrade Go/security or call archive a release. The Original and Iris Kiss Kiss separate; `%APPDATA%\IrisOnlineDatabase\UserData\profile.json` and UI/terminology preserved.

## Exactly one next action

Obtain an authentic read-only snapshot of the working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
