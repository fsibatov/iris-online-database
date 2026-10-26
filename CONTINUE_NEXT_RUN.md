# Iris Online Database — CONTINUE NEXT RUN (iter154)

UTC: 2026-10-26 02:14:49 UTC. One local `ai-development` engineering iteration, **not a Windows release**. Output ZIP SHA/CRC/commit/Library readback recorded in external immutable `IrisOnlineDB_AI_LATEST_CHECKPOINT_ITER154.md` after verification (avoid circular self-hash).

## Authentic input

- Enumerated conversation ZIPs iter144–153 and Library Engineering_Candidates ZIP/checkpoints. Selected newest full Library `IrisOnlineDB_AI_2026-10-26_iter153.zip`, byte-identical to conversation attachment; SHA-256 `d93b93c5fd1ada63687fcb86cc8601b7a78bf1b9a2dda3157ee76406538a03c9`, size 14,692,151 bytes, 850 ZIP entries; sole root `iris-online-database`; CRC 850/850, full extraction and ZIP↔source identity 850/850 PASS; CRC manifest SHA-256 `77f98863294b3f37f3af00487cdfd6dbf91eddffb0caf0146b3d3bac9526bf52`. Latest and immutable iter153 checkpoint copies byte-identical SHA-256 `442bf23f69e1fcd4891d335c812f492d280f045e910b54062a6c84f7d79d4b2d`. Named Library methodology not located; explicit safety constraints applied.
- Input Git `ai-development` HEAD `87ba54a683596f2729422c78075890e081d07262`, parent `522b69f7e770ef4ff060ec95958231d9c440f657`, clean, fsck PASS (pre-existing dangling commit `95bfa0933292843980a63d6bb4ce621909363a47`). VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`. Architecture/build/release docs, IrisTools.ps1, AI log and continuation present. Main/origin-main `f17d835e7443eec6d0dd5884ad3ddb53867ad214`, nine tags and remote refs untouched.

## Exactly one change and proof

- Named PE resource directory entries with name offsets/UTF-16 length outside `.rsrc` previously accepted: three malformed cases RED before → GREEN after minimal fail-closed bounds checks; two valid controls PASS both. Existing named fixture corrected to valid structure. No app, UI, game data, Go/security pins, workflows, profile, telemetry, network, Windows working copy or separate TCP server changed.

## Actual QA and blockers

- Python 3.13.5 unittest **282 PASS** (`PYTHONPATH=tools`, external pycache), compileall PASS; data audit `fatal=[]`; repository audit `WARN: none`; Node 22.16.0 check PASS; gofmt PASS; offline Chromium/Playwright monster-drop navigation PASS; Git fsck/diff-check PASS. One initial hygiene FAIL caused by untracked pycache from preliminary help invocation; removed generated cache, rerun full suite PASS.
- Workflow static PASS; Windows PowerShell parser NOT RUN; overall workflow NOT PASS (exit 2).
- Full Go test/vet/race BLOCKED (offline Go 1.23.2 vs go.mod >=1.25.0); isolated Go NOT RUN (Python-only patch). Windows Wails/WebView2/Quality Gate/race NOT RUN. Main frontend TCP smoke NOT RUN (no separate TCP server). Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN (unavailable).
- Critical unresolved divergence: archived `.go-version=1.26.6` not compared with working Windows copy after ai-development→main merge and Go/security updates. Never replace working copy, downgrade Go/security or call archive a release. The Original and Iris Kiss Kiss separate; `%APPDATA%\IrisOnlineDatabase\UserData\profile.json`, UI/terminology preserved.

## Exactly one next action

Obtain authentic read-only snapshot of working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
