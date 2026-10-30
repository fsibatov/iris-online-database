# Iris Online Database — CONTINUE NEXT RUN (iter214)

**UTC:** 2026-10-30 19:16 UTC. **Status:** PARTIAL, NOT A RELEASE. Full ZIP output hash and Git HEAD are in the separately verified immutable Library checkpoint; no self-referential archive hash.

## Verified canonical input

- Selected newest confirmed Library full ZIP **iter213**, SHA-256 `b7f171ef5cd370e0055f0ed159ad58819d95113105de20ad7197b3d014446032`, 30,277,839 bytes, 1430 files, one root `iris-online-database`, CRC 1430/1430, safe full extraction and byte identity PASS, sorted full-path CRC/size manifest SHA `7612dcf95c35e56089d9847309efe21e61043793264ac83b59bd958f367a31cb`. Git `ai-development` HEAD `e113cffb0f76609ebb71a4b134ff48f3a18c6f60`, parent `da53c864d57e966ff6cb71a4591354e46fb55847`, clean/fsck PASS (two pre-existing dangling commits); VERSION 2.0.5, `.go-version` 1.26.6, go.mod 1.25.0; required ARCHITECTURE/BUILD/RELEASE docs, IrisTools.ps1 and AI log present.
- Primary Library `/Iris Resourses.rar` SHA `2df78a9b5d4cf659462c8c08a42b2e0e6c5463209079c046576cf5e3aa3c3565`, 2,544,832 bytes, read via libarchive. Parsed primary Kiss/Original dropn, droplist and dropw tables independently: normalized embedded data equal in all 616/563 direct monster sets, 3981/3960 drop groups, 171/155 world rules. The named original `Iris Resourses.7z` and `Iris.7z` were not found in the inspected own Library scopes; RAR equivalence to missing .7z is unproven.

## Single RED→GREEN correction

- Source monster 10052, item 144001001 in mutually exclusive groups 1005201 and 1005221 (Kiss direct line 1939, Original 1946). Selecting the item in the preview then returning and expanding both groups wrongly marked both group rows active, implying a selected source group. Playwright RED on prior HEAD, GREEN after one-condition fix in `web/app.js`: require `model.activeGroupIndex === groupIndex`. Preview selection remains highlighted, exact source links, group expansion, item positions, and return navigation remain functional. Regression in `tools/monster_drop_navigation_smoke.py`; no game data modified.

## Archive inventory and exact cursor

- Current own Library Engineering_Candidates: **237 entries = 103 full engineering ZIPs + 133 checkpoints + 1 recovery delta** (before iter214 upload). Existing verified full ZIPs: **iter186–195 and iter199–213 = 25**. **78** older full ZIPs remain unverified at content level; missing full iter196–198 and iter137–158 are not assumed reconstructible. No deletion/deduplication without SHA/byte evidence. The 196–198 recovery delta is not a full base.
- **iter186** newly independently materialized and verified: SHA `dabda1d31954e22175e412a83381db4d40a72c77a4b959fe126e9abc105b1c7b`, 22,302,437 bytes, 1152 files, CRC 1152/1152, full extraction/byte identity PASS; manifest SHA `8acb29069b3d8fe56c37af024347f624fe1b28b7adbcf1dff9b42b0247f674c2`; Git `ai-development` HEAD `6da693c65a1a4dd3d16ce813b77064965234b5c9`, parent `d001bcd8737b8ec0ac6c557e6d8a18133d38411d`, clean/fsck PASS. Immutable checkpoint SHA `910f66da669d7095ac45515c6143c1799fb72e4e1ae03cb6f1edb026261da281` agrees with ZIP. **NEXT ARCHIVE CURSOR: iter185**.

## Actual QA, limitations and output gate

- Linux Python 3.13.5, `PYTHONPATH=tools`, external `PYTHONPYCACHEPREFIX`: 342 unittest PASS; `data_presentation_audit fatal=[]`, `repository_audit PASS`, Node syntax PASS, Chromium Playwright monster drop smoke PASS, gofmt zero unformatted, git diff --check/fsck PASS. Workflow static PASS, overall **NOT PASS [WINDOWS_POWERSHELL]**. Full Go test/vet/race **BLOCKED** (offline Go 1.23.2 < go.mod 1.25.0). Windows Wails/WebView2/Quality Gate/PowerShell **NOT RUN**. Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks **NOT RUN**. No separate TCP server/telemetry, no profile changes.
- Archive Go 1.26.6/security state remains unreconciled with working Windows copy after merge ai-development→main; do not overwrite Windows, downgrade Go/security, or publish main/remote/tags/Releases. The Original and Iris Kiss Kiss remain separate.
- After one local `ai-development` commit and clean tree, create one full ZIP with `.git`; verify CRC, safe full extraction, byte identity, Git/fsck, SHA; upload ZIP and immutable checkpoint to Library, independently materialize and verify both; **only then** update and reverify mutable latest checkpoint. On failure leave latest pointer iter213 and report BLOCKED storage. Keep newest five full verified iterations and protected originals, never disable schedule for storage.

**Exactly one next action:** independently materialize and fully verify iter185 ZIP and immutable checkpoint; advance archive cursor only after success.
