# Iris Online Database — CONTINUE NEXT RUN (iter153)

UTC: 2026-10-26 01:19:43 UTC. One local `ai-development` engineering iteration, **not a Windows release**. Output ZIP SHA/CRC/commit/Library readback belong in external immutable `IrisOnlineDB_AI_LATEST_CHECKPOINT_ITER153.md` after verification (avoid circular self-hash).

## Authentic input

- Enumerated fresh conversation iter143–152 ZIPs and Library Engineering_Candidates ZIP/checkpoints. Selected newest complete Library `IrisOnlineDB_AI_2026-10-26_iter152.zip`; SHA-256 `6e33bd7c7ff1e87e601a4b9362663fa4bc89227216f787bde686e5eccb0f7194`, size 14,491,897 bytes, 841 ZIP entries; sole root `iris-online-database`; CRC 841/841, full extraction and ZIP↔source byte identity 841/841 PASS; sorted filename/tab/lowercase-CRC32/newline manifest SHA-256 `627dcecc6f5931aed047885084b5e573aea0cfebbfe49fde447db65d81e273e0`. Latest and immutable iter152 checkpoint byte-identical SHA-256 `180d9675f2ac3247fae1f3ba981b4228efd0e967568494f8024060b27a94304e`. Named Library methodology not located; explicit safety requirements applied.
- Input Git `ai-development` HEAD `522b69f7e770ef4ff060ec95958231d9c440f657`, parent `54ce49a18cffc158d148c3d3282a81e9438216c0`, clean, fsck PASS (pre-existing dangling commit `95bfa0933292843980a63d6bb4ce621909363a47`). VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`. Architecture/build/release docs, IrisTools.ps1, AI log and continuation present. Main/origin-main `f17d835e7443eec6d0dd5884ad3ddb53867ad214`, nine tags, remote refs untouched.

## Exactly one change and proof

- `directory_entries` accepted numeric PE resource IDs with nonzero reserved high bits. Two synthetic invalid IDs **RED before**, **GREEN after** minimal fail-closed range validation; two valid controls PASS before/after. No app, UI, game data, Go/security pins, workflows, profile, telemetry, network, Windows working copy or separate TCP server changed.

## Actual QA and blockers

- Python 3.13.5 unittest **281 PASS** (`PYTHONPATH=tools`, external pycache); compileall PASS; data audit `fatal=[]`; repository audit `WARN: none`; Node 22.16.0 check PASS; gofmt PASS; offline Chromium/Playwright monster-drop navigation PASS; Git fsck/diff-check PASS.
- Workflow static PASS; Windows PowerShell parser NOT RUN; overall workflow NOT PASS (exit 2).
- Full Go test/vet/race BLOCKED (offline Go 1.23.2 vs go.mod >=1.25.0); isolated Go NOT RUN (Python-only patch). Windows Wails/WebView2/Quality Gate/race NOT RUN. Main frontend TCP smoke NOT RUN (no separate TCP server). Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN (unavailable).
- Critical unresolved divergence: archived `.go-version=1.26.6` not compared with working Windows copy after ai-development→main merge and Go/security updates. Never replace working copy, downgrade Go/security or call archive a release. The Original and Iris Kiss Kiss separate; `%APPDATA%\IrisOnlineDatabase\UserData\profile.json`, UI/terminology preserved.

## Exactly one next action

Obtain an authentic read-only snapshot of the working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
