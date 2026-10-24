# Iris Online Database — CONTINUE NEXT RUN (iter122)

**UTC:** 2026-10-24 18:17:28 UTC. **Status:** one negative-branch-filter CI/CodeQL policy defect RED→GREEN; Linux offline QA partial. NOT a release.

## Verified input
- Selected newest Library/conversation iter121 ZIP, byte-identical, SHA-256 `de1afd831fb20ab9d90a47724d7156ad9fe09f16107fd30cd64f008bf8a954db`, 8,850,037 bytes, 561 ZIP files, one root `iris-online-database`, CRC/full extraction/byte identity PASS; CRC manifest SHA-256 `372d83f6f0010f02c066e94b30aaa5cafb51b1c8c23685b69f6d469a0bbd453a`. Latest/immutable checkpoint SHA-256 `b0a5034d12ff40020e2bcad6fe6a046fac93e35916b15d0148daf11d8c66e9a0`, byte-identical.
- Git input `ai-development` HEAD `95872002daa152d99c588359b2e5a4b50b5d0b5b`, parent `950f40f72f5bb7b1499e36759e33b7e733ba0260`, clean, fsck PASS (pre-existing dangling commit). VERSION `2.0.5`, `.go-version` `1.26.6`, go.mod `1.25.0`. ARCHITECTURE/BUILD/RELEASE, IrisTools.ps1 and log present. Named engineering standard not found in accessible Library.

## One engineering iteration
- `tools/validate_workflows.py::ci_trigger_failures` accepted later negated branch patterns excluding `main` despite literal `main` entry; eight negative subcases RED before patch and GREEN after minimal guard; unchanged controls PASS. Only Python validator, regression test, log and continuation changed; no workflow, Go, UI, game data, security pins, profile or network changed.
- QA: Python **251 unittest PASS**, compileall PASS, data audit `fatal=[]`, repository audit PASS, Node check PASS, gofmt PASS, offline Chromium/Playwright navigation PASS, git diff/fsck PASS. Full Go test/vet/race BLOCKED (Go 1.23.2 < go.mod 1.25.0); isolated Go NOT RUN. Workflow static PASS but native Windows PowerShell NOT RUN, overall workflow NOT PASS. Windows Wails/WebView2/Quality Gate/race NOT RUN. TCP frontend smoke NOT RUN; security tools Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.
- Output HEAD and ZIP SHA are stored in external immutable checkpoint, not in this committed file (self-reference avoidance). Full ZIP `.git`/CRC/extraction/byte identity/Git/fsck/SHA and Library readback required before updating latest pointer. Preserve main, remote, tags, Releases.

## Critical blocker / exactly one next action
Archived `.go-version=1.26.6` NOT reconciled with working Windows copy after merge/security updates. Never overwrite Windows copy, downgrade Go/security, publish or call ZIP a release.
**Exactly one next action:** obtain authentic read-only snapshot of working Windows copy and compare Git HEAD, `.go-version`, security toolchain without replacing files.
