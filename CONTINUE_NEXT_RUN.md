# Iris Online Database — CONTINUE NEXT RUN (iter158)

UTC: 2026-10-26 06:22 UTC. Exactly one safe local `ai-development` iteration. **NOT a release.** Output ZIP SHA-256, final HEAD and Library readback belong in independently verified `IrisOnlineDB_AI_LATEST_CHECKPOINT_ITER158.md` (avoid circular ZIP self-hash).

## Verified authentic input

- Enumerated latest conversation iter148–157 and Library iter152–157 ZIP/checkpoints. Newest complete Library iter157: SHA-256 `8bc42e21d0d8ee6b787eb0516619e0130acba2e21b2f23e467f5174871dcaccf`, 15,692,522 bytes, 891 ZIP files, sole root `iris-online-database`; CRC 891/891, full extraction, byte identity 891/891 PASS, CRC manifest SHA-256 `c516c0fa3b3ddc8c71f22e1e36f7c886fa9cfb30d91a1d3f1189d293e518237a`. Library/conversation ZIP identical. Latest/immutable iter157 checkpoint identical SHA-256 `b8632aea1bf46e06ad7cd0f283d42d59232fa0396edd0bd6e788954563e36a6e`. Named `Engineering_Maintenance_Standard_OpenAI_2026-10-22.md` not found in accessible Library; explicit engineering requirements applied.
- Input Git `ai-development` HEAD `97ad31324bd75860ed875004e4cc51ee549170c6`, parent `06c4118709164ff3b5afc12dfc43f685732accba`, clean, fsck PASS (two pre-existing dangling commits). VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`; required architecture/build/release docs, IrisTools.ps1 and AI log present. `main`/`origin/main` `f17d835e7443eec6d0dd5884ad3ddb53867ad214`; nine tags unchanged.

## Exactly one defect and measured regression

- `tools/verify_windows_resources.py::verify_target` accepted PE with declared RT_ICON (3) type and RT_GROUP_ICON (14) but missing/empty actual icon image data. New `test_windows_release_requires_nonempty_icon_image_payloads`: **2 RED before** (missing/empty) + 1 valid control PASS; minimum fail-closed nonempty RT_ICON image validation: **3/3 PASS after**. Benefit: reject a release image without usable icon data; low risk for conforming PE. No icon payload format or group reference parsing in this iteration. Only verifier, regression test, docs/AI_DEVELOPMENT_LOG.md and this continuation changed.

## Actual offline QA and limitations

- Linux Python 3.13.5, `PYTHONPATH=tools`, external `PYTHONPYCACHEPREFIX`, `PYTHONDONTWRITEBYTECODE=1`: **286 unittest PASS**, compileall PASS; data_presentation_audit `fatal=[]` PASS; repository_audit `WARN: none` PASS; Node 22.16.0 `node --check web/*.js` PASS; gofmt PASS; offline Chromium/Playwright monster-drop navigation PASS on bounded retry (first attempt timed out); git diff --check/fsck PASS.
- Workflow static PASS, native Windows PowerShell parser **NOT RUN [WINDOWS_POWERSHELL]**, workflow **NOT PASS** exit 2. Full Go test/vet/race **BLOCKED**: offline Linux Go 1.23.2 vs go.mod >=1.25.0 (`GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off`); isolated Go **NOT RUN** (Python-only change). Windows Wails/WebView2/Quality Gate/PowerShell/race **NOT RUN**; main frontend TCP smoke **NOT RUN** (separate server forbidden). Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks **NOT RUN** (unavailable).
- Output acceptance: one local `ai-development` commit and clean tree; full `.git` ZIP, CRC/full extraction/source↔ZIP↔extracted byte identity/Git/fsck/SHA; Library upload/readback and immutable checkpoint readback before latest pointer promotion. No main/remote/tags/Releases modification or publication. ZIP is not a release.

## Critical Windows divergence and exactly one next action

- Archived `.go-version=1.26.6` **NOT reconciled** with working Windows copy after `ai-development→main` merge and Go/security updates. Never replace Windows copy, downgrade Go/security or call ZIP a release. The Original/Iris Kiss Kiss separate; `%APPDATA%\IrisOnlineDatabase\UserData\profile.json`, UI/terminology, offline/privacy/no-telemetry/no-TCP invariants unchanged.
- **Exactly one next action:** obtain authentic read-only snapshot of working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
