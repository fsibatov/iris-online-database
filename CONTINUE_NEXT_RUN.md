# Iris Online Database — CONTINUE NEXT RUN (iter151)

UTC: 2026-10-25 23:22:43 UTC. Status: exactly one local `ai-development` engineering iteration; not a Windows release. Final output ZIP SHA, CRC manifest, commit and Library readback must be recorded in external `IrisOnlineDB_AI_LATEST_CHECKPOINT_ITER151.md` after verification, not embedded here (avoids self-hash).

## Authentic input

- Selected newest complete Library ZIP `/Iris Online Database/Engineering_Candidates/IrisOnlineDB_AI_2026-10-25_iter150.zip`, byte-identical to conversation iter150. SHA-256 `e2f15f908f04ae2128ddc6395d2ed569abd1ad9c3b4ff05c383eec38d194de4c`; size 14,094,434; ZIP 823 entries; single root `iris-online-database`; CRC 823/823 PASS; full extraction and source↔ZIP byte identity 823/823 PASS; sorted filename/tab/lowercase-CRC32/newline manifest SHA-256 `93d791bdf70daf5616f7f64daf0d3bcceb39b16c2e74b3ceabfae7e0910e12bb`.
- Input Git branch `ai-development`, HEAD `fd6f57045f7cfdf6e3af04e6e5efd6d3563f8138`, parent `78a45f07c5b13e4f6aae223028a168ae72659b77`, clean, fsck PASS (pre-existing dangling commit `95bfa0933292843980a63d6bb4ce621909363a47`). VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`. Architecture/build/release docs, IrisTools.ps1, AI development log present. Main/origin-main `f17d835e7443eec6d0dd5884ad3ddb53867ad214`, remote refs/tags untouched.
- Latest and immutable iter150 checkpoints SHA `1e8cc82e4ee07b2af49c24fa15019023e2fc4ddc6d5aca4218690d3c76c1df19`. Named Library maintenance standard not found in accessible search; explicit engineering constraints followed.

## Exactly one change and proof

- `directory_entries` accepted duplicate PE resource directory type/ID keys, allowing first-match/set logic to conceal ambiguous resources. Two synthetic duplicate-key negative cases **RED before**, **GREEN after** minimal fail-closed guard; distinct-key control PASS both times. Changed `tools/verify_windows_resources.py`, `tests/test_release_helpers.py`, log and this continuation only. No runtime, game data, security pins, UI, profile, telemetry, network, workflow or working Windows files modified.

## Actual QA and blockers

- Python 3.13.5 unittest **279 PASS** (`PYTHONPATH=tools`, pycache outside source); compileall PASS; data audit fatal=[]; repository audit PASS; Node 22.16.0 check PASS; gofmt PASS; offline Chromium/Playwright monster-drop navigation PASS; Git fsck/diff-check PASS.
- Workflow static PASS; native Windows PowerShell parser NOT RUN; overall workflow NOT PASS (exit 2).
- Full Go test/vet/race BLOCKED (offline Go 1.23.2 vs go.mod >=1.25.0); isolated Go NOT RUN (Python-only patch). Windows Wails/WebView2/Quality Gate/race NOT RUN. Main frontend TCP smoke NOT RUN (no separate TCP server). Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN (unavailable).
- Critical unresolved divergence: archived `.go-version=1.26.6` has not been compared to working Windows copy after ai-development→main merge and Go/security updates. Never replace working copy, downgrade Go/security or label archive release. The Original and Iris Kiss Kiss remain separate; `%APPDATA%\IrisOnlineDatabase\UserData\profile.json` and UI/terminology preserved.

## Exactly one next action

Obtain an authentic read-only snapshot of the working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
