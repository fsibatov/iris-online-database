# Iris Online Database — CONTINUE NEXT RUN (iter156)

UTC: 2026-10-26 04:22 UTC. Exactly one safe local `ai-development` iteration. **NOT a release.** Output ZIP SHA-256, final HEAD and Library readback belong in independently verified `IrisOnlineDB_AI_LATEST_CHECKPOINT_ITER156.md` (avoid circular ZIP self-hash).

## Verified authentic input

- Enumerated conversation iter146–155 ZIPs and Library iter151–155 ZIP/checkpoints; newest complete Library iter155 ZIP `IrisOnlineDB_AI_2026-10-26_iter155.zip`, SHA-256 `2c3853f750acf0904f4d3936493c074d7d9347b59b155c94ac365b201a53de6b`, 15,098,067 bytes, 868 ZIP files, sole root `iris-online-database`; CRC 868/868, full extraction and ZIP↔extracted bytes 868/868 PASS; sorted filename/tab/lowercase-CRC32/newline manifest SHA-256 `e4a3c6f9984e5bef7e7f8b43e954717023b30e7c2c5af0192824c714d9fff1be`. Library ZIP equals conversation ZIP byte-for-byte. Latest and immutable iter155 checkpoint SHA-256 `ce7154d873f1722324d7e13d58ab82e6821f2730d3151b993440d28d446db04e`, cmp PASS. Named `Engineering_Maintenance_Standard_OpenAI_2026-10-22.md` not found by available Library search; explicit engineering requirements applied.
- Input Git `ai-development` HEAD `714b94e5f9130f0c53596dbb0bc82bf6cbb0f3c7`, parent `260d7b6fd7abbc18e47b4ddf44a73483962dfef9`, clean, fsck PASS (pre-existing dangling commit `95bfa0933292843980a63d6bb4ce621909363a47`). VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`; docs/ARCHITECTURE.md, docs/BUILD.md, docs/RELEASE.md, IrisTools.ps1, AI log and continuation present. `main`/`origin/main` `f17d835e7443eec6d0dd5884ad3ddb53867ad214`; 9 tags unchanged.

## Exactly one defect and measured regression

- `tools/verify_windows_resources.py::directory_entries` accepted unsorted numeric PE resource IDs, which can disagree with Windows loader lookup. Added regression `test_resource_directory_numeric_ids_must_be_sorted`: **2 RED before**, two sorted controls PASS; minimal numeric-ID ascending check; **2 GREEN after**, controls PASS. Benefit fail-closed PE verification; low risk for conforming Windows PE. Only verifier, test, docs/AI_DEVELOPMENT_LOG.md and this continuation modified. No application, UI, game data, Go/security pins, workflows, profile, network, telemetry, separate TCP server or Windows working copy modified.

## Actual offline QA and limitations

- Linux Python 3.13.5, `PYTHONPATH=tools`, external `PYTHONPYCACHEPREFIX`, `PYTHONDONTWRITEBYTECODE=1`: **284 unittest PASS**, compileall PASS; data_presentation_audit `fatal=[]` PASS; repository_audit `WARN: none` PASS; Node 22.16.0 `node --check web/*.js` PASS; gofmt PASS; offline Chromium/Playwright monster-drop navigation PASS; git diff --check/fsck PASS.
- Workflow static PASS, native Windows PowerShell parser **NOT RUN [WINDOWS_POWERSHELL]**, workflow **NOT PASS** exit 2. Full Go test/vet/race **BLOCKED**: offline Go 1.23.2 vs go.mod >=1.25.0 (`GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off`). Isolated Go **NOT RUN** (Python-only change). Windows Wails/WebView2/Quality Gate/PowerShell/race **NOT RUN**; main frontend TCP smoke **NOT RUN** (separate server prohibited); Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks **NOT RUN** (unavailable).
- Output acceptance: exactly one local `ai-development` commit and clean tree; full `.git` ZIP with CRC/full extraction/source↔ZIP↔extracted byte identity/Git/fsck/SHA-256; Library upload/readback and immutable checkpoint materialization/byte verification before latest pointer promotion. Output SHA/HEAD in external immutable checkpoint only. Main, remote, tags, Releases untouched; no publication.

## Critical Windows divergence and one next action

- Archived `.go-version=1.26.6` **NOT reconciled** with working Windows copy after `ai-development→main` merge and Go/security updates. Never replace Windows copy, downgrade Go/security or call ZIP a release. The Original/Iris Kiss Kiss remain separate; `%APPDATA%\IrisOnlineDatabase\UserData\profile.json`, UI/terminology, offline/privacy/no-telemetry/no-TCP invariants unchanged.
- **Exactly one next action:** obtain authentic read-only snapshot of working Windows copy and compare Git HEAD, `.go-version` and security toolchain without replacing files.
