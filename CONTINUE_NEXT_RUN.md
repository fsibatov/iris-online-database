# Iris Online Database — CONTINUE NEXT RUN (iter124)

**UTC:** 2026-10-24 20:15:58 UTC. **Status:** one CI gate command substitution defect reproduced RED→GREEN; partial offline Linux QA. **NOT a release.**

## Authentic input
- Selected newest verified complete Library/Conversation iter123 ZIP; SHA-256 `acd7eae05bbe6bafdca38c5c14e6631cf119a431ea440dc18729fb70b372ce40`; 9,177,169 bytes; 579 entries; sole root `iris-online-database`; CRC 579/579, full extraction and byte identity 579/579 PASS; CRC-manifest SHA-256 `22c30fa3629a5a207081bae1de43319e19996486f868a501902314b74320bf2d`; conversation/library cmp PASS.
- Input Git `ai-development` HEAD `44f0c4c746db1c0a317b079fa980caab9f293530`, parent `7ea34754eff8b51279821264f5039976aff4357a`, clean, fsck exit 0 (pre-existing dangling commit). VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`; architecture/build/release docs, IrisTools.ps1, AI log and continuation present. Named engineering methodology MD not found in accessible own Library; explicit constraints followed.

## Exactly one defect and QA
- Windows mandatory CI steps were accepted when `run` was replaced with no-op and original commands survived only as YAML comments. Three cases RED before (3 failed assertions; control PASS). Minimal parsed-step command/shell/exit-guard validation added; 3 GREEN after; control PASS. No actual workflow, Go app, UI, data, profile, remote or network changed. Benefit: fail-closed CI gate. Risk: intended strictness on future legitimate command changes.
- Offline Linux: Python 3.13.5 unittest **253 PASS**, compileall PASS, data audit fatal=[], repository audit PASS, node --check PASS, gofmt PASS, offline Playwright Chromium monster-drop smoke PASS, Git diff --check/fsck PASS. Workflow static PASS but overall NOT PASS: native Windows PowerShell parser NOT RUN (exit 2). Full Go test/vet/race BLOCKED: Go 1.23.2 < go.mod 1.25; isolated Go NOT RUN (Python-only). Windows Wails/WebView2/quality/race/PowerShell NOT RUN; TCP frontend smoke NOT RUN (no separate server). Staticcheck, govulncheck, Ruff, Bandit, pip-audit, Gitleaks NOT RUN (unavailable).

## Persistence, divergence, next action
- Commit locally to `ai-development`, require clean tree; ZIP must include `.git`, new HEAD, log, continuation; CRC/full extraction/byte-identity/Git/fsck/SHA; upload and independent Library readback, plus immutable and latest checkpoints readback. Never advance latest on failure. Final ZIP SHA/Git/output CRC-manifest and Library IDs go into external checkpoint, not self-referential ZIP.
- Critical Windows divergence remains: archived `.go-version=1.26.6` not reconciled with live Windows copy after ai-development→main merge and Go/security updates. Never overwrite Windows, downgrade security/toolchain, or call archive a release.
- **Exactly one next action:** obtain read-only snapshot of live Windows copy and compare Git HEAD, Go and security toolchain without replacing files.
