# Iris Online Database — CONTINUE NEXT RUN (iter210)

**UTC:** 2026-10-30 15:16 UTC. **Status:** PARTIAL, NOT A RELEASE. One local `ai-development` commit only; main/remote/tags/Releases/working Windows installation untouched. Final output ZIP SHA and Git HEAD recorded in independently verified external Library checkpoint (not embedded to avoid circular hashing).

## Input and authentic chain

- Library `/Iris Online Database/Engineering_Candidates/IrisOnlineDB_AI_2026-10-30_iter209.zip` independently materialized, byte-identical to conversation ZIP. SHA-256 `3a4d8a5216794b1e2dc5c6001a2af62b4b9ed20faa4b6d7a6f1edae31924c427`, 29,045,244 bytes, 1386 regular entries, sole root `iris-online-database`, CRC 1386/1386 PASS, full safe extraction and byte identity 1386/1386 PASS. Manifest SHA `db3a96b5a4541399482306e55ec11e8a1c333b366af5c2ea1f41a1e6679f0b99`. Input Git `ai-development` HEAD `2ff0cb21fcf1374db8e03aff6576cf5770e64757`, parent `faa40444eeb8dd9575fe69811dcba0a588bd63e1`, clean/fsck PASS (two pre-existing dangling commits); VERSION 2.0.5, `.go-version` 1.26.6, go.mod 1.25.0; required docs and IrisTools.ps1 present.
- Library latest and immutable iter209 checkpoints independently materialized and SHA-identical `79fa5456fa1eebea145a6f53edcb67d711160ecf83092cdcd7ccf862ad42cce8`, 5530 bytes. `main`/`origin/main` `f17d835e7443eec6d0dd5884ad3ddb53867ad214`, nine tags unchanged.

## Inventory with precise cursor

- Accessible own Library Iris folder enumerated all 3 pages: **230 items** = **99 full engineering ZIPs**, **129 checkpoints**, one recovery delta and one folder; conversation iter200–209. Independently verified full ZIPs iter190–195 and iter199–209 = **17**; **82 other ZIPs NOT content-verified**; iter196–198 have only recovery-delta ZIP, no independently verified full snapshots.
- **Newly verified cursor iter190:** Library ZIP SHA-256 `09e7b7345c3b0865ae8103c40ee498e977a4c525d4492c1041d0c387fb46ebe5`, 23,285,557 bytes, 1188 files, sole root, CRC/full extraction/extracted byte identity PASS. Git `ai-development` HEAD `572e106bd7c3348ec0f2b2f76230d0b1a207f996`, parent `c8269efa201909ba11acfea562d5bcd603ae547d`, clean/fsck PASS; VERSION 2.0.5, Go 1.26.6; immutable checkpoint SHA agrees. **NEXT CURSOR iter189**; do not skip.
- Named original `Iris Resourses.7z`/`Iris.7z` NOT FOUND in accessible own Library/conversation. Named `Engineering_Maintenance_Standard_OpenAI_2026-10-22.md` NOT FOUND. Other root source 2.1 ZIPs/iris-ui-audit ZIP NOT fully verified and not substitutes for current complete Git base. No speculative duplicate grouping/deletion; unverified archive SHA/CRC/provenance remains UNKNOWN.

## Primary drop audit and one RED→GREEN

- Root Library `/Iris Resourses.rar` SHA-256 `2df78a9b5d4cf659462c8c08a42b2e0e6c5463209079c046576cf5e3aa3c3565`, 2,544,832 bytes; independently materialized and libarchive fully decompressed 307 regular files / 19,112,057 bytes, CRC32 307/307 PASS. Not proven identical to missing 7z. Both servers' six primary drop tables (`item_dropn`, `item_droplist`, `item_dropw`) match ordered packaged projection after omitted zero additional-attempt fields; Kiss direct monsters/groups/world 616/3981/171, Original 563/3960/155.
- **Exactly one defect:** monster 148, direct line 775, group 14801, item 835243, two distinct source positions 2/3 and quantities 1/5, source lines 10894/10895 in BOTH servers. Item source UI omitted quantity for the ×5 source, despite preserving it in monster group. New offline Playwright test RED before one-line `web/app.js` patch, GREEN after; Python primary regression PASS. Navigation item→monster exact position→item→back retains open group and active position 3. Existing Russian quantity label reused. No game data changes; game-data SHA `9ad89a2fabff0f3a8375277545ee068e093f3935d09ea746508148ca1a79004b`.

## Actual QA and blockers

- Python 3.13.5, `PYTHONPATH=tools`, no source pycache: **339 unittest PASS**, compileall PASS; data_presentation_audit fatal=[] PASS, repository_audit WARN none PASS. Node 22.16.0 `node --check web/*.js` PASS; gofmt zero unformatted; offline Chromium/Playwright monster-drop smoke PASS; git diff --check/fsck PASS.
- Workflow static PASS but overall **NOT RUN [WINDOWS_POWERSHELL]**; full Go test/vet/race **BLOCKED** (Linux Go 1.23.2 vs go.mod>=1.25, offline). Isolated Go NOT RUN; Windows Wails/WebView2/Quality Gate/PowerShell/race NOT RUN; main TCP smoke NOT RUN; Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN (unavailable). No false PASS.

## Storage and continuity

- Build exactly one complete ZIP with `.git`, clean `ai-development` new HEAD, CRC/full extraction/byte identity/SHA/fsck. Upload ZIP and immutable checkpoint to Library, independently materialize/compare both; **only after that** update and re-verify mutable latest pointer. On failure preserve iter209 pointer and report BLOCKED storage. Retain five newest full verified iterations plus protected originals, recovery chains, working Windows release; older cleanup only after proof by dedicated task. Do not pause schedule for storage.
- Critical Windows divergence: archived Go pin 1.26.6 and security toolchain NOT reconciled with working Windows copy after merge. Do not replace working copy, downgrade Go/security, call ZIP a release, or alter `%APPDATA%\IrisOnlineDatabase\UserData\profile.json`, offline/privacy/no telemetry/no separate TCP.

**Exactly one next action:** independently materialize and fully verify iter189 full ZIP and immutable checkpoint; advance inventory cursor only after validation.
