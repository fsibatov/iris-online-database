$ErrorActionPreference = "Stop"

$Root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
Push-Location $Root
try {
    $ExpectedBranch = "agent/windows-only-release-pipeline"
    $Branch = (& git branch --show-current | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $Branch -ne $ExpectedBranch) {
        throw "Run this migration only on branch $ExpectedBranch."
    }

    git config --local core.filemode false
    if ($LASTEXITCODE -ne 0) { throw "Could not configure Windows Git file-mode handling." }

    $Status = @(& git status --porcelain=v1 --untracked-files=all)
    if ($LASTEXITCODE -ne 0) { throw "git status failed." }
    if ($Status.Count -ne 0) {
        $Status | ForEach-Object { Write-Host $_ }
        throw "The migration branch must be clean before starting."
    }

    $Migration = Join-Path $Root "tools\windows_only_migration.py"
    if (-not (Test-Path -LiteralPath $Migration -PathType Leaf)) {
        throw "Guarded migration source is missing."
    }

    $Utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    $Source = [IO.File]::ReadAllText($Migration, [Text.Encoding]::UTF8)
    $Old = @'
def replace_exact(path: str, old: str, new: str) -> None:
    text = read(path)
    count = text.count(old)
'@
    $New = @'
def replace_exact(path: str, old: str, new: str) -> None:
    text = read(path)
    old = old.replace(chr(92) + "n", chr(10))
    new = new.replace(chr(92) + "n", chr(10))
    count = text.count(old)
'@
    if (($Source.Split($Old).Count - 1) -ne 1) {
        throw "Migration bootstrap anchor mismatch."
    }
    $Source = $Source.Replace($Old, $New)
    [IO.File]::WriteAllText($Migration, $Source.Replace("`r`n", "`n"), $Utf8NoBom)

    $AuditPython = Join-Path $env:LOCALAPPDATA "IrisOnlineDatabase\BuildTools\python-audit\Scripts\python.exe"
    if (-not (Test-Path -LiteralPath $AuditPython -PathType Leaf)) {
        throw "Validated Python audit environment is missing. Run IrisTools.ps1 -Action Install first."
    }
    $AuditEnv = Split-Path (Split-Path $AuditPython -Parent) -Parent
    $Ruff = Join-Path $AuditEnv "Scripts\ruff.exe"
    if (-not (Test-Path -LiteralPath $Ruff -PathType Leaf)) {
        throw "Pinned Ruff is missing from the audit environment."
    }

    Write-Host "`n=== APPLY WINDOWS-ONLY SOURCE MIGRATION ==="
    & $AuditPython -B $Migration
    if ($LASTEXITCODE -ne 0) { throw "Windows-only migration failed." }

    $SavedPythonPath = [Environment]::GetEnvironmentVariable("PYTHONPATH", "Process")
    $SavedPyCachePrefix = [Environment]::GetEnvironmentVariable("PYTHONPYCACHEPREFIX", "Process")
    $ExternalPyCache = Join-Path $env:LOCALAPPDATA "IrisOnlineDatabase\BuildTools\migration-pycache"
    Remove-Item -LiteralPath $ExternalPyCache -Recurse -Force -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force -Path $ExternalPyCache | Out-Null
    $env:PYTHONPATH = Join-Path $Root "tools"
    if ($SavedPythonPath) {
        $env:PYTHONPATH += [IO.Path]::PathSeparator + $SavedPythonPath
    }
    $env:PYTHONPYCACHEPREFIX = $ExternalPyCache

    try {
        Write-Host "`n=== RUFF FORMAT ==="
        & $Ruff format --no-cache .
        if ($LASTEXITCODE -ne 0) { throw "Ruff format failed." }

        Write-Host "`n=== RUFF CHECK ==="
        & $Ruff check --no-cache .
        if ($LASTEXITCODE -ne 0) { throw "Ruff check failed." }

        Write-Host "`n=== WORKFLOW VALIDATION ==="
        & $AuditPython -B tools\validate_workflows.py
        if ($LASTEXITCODE -ne 0) { throw "Workflow validation failed." }

        Write-Host "`n=== PYTHON TESTS ==="
        & $AuditPython -B -m unittest discover -s tests -p test_*.py
        if ($LASTEXITCODE -ne 0) { throw "Python tests failed." }

        Write-Host "`n=== NODE SYNTAX ==="
        node --check web\app.js
        if ($LASTEXITCODE -ne 0) { throw "Node syntax check failed." }

        Write-Host "`n=== GOFMT ==="
        $GoFiles = Get-ChildItem -LiteralPath $Root -Filter '*.go' -File | Select-Object -ExpandProperty FullName
        $Unformatted = @(& gofmt -l -- $GoFiles)
        if ($LASTEXITCODE -ne 0) { throw "gofmt failed." }
        if ($Unformatted.Count -ne 0) {
            $Unformatted | ForEach-Object { Write-Host $_ }
            throw "Go source is not formatted."
        }

        Write-Host "`n=== GO TEST ==="
        go test -count=1 ./...
        if ($LASTEXITCODE -ne 0) { throw "Go tests failed." }

        Write-Host "`n=== GO VET ==="
        go vet ./...
        if ($LASTEXITCODE -ne 0) { throw "go vet failed." }

        Write-Host "`n=== POWERSHELL PARSER ==="
        foreach ($File in @("IrisTools.ps1", "scripts\windows\IrisTools.ps1")) {
            $Tokens = $null
            $Errors = $null
            [System.Management.Automation.Language.Parser]::ParseFile(
                (Resolve-Path -LiteralPath $File).Path,
                [ref]$Tokens,
                [ref]$Errors
            ) | Out-Null
            if ($Errors.Count -ne 0) {
                $Errors | ForEach-Object { Write-Host $_ }
                throw "PowerShell syntax validation failed: $File"
            }
        }
    } finally {
        [Environment]::SetEnvironmentVariable("PYTHONPATH", $SavedPythonPath, "Process")
        [Environment]::SetEnvironmentVariable("PYTHONPYCACHEPREFIX", $SavedPyCachePrefix, "Process")
        Remove-Item -LiteralPath $ExternalPyCache -Recurse -Force -ErrorAction SilentlyContinue
    }

    Write-Host "`n=== REMOVE ONE-SHOT MIGRATION FILES ==="
    foreach ($Relative in @(
        ".github\workflows\windows-only-migration.yml",
        ".github\workflows\windows-only-migration-v2.yml",
        ".github\workflows\windows-only-migration-v3.yml",
        "tools\windows_only_migration.py",
        "scripts\windows\APPLY_WINDOWS_ONLY_MIGRATION.ps1"
    )) {
        $Target = Join-Path $Root $Relative
        if (Test-Path -LiteralPath $Target -PathType Leaf) {
            Remove-Item -LiteralPath $Target -Force
        }
    }

    foreach ($Legacy in @(
        "scripts\release-gate.sh",
        "scripts\build-release.sh",
        "scripts\release-tools.sh"
    )) {
        if (Test-Path -LiteralPath (Join-Path $Root $Legacy) -PathType Leaf) {
            throw "Legacy POSIX release file remains: $Legacy"
        }
    }

    Write-Host "`n=== FINAL REPOSITORY AUDIT ==="
    $FinalPyCache = Join-Path $env:LOCALAPPDATA "IrisOnlineDatabase\BuildTools\migration-final-pycache"
    Remove-Item -LiteralPath $FinalPyCache -Recurse -Force -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force -Path $FinalPyCache | Out-Null
    $OldFinalPrefix = [Environment]::GetEnvironmentVariable("PYTHONPYCACHEPREFIX", "Process")
    try {
        $env:PYTHONPYCACHEPREFIX = $FinalPyCache
        & $AuditPython -B tools\repository_audit.py
        if ($LASTEXITCODE -ne 0) { throw "Final repository audit failed." }
    } finally {
        [Environment]::SetEnvironmentVariable("PYTHONPYCACHEPREFIX", $OldFinalPrefix, "Process")
        Remove-Item -LiteralPath $FinalPyCache -Recurse -Force -ErrorAction SilentlyContinue
    }

    git diff --check
    if ($LASTEXITCODE -ne 0) { throw "git diff --check failed." }

    Write-Host "`n=== STAGE AND VERIFY ==="
    git add --renormalize .
    if ($LASTEXITCODE -ne 0) { throw "git add --renormalize failed." }
    git add -A
    if ($LASTEXITCODE -ne 0) { throw "git add failed." }
    git diff --cached --check
    if ($LASTEXITCODE -ne 0) { throw "Staged diff check failed." }
    git status --short

    Write-Host "`n=== COMMIT ==="
    git commit -m "refactor: make release pipeline Windows-only"
    if ($LASTEXITCODE -ne 0) { throw "Migration commit failed." }

    Write-Host "`n=== PUSH ==="
    git push -u origin $ExpectedBranch
    if ($LASTEXITCODE -ne 0) { throw "Migration branch push failed." }

    Write-Host "`nWINDOWS-ONLY MIGRATION: PASS" -ForegroundColor Green
} finally {
    Pop-Location
}
