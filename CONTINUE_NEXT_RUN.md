# Iris Online Database — CONTINUE NEXT RUN (iter150)

UTC: 2026-10-25 22:17 UTC. Status: one local ai-development engineering iteration; not a Windows release. Final output ZIP SHA, CRC manifest, commit and Library readback are recorded in external `IrisOnlineDB_AI_LATEST_CHECKPOINT_ITER150.md` after complete verification, not embedded here (avoids self-hash).

## Authentic input

- Selected newest complete Library ZIP `/Iris Online Database/Engineering_Candidates/IrisOnlineDB_AI_2026-10-25_iter149.zip`, byte-identical to conversation iter149. SHA-256 `c310e018d18da0d4ad0e3a1123c94c1d78e1f5c9b6dbcbbae8931d698d962f80`; size 13,897,716; ZIP 814 files; single root `iris-online-database`; CRC 814/814 PASS; full extraction and byte identity 814/814 PASS; sorted filename/tab/lowercase-CRC32/newline manifest SHA-256 `36cfab510044b0d22171a62d4cd1ce3558316e3a4d84256d04a434531e6d1a1a`.
- Input Git branch ai-development, HEAD `78a45f07c5b13e4f6aae223028a168ae72659b77`, parent `45f65377bafca92f09cfadb22580acf9aa153eaf`, clean, fsck PASS (pre-existing dangling commit `95bfa0933292843980a63d6bb4ce621909363a47`). VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`. Architecture/build/release docs, IrisTools.ps1, AI development log present. Main/origin-main `f17d835e7443eec6d0dd5884ad3ddb53867ad214`, remote refs/tags untouched.
- Immutable iter149 checkpoint SHA `7c43aac6749f462e44d801d9fe4712618f543cb11a27865df193bcc1001830ec`. Generic latest pointer was iter148, not authoritative. Named Library maintenance standard not located; user-specified safety rules applied.

## Exactly one change and proof

- `parse_pe` previously overwrote duplicate PE section names; could validate decoy `.rsrc` content. Two negative synthetic PE section cases RED before, GREEN after minimal duplicate-name rejection; distinct section names PASS both before/after. Changed `tools/verify_windows_resources.py`, `tests/test_release_helpers.py`, log and this checkpoint only. No runtime, game data, security pins, UI, profile, telemetry, network, workflow or working Windows files modified.

## Actual QA and blockers

- Python 3.13.5 unittest 278 PASS (`PYTHONPATH=tools`, pycache outside source); compileall PASS; data audit fatal=[]; repository audit PASS; Node 22.16.0 check PASS; gofmt PASS; offline Chromium/Playwright monster-drop navigation PASS after initial timeout; Git fsck/diff-check PASS.
- Workflow static PASS; native Windows PowerShell parser NOT RUN; overall workflow NOT PASS (exit 2).
- Full Go test/vet/race BLOCKED (offline Go 1.23.2 vs go.mod >=1.25.0); isolated Go NOT RUN (Python-only change). Windows Wails/WebView2/Quality Gate/race NOT RUN. Main frontend TCP smoke NOT RUN (no separate TCP server). Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN (unavailable).
- Critical unresolved divergence: archived `.go-version=1.26.6` has not been compared to working Windows copy after ai-development→main merge and Go/security updates. Never replace working copy, downgrade Go/security or label archive release. The Original and Iris Kiss Kiss remain separate; `%APPDATA%\IrisOnlineDatabase\UserData\profile.json` and UI/terminology preserved.

## Exactly one next action

Obtain an authentic read-only snapshot of the working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
