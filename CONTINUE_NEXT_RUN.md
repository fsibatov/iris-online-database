# Iris Online Database — final release candidate handoff (2026-11-18)

**Status:** PRE-RELEASE / NOT PUBLISHED. Do not push, tag or release automatically.

## Provenance

- User-provided Git bundle `Iris_Final_Review_e39bc82.bundle` verified as complete.
- Canonical Windows `main`: `36e1513c6672a1b3d44a7d114430999702498048` (version 2.0.6).
- Candidate review branch: `ai/iris206-consolidated-review`, `e39bc8297986bd2611ec0be49f0cf36e419e9d6d`, built atop that Windows main.
- Previous review commits: `473a007` (drop/VK/recent), `5cc5a03` (BG/placeholder), `8d8d972` (storage/final notes), `e39bc82` (Go regression alignment).
- This handoff updates **release version metadata** to `2.1.0`; no gameplay database or other game data changed by this handoff.

## Verified in isolated Linux review on the exact e39bc82 candidate

- Python unittest: 346/346 PASS (2026-11-18).
- Node JavaScript regression scripts: 5/5 PASS; `repository_audit.py` PASS; `data_presentation_audit.py` reported `fatal=[]`.
- On the user's Windows review branch at `e39bc82`: `go test ./...` PASS after the storage test fix; JavaScript regressions PASS.
- Full Windows `PREPARE RELEASE`, Wails/WebView2 visual tests, Windows x64/x86/arm64 builds, static and security scanners, reproducibility, CI, and publication rights review: **NOT VERIFIED FOR 2.1.0**.
- Historical Library engineering patches remain candidates, not an assertion that every historical idea was merged.

## Required next step

1. Apply this release-metadata patch to the clean review branch at `e39bc82`.
2. Recheck all unit/regression/audit tests on Windows, then commit the metadata-only changes on the review branch.
3. After auditing differences and source/game-data publication rights, integrate that verified branch into the canonical Windows `main` by safe fast-forward.
4. Run `IrisTools.ps1 -Action Prepare` **only from canonical Windows main** and complete every strict release gate before Publish and Release.
5. Verify the final GitHub release notes include the diplomatic final-development notice already in `CHANGELOG.md`, with no promise of ongoing maintenance.
