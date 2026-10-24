# Iris Online Database — continuation after iter111

**UTC:** 2026-10-24 07:13:19 UTC. **Status:** one narrow CodeQL security-gate defect RED→GREEN; full Go BLOCKED; Windows QA NOT RUN; workflow NOT PASS. NOT a release.

**Authentic input:** latest confirmed Library iter110, SHA-256 `23a84f0a46ef80e6e27e3123e4e7eac71b1b1d9719450fbc3722d9e00be1df7a`, 7,156,905 bytes, 462 files, one root; ZIP CRC and full extraction/byte identity 462/462 PASS; sorted CRC table SHA-256 `85164194c56065eacf747c5ac447266c54ec592d033c7a7444491ade6249021b`; conversation/Library ZIP byte-identical. Latest/immutable iter110 checkpoints byte-identical SHA-256 `43c39b8e625c62d5d251d00af902ea9325363f2f14b733f4c6f255854edbb6db`. Input Git `ai-development`, HEAD `1f5dd1e143ab2f69e4501db008db11c963ddc367`, parent `8bddcf082e23a6e83e5dff3c590ab4ee12b0f10b`, clean tree and fsck PASS; VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`; required docs and IrisTools.ps1 verified. Requested maintenance standard absent from accessible Library; explicit constraints followed.

**One change:** CodeQL security policy accepted missing/commented/replaced/disabled `github/codeql-action/analyze` action, 4 negative subcases RED before and 4 GREEN after; unmodified CodeQL control PASS. Validator now requires exactly one parsed SHA-pinned unconditional analyze action. Low risk; only Python validator/test/log/continuation changed, no workflows, application, game data, UI, profile, TCP/network, telemetry or security pins changed.

**Actual QA:** Linux Python 3.13.5: 240 unittest PASS, compileall PASS, data audit fatal=[] (items 13,927, monsters 1,342, sets 467), repository audit PASS, node --check PASS, gofmt PASS, offline Playwright monster-drop PASS, git diff --check/fsck PASS. Workflow static PASS but native PowerShell NOT RUN (exit 2), workflow overall NOT PASS. Full Go test/vet/race BLOCKED (Go 1.23.2 < go.mod 1.25.0, offline); isolated Go NOT RUN. Windows Wails/WebView2/Quality Gate/race NOT RUN; frontend TCP smoke NOT RUN; Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.

**Storage:** one local `ai-development` commit and clean tree; ZIP with `.git` SHA/CRC/full extraction/byte identity/Git/fsck must pass, then Library upload/readback/cmp and separate immutable checkpoint readback before updating latest pointer. External checkpoint contains output ZIP SHA/HEAD (avoids self-reference). Main, remote, tags and Releases unchanged; not a release.

**Critical Windows divergence:** archive `.go-version=1.26.6` not compared with working Windows copy after merge and Go/security updates. Never overwrite/downgrade.

**Exactly one next action:** obtain authentic read-only Windows working-copy snapshot; compare HEAD, `.go-version`, security toolchain without replacing files.
