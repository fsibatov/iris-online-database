# Iris Online Database — continuation after iter109

**UTC:** 2026-10-24 05:16:48 UTC. **Status:** one Python CI-policy defect RED→GREEN, Windows QA NOT RUN, full Go BLOCKED. NOT a release.

**Authentic input:** Library iter108, SHA-256 `e00fa4bd6d6bc9afce6b849a7f51af15ab2fd69215bf0110c148852946138580`, 6,869,107 bytes, 444 files, one root; ZIP CRC and full extraction/byte identity 444/444 PASS; CRC-table SHA-256 `d8cd88f335a979e8a2afac5f383f345a3f07e76f87a1950396b8b15efe6b542a`. Conversation/Library ZIP byte-identical; latest/immutable checkpoint iter108 byte-identical (SHA-256 `45b70fa369a37453a44a824ffa0da5faded5b30044eadc9ad3031c7eec80977e`). Input Git `ai-development` HEAD `294a9c6bf90f33c6f99bf0a9756c248ad14ee5c8`, parent `5211e89ec1b1b5ed9dad709c9edbd53ee4b23507`, clean, fsck PASS. VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`; required docs and IrisTools.ps1 checked. Methodology file not found in accessible Library.

**Single change:** CI runner count could be spoofed by decoy job: mandatory `windows-quality`, `windows-race`, or `windows-build` moved to `windows-2022` with unrelated `windows-2025` job yielded false PASS. Three RED before, three GREEN after, baseline control PASS. Bind each mandatory job ID to `windows-2025`. Low risk, no workflows/application/game data/security pins changed.

**Actual QA:** Python 238 unittest PASS, compileall PASS; data audit fatal=[] (items=13927, monsters=1342, sets=467), repository audit PASS, node --check PASS, gofmt PASS, offline Playwright monster-drop PASS, git diff --check/fsck PASS. Workflow static PASS; Windows PowerShell NOT RUN (exit 2), workflow NOT PASS. Full Go test/vet/race BLOCKED (Linux Go 1.23.2 < go.mod 1.25.0); isolated Go NOT RUN; Windows Wails/WebView2/Quality Gate/race NOT RUN. Frontend TCP smoke NOT RUN; Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.

**Storage:** one local commit `ai-development`, clean; one complete ZIP including `.git`, SHA/CRC/full extraction/byte comparison/Git fsck, Library upload/readback. External immutable and latest checkpoints record output ZIP SHA/HEAD after verification. No main/remote/tags/Releases changes. ZIP is not a release.

**Critical divergence:** archive `.go-version=1.26.6` not verified against working Windows copy after merge and Go/security updates; do not overwrite/downgrade.

**Exactly one next action:** obtain read-only snapshot of working Windows copy; compare HEAD, `.go-version`, security toolchain without replacing files.
