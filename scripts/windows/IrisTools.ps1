param(
    [ValidateSet("Menu", "Check", "Install", "Prepare", "Test", "Build", "Publish", "Release", "Open", "SelfTest")]
    [string]$Action = "Menu",
    [string]$OutputDirectory = ""
)

$ErrorActionPreference = "Stop"
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$Version = (Get-Content -LiteralPath (Join-Path $Root "VERSION") -Raw).Trim()
$GoPin = (Get-Content -LiteralPath (Join-Path $Root ".go-version") -Raw).Trim()
$env:GOTOOLCHAIN = "local"
$WailsPin = "v2.14.0"
$StaticcheckPin = "2026.1"
$GovulncheckPin = "v1.6.0"
$GitleaksPin = "8.30.1"
$GitleaksWindowsX64Sha256 = "d29144deff3a68aa93ced33dddf84b7fdc26070add4aa0f4513094c8332afc4e"
$ToolRoot = Join-Path $env:LOCALAPPDATA "IrisOnlineDatabase\BuildTools"
$PinnedGoDirectory = Join-Path $ToolRoot ("go-" + $GoPin + "-windows-amd64")
$PinnedGoBinDirectory = Join-Path $PinnedGoDirectory "go\bin"
$PinnedGoExecutable = Join-Path $PinnedGoBinDirectory "go.exe"
$AuditEnvPointer = Join-Path $ToolRoot "python-audit-active.txt"
$AuditEnv = Join-Path $ToolRoot "python-audit"
$AuditPython = Join-Path $AuditEnv "Scripts\python.exe"
if (Test-Path -LiteralPath $AuditEnvPointer -PathType Leaf) {
    try {
        $CachedAuditEnv = (Get-Content -LiteralPath $AuditEnvPointer -Raw).Trim()
        $CachedAuditEnvFull = [IO.Path]::GetFullPath($CachedAuditEnv)
        $ToolRootFull = [IO.Path]::GetFullPath($ToolRoot)
        if ($CachedAuditEnvFull.StartsWith($ToolRootFull + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase) -and
            (Test-Path -LiteralPath (Join-Path $CachedAuditEnvFull "Scripts\python.exe") -PathType Leaf)) {
            $AuditEnv = $CachedAuditEnvFull
            $AuditPython = Join-Path $AuditEnv "Scripts\python.exe"
        }
    } catch {
        # Ignore a stale/corrupt local cache pointer. Ensure-AuditEnvironment validates it later.
    }
}
$env:PLAYWRIGHT_BROWSERS_PATH = Join-Path $ToolRoot "playwright"
$PinnedGitleaksDirectory = Join-Path $ToolRoot "gitleaks-$GitleaksPin"
$RepositorySlug = "fsibatov/iris-online-database"
$ReleaseGitName = "fsibatov"
$ReleaseGitEmail = "farushik01@gmail.com"
$ReleaseGpgExecutable = "C:\Program Files\Git\usr\bin\gpg.exe"
$ReleaseGpgFingerprint = "B0A5D341B2EE901172F485DE9BC0EBCFE2795291"

function Test-IsAdministrator {
    $Identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $Principal = New-Object Security.Principal.WindowsPrincipal($Identity)
    return $Principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Update-ProcessPath {
    $Values = @(
        [Environment]::GetEnvironmentVariable("Path", "Process"),
        [Environment]::GetEnvironmentVariable("Path", "Machine"),
        [Environment]::GetEnvironmentVariable("Path", "User")
    )
    $KnownDirectories = @()
    if ($env:ProgramFiles) {
        $KnownDirectories += Join-Path $env:ProgramFiles "Git\cmd"
        $KnownDirectories += Join-Path $env:ProgramFiles "nodejs"
    }
    if (${env:ProgramFiles(x86)}) {
        $KnownDirectories += Join-Path ${env:ProgramFiles(x86)} "Git\cmd"
    }
    $KnownDirectories += Join-Path $env:LOCALAPPDATA "Programs\Git\cmd"
    $KnownDirectories += Join-Path $env:LOCALAPPDATA "Programs\nodejs"
    $KnownDirectories += $PinnedGoBinDirectory
    $KnownDirectories += Join-Path $env:LOCALAPPDATA "Microsoft\WinGet\Links"

    $Seen = @{}
    $Directories = New-Object System.Collections.Generic.List[string]
    foreach ($Directory in $KnownDirectories) {
        if (-not (Test-Path -LiteralPath $Directory -PathType Container)) { continue }
        $Key = $Directory.ToLowerInvariant()
        if (-not $Seen.ContainsKey($Key)) {
            $Seen[$Key] = $true
            $Directories.Add($Directory)
        }
    }
    foreach ($Value in $Values) {
        if (-not $Value) { continue }
        foreach ($Directory in ($Value -split ";")) {
            $Directory = $Directory.Trim()
            if (-not $Directory) { continue }
            $Key = $Directory.ToLowerInvariant()
            if (-not $Seen.ContainsKey($Key)) {
                $Seen[$Key] = $true
                $Directories.Add($Directory)
            }
        }
    }
    $env:PATH = $Directories -join ";"
}

Update-ProcessPath
if ((Test-Path -LiteralPath (Join-Path $PinnedGitleaksDirectory "gitleaks.exe")) -and
    (($env:PATH -split ";") -notcontains $PinnedGitleaksDirectory)) {
    $env:PATH = $PinnedGitleaksDirectory + ";" + $env:PATH
}

function Test-GovulncheckNetworkFailure {
    param([string]$Text)
    $Patterns = @(
        "fetching vulnerabilities",
        "\b(?:dial|read|write) tcp\b",
        "wsarecv",
        "no such host",
        "temporary failure in name resolution",
        "i/o timeout",
        "timed out",
        "tls handshake timeout",
        "context deadline exceeded",
        "connection (?:refused|reset|timed out)",
        "proxyconnect tcp",
        "unexpected eof",
        "http2: client connection lost",
        "x509:",
        "status(?: code)? (?:403|408|429|5\d\d)"
    )
    foreach ($Pattern in $Patterns) {
        if ($Text -match ("(?i)" + $Pattern)) { return $true }
    }
    return $false
}

function ConvertTo-SafeToolOutput {
    param([string]$Text)
    if (-not $Text) { return "" }
    $SafeText = $Text
    foreach ($SensitiveRoot in @($Root, $env:USERPROFILE, $env:LOCALAPPDATA, $env:TEMP)) {
        if (-not $SensitiveRoot) { continue }
        $SafeText = [Regex]::Replace(
            $SafeText,
            [Regex]::Escape($SensitiveRoot),
            "[redacted-path]",
            [Text.RegularExpressions.RegexOptions]::IgnoreCase
        )
    }
    $SafeText = [Regex]::Replace(
        $SafeText,
        "(?i)\b(?:gh[pousr]_[A-Za-z0-9_]{20,}|github_pat_[A-Za-z0-9_]{20,})\b",
        "[redacted-token]"
    )
    return $SafeText
}

function ConvertTo-NativeArgument {
    param([AllowEmptyString()][string]$Value)
    if ($null -eq $Value -or $Value.Length -eq 0) { return '""' }
    if ($Value -notmatch '[\s"]') { return $Value }

    $Builder = New-Object System.Text.StringBuilder
    $Slash = [string][char]0x5C
    [void]$Builder.Append([char]0x22)
    $Backslashes = 0
    foreach ($Character in $Value.ToCharArray()) {
        if ($Character -eq [char]0x5C) {
            $Backslashes++
            continue
        }
        if ($Character -eq [char]0x22) {
            if ($Backslashes -gt 0) {
                [void]$Builder.Append(($Slash * ($Backslashes * 2)))
            }
            [void]$Builder.Append($Slash)
            [void]$Builder.Append([char]0x22)
            $Backslashes = 0
            continue
        }
        if ($Backslashes -gt 0) {
            [void]$Builder.Append(($Slash * $Backslashes))
            $Backslashes = 0
        }
        [void]$Builder.Append($Character)
    }
    if ($Backslashes -gt 0) {
        [void]$Builder.Append(($Slash * ($Backslashes * 2)))
    }
    [void]$Builder.Append([char]0x22)
    return $Builder.ToString()
}

function ConvertTo-NativeArgumentString {
    param([string[]]$Arguments)
    $EncodedArguments = @($Arguments | ForEach-Object { ConvertTo-NativeArgument -Value $_ })
    return $EncodedArguments -join " "
}

function Invoke-CapturedNativeProcess {
    param(
        [string]$File,
        [string[]]$Arguments,
        [string]$WorkingDirectory,
        [int]$TimeoutSeconds
    )
    $StartInfo = New-Object System.Diagnostics.ProcessStartInfo
    $StartInfo.FileName = $File
    $StartInfo.Arguments = ConvertTo-NativeArgumentString $Arguments
    $StartInfo.WorkingDirectory = $WorkingDirectory
    $StartInfo.UseShellExecute = $false
    $StartInfo.CreateNoWindow = $true
    $StartInfo.RedirectStandardOutput = $true
    $StartInfo.RedirectStandardError = $true
    $Utf8 = New-Object System.Text.UTF8Encoding($false)
    $StartInfo.StandardOutputEncoding = $Utf8
    $StartInfo.StandardErrorEncoding = $Utf8
    $StartInfo.EnvironmentVariables["PYTHONIOENCODING"] = "utf-8"
    $StartInfo.EnvironmentVariables["PYTHONUTF8"] = "1"

    $Process = New-Object System.Diagnostics.Process
    $Process.StartInfo = $StartInfo
    try {
        if (-not $Process.Start()) { throw "Native process did not start." }
        $StdoutTask = $Process.StandardOutput.ReadToEndAsync()
        $StderrTask = $Process.StandardError.ReadToEndAsync()
        $TimedOut = -not $Process.WaitForExit($TimeoutSeconds * 1000)
        if ($TimedOut) {
            $Taskkill = Join-Path $env:SystemRoot "System32\taskkill.exe"
            if (Test-Path -LiteralPath $Taskkill -PathType Leaf) {
                & $Taskkill /PID $Process.Id /T /F *> $null
            }
            if (-not $Process.HasExited) { $Process.Kill() }
        }
        [void]$Process.WaitForExit()
        $StdoutText = $StdoutTask.GetAwaiter().GetResult()
        $StderrText = $StderrTask.GetAwaiter().GetResult()
        $ExitCode = if ($TimedOut) { -1 } else { $Process.ExitCode }
        return [pscustomobject]@{
            ExitCode = $ExitCode
            TimedOut = $TimedOut
            Stdout = [string]$StdoutText
            Stderr = [string]$StderrText
        }
    } catch {
        throw "Native process could not be started or monitored."
    } finally {
        $Process.Dispose()
    }
}

function Resolve-NativeExecutablePath {
    param([string]$File)
    if ([string]::IsNullOrWhiteSpace($File)) {
        throw "Required tool executable name is empty."
    }
    if ([IO.Path]::IsPathRooted($File) -or $File.Contains("\") -or $File.Contains("/")) {
        if (-not (Test-Path -LiteralPath $File -PathType Leaf)) {
            throw "Required tool executable is missing."
        }
        return [IO.Path]::GetFullPath($File)
    }
    $Executable = Get-Command $File -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $Executable) {
        throw "Required tool executable is missing: $File"
    }
    return [string]$Executable.Source
}

function Invoke-Checked {
    param([string]$File, [string[]]$Arguments, [int]$TimeoutSeconds = 600)
    Write-Host ("+ " + $File + " " + ($Arguments -join " "))
    try {
        $ExecutablePath = Resolve-NativeExecutablePath -File $File
        $Result = Invoke-CapturedNativeProcess `
            -File $ExecutablePath `
            -Arguments $Arguments `
            -WorkingDirectory (Get-Location).Path `
            -TimeoutSeconds $TimeoutSeconds
    } catch {
        throw "Required tool process could not be completed."
    }
    foreach ($Text in @([string]$Result.Stdout, [string]$Result.Stderr)) {
        if (-not $Text) { continue }
        $SafeText = (ConvertTo-SafeToolOutput $Text).TrimEnd()
        if ($SafeText) { Write-Host $SafeText }
    }
    if ($Result.TimedOut) {
        throw "Tool watchdog expired after $TimeoutSeconds seconds."
    }
    $ExitCode = [int]$Result.ExitCode
    if ($ExitCode -ne 0) { throw "Required tool failed with exit code $ExitCode." }
}

function Invoke-Govulncheck {
    param([int]$TimeoutSeconds = 300)
    try {
        $GovulncheckExecutable = Get-PinnedGovulncheckExecutable
    } catch {
        throw "[SECURITY FAIL] pinned govulncheck executable is missing or incompatible."
    }

    $DatabaseAttempts = @(
        [pscustomobject]@{ Name = "Google storage"; URL = "https://storage.googleapis.com/go-vulndb" },
        [pscustomobject]@{ Name = "Google storage retry"; URL = "https://storage.googleapis.com/go-vulndb" },
        [pscustomobject]@{ Name = "canonical fallback"; URL = "https://vuln.go.dev" }
    )
    for ($AttemptIndex = 0; $AttemptIndex -lt $DatabaseAttempts.Count; $AttemptIndex++) {
        $Attempt = $AttemptIndex + 1
        $Database = $DatabaseAttempts[$AttemptIndex]
        $Attempts = $DatabaseAttempts.Count
        try {
            Write-Host "+ govulncheck -db $($Database.URL) ./... (attempt $Attempt/$Attempts)"
            $Result = Invoke-CapturedNativeProcess `
                -File $GovulncheckExecutable `
                -Arguments @("-db", $Database.URL, "./...") `
                -WorkingDirectory $Root `
                -TimeoutSeconds $TimeoutSeconds
        } catch {
            throw "[SECURITY FAIL] govulncheck process could not be started or monitored."
        }
        $TimedOut = [bool]$Result.TimedOut
        $ExitCode = [int]$Result.ExitCode
        $StdoutText = [string]$Result.Stdout
        $StderrText = [string]$Result.Stderr

        if (-not $TimedOut -and $ExitCode -eq 0) {
            if ($StdoutText) { Write-Host (ConvertTo-SafeToolOutput $StdoutText).TrimEnd() }
            if ($StderrText) { Write-Host (ConvertTo-SafeToolOutput $StderrText).TrimEnd() }
            if ($Database.Name -eq "canonical fallback") {
                Write-Host "[NETWORK/INFRASTRUCTURE FALLBACK] Google-hosted Go vulnerability database storage was unavailable; the scan used the canonical vuln.go.dev endpoint." -ForegroundColor Yellow
            }
            Write-Host "govulncheck: PASS" -ForegroundColor Green
            return
        }

        $FailureText = $StdoutText + "`n" + $StderrText
        $InfrastructureFailure = $TimedOut -or (Test-GovulncheckNetworkFailure $FailureText)
        if (-not $InfrastructureFailure) {
            if ($StdoutText) { Write-Host (ConvertTo-SafeToolOutput $StdoutText).TrimEnd() }
            if ($StderrText) { Write-Host (ConvertTo-SafeToolOutput $StderrText).TrimEnd() }
            throw "[SECURITY FAIL] govulncheck completed without a successful result (exit code $ExitCode)."
        }

        if ($Attempt -lt $Attempts) {
            $DelaySeconds = 2
            $NextDatabase = $DatabaseAttempts[$AttemptIndex + 1]
            Write-Host "[NETWORK/INFRASTRUCTURE RETRY] Go vulnerability database is unavailable through $($Database.Name) (attempt $Attempt/$Attempts); retrying through $($NextDatabase.Name) in $DelaySeconds seconds." -ForegroundColor Yellow
            Start-Sleep -Seconds $DelaySeconds
            continue
        }

        throw "[NETWORK/INFRASTRUCTURE SKIP] govulncheck could not reach the Go vulnerability database through its Google storage and canonical endpoints after $Attempts attempts. Vulnerability status is UNKNOWN; RELEASE gate remains FAILED. Allow HTTPS/443 access to storage.googleapis.com and vuln.go.dev and retry."
    }
}

function Get-VersionLine {
    param([string]$Command, [string[]]$Arguments)
    $Executable = Get-Command $Command -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $Executable) { return "MISSING" }
    $Output = & $Executable.Source @Arguments 2>&1
    $ExitCode = $LASTEXITCODE
    if ($ExitCode -ne 0) { return "FAIL" }
    $Value = $Output | ForEach-Object { ([string]$_).Trim() } | Where-Object { $_ } | Select-Object -First 1
    if (-not $Value) { return "FAIL" }
    return [string]$Value
}

function ConvertFrom-WailsModuleMetadata {
    param([string[]]$Lines)
    $ExpectedMainPath = "github.com/wailsapp/wails/v2/cmd/wails"
    $ExpectedModulePath = "github.com/wailsapp/wails/v2"
    $MainPathFound = $false
    $ModuleVersion = ""
    foreach ($Line in $Lines) {
        $Text = ([string]$Line).Trim()
        if ($Text -match ("^path\s+" + [Regex]::Escape($ExpectedMainPath) + "$")) {
            $MainPathFound = $true
            continue
        }
        if ($Text -match ("^mod\s+" + [Regex]::Escape($ExpectedModulePath) + "\s+([^\s]+)(?:\s+.*)?$")) {
            $ModuleVersion = $Matches[1]
        }
    }
    if (-not $MainPathFound -or -not $ModuleVersion) { return "FAIL" }
    return $ModuleVersion
}

function Get-GoBinDirectory {
    $GoExecutable = Get-Command "go.exe" -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $GoExecutable) {
        $GoExecutable = Get-Command "go" -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    }
    if (-not $GoExecutable) { return "" }
    try {
        $GoBin = (& $GoExecutable.Source env GOBIN 2>$null | Out-String).Trim()
        if ($LASTEXITCODE -ne 0) { return "" }
        if (-not $GoBin) {
            $GoPath = (& $GoExecutable.Source env GOPATH 2>$null | Out-String).Trim()
            if ($LASTEXITCODE -ne 0 -or -not $GoPath) { return "" }
            $GoPathEntries = $GoPath -split [Regex]::Escape([string][IO.Path]::PathSeparator)
            $GoPath = $GoPathEntries | Where-Object { $_ } | Select-Object -First 1
            if (-not $GoPath) { return "" }
            $GoBin = Join-Path $GoPath "bin"
        }
        return [IO.Path]::GetFullPath($GoBin)
    } catch {
        return ""
    }
}

function Get-WailsModuleVersion {
    param([string]$Executable)
    if (-not $Executable -or -not (Test-Path -LiteralPath $Executable -PathType Leaf)) { return "MISSING" }
    $GoExecutable = Get-Command "go.exe" -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $GoExecutable) {
        $GoExecutable = Get-Command "go" -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    }
    if (-not $GoExecutable) { return "FAIL" }
    try {
        $Output = & $GoExecutable.Source version -m $Executable 2>&1
        $ExitCode = $LASTEXITCODE
    } catch {
        return "FAIL"
    }
    if ($ExitCode -ne 0) { return "FAIL" }
    return ConvertFrom-WailsModuleMetadata -Lines @($Output)
}

function Get-WailsInfo {
    $Candidates = New-Object System.Collections.Generic.List[string]
    $Seen = @{}
    $GoBin = Get-GoBinDirectory
    if ($GoBin) {
        $Candidate = Join-Path $GoBin "wails.exe"
        $Key = $Candidate.ToLowerInvariant()
        if (-not $Seen.ContainsKey($Key)) { $Seen[$Key] = $true; $Candidates.Add($Candidate) }
    }
    if ($env:USERPROFILE) {
        $Candidate = Join-Path $env:USERPROFILE "go\bin\wails.exe"
        $Key = $Candidate.ToLowerInvariant()
        if (-not $Seen.ContainsKey($Key)) { $Seen[$Key] = $true; $Candidates.Add($Candidate) }
    }
    $PathWails = Get-Command "wails.exe" -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $PathWails) {
        $PathWails = Get-Command "wails" -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    }
    if ($PathWails) {
        $Candidate = $PathWails.Source
        $Key = $Candidate.ToLowerInvariant()
        if (-not $Seen.ContainsKey($Key)) { $Seen[$Key] = $true; $Candidates.Add($Candidate) }
    }

    $FirstExisting = $null
    foreach ($Candidate in $Candidates) {
        if (-not (Test-Path -LiteralPath $Candidate -PathType Leaf)) { continue }
        $Version = Get-WailsModuleVersion -Executable $Candidate
        $Info = [pscustomobject]@{ Path = [IO.Path]::GetFullPath($Candidate); Version = $Version }
        if (-not $FirstExisting) { $FirstExisting = $Info }
        if ($Version -eq $WailsPin) { return $Info }
    }
    if ($FirstExisting) { return $FirstExisting }
    return [pscustomobject]@{ Path = ""; Version = "MISSING" }
}

function Get-PinnedWailsExecutable {
    $Info = Get-WailsInfo
    if ($Info.Version -eq $WailsPin -and $Info.Path) { return $Info.Path }
    if ($Info.Path) {
        throw "Wails version mismatch. Required $WailsPin; found $($Info.Version) at $($Info.Path). Run menu option 5 - INSTALL/UPDATE TOOLS."
    }
    throw "Wails $WailsPin executable is missing. Run menu option 5 - INSTALL/UPDATE TOOLS."
}

function Get-AuditPackageVersion {
    param([string]$Package)
    if (-not (Test-Path -LiteralPath $AuditPython)) { return "MISSING" }
    $Output = & $AuditPython -c "import importlib.metadata,sys; print(importlib.metadata.version(sys.argv[1]))" $Package 2>$null
    $ExitCode = $LASTEXITCODE
    $Value = $Output | ForEach-Object { ([string]$_).Trim() } | Where-Object { $_ } | Select-Object -First 1
    if ($ExitCode -ne 0 -or -not $Value) { return "MISSING" }
    return ([string]$Value).Trim()
}

function Get-RequirementPin {
    param([string]$Package)
    $Pattern = "^" + [Regex]::Escape($Package) + "==(.+)$"
    $Line = Get-Content -LiteralPath (Join-Path $Root "tools\requirements-audit.txt") | Where-Object { $_ -match $Pattern } | Select-Object -First 1
    if (-not $Line) { return "MISSING_PIN" }
    [void]($Line -match $Pattern)
    return $Matches[1]
}

function Get-GoToolCandidates {
    param([string]$Name)
    $Candidates = New-Object System.Collections.Generic.List[string]
    $Seen = @{}
    $GoBin = Get-GoBinDirectory
    if ($GoBin) {
        $Candidate = Join-Path $GoBin ($Name + ".exe")
        $Key = $Candidate.ToLowerInvariant()
        if (-not $Seen.ContainsKey($Key)) { $Seen[$Key] = $true; $Candidates.Add($Candidate) }
    }
    if ($env:USERPROFILE) {
        $Candidate = Join-Path $env:USERPROFILE ("go\bin\" + $Name + ".exe")
        $Key = $Candidate.ToLowerInvariant()
        if (-not $Seen.ContainsKey($Key)) { $Seen[$Key] = $true; $Candidates.Add($Candidate) }
    }
    $PathTool = Get-Command ($Name + ".exe") -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $PathTool) {
        $PathTool = Get-Command $Name -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    }
    if ($PathTool) {
        $Candidate = [string]$PathTool.Source
        $Key = $Candidate.ToLowerInvariant()
        if (-not $Seen.ContainsKey($Key)) { $Seen[$Key] = $true; $Candidates.Add($Candidate) }
    }
    return $Candidates.ToArray()
}

function ConvertFrom-StaticcheckVersionLine {
    param([string]$Line)
    if ($Line -match "^staticcheck(?:\.exe)?\s+([^\s]+)") { return $Matches[1] }
    return $Line
}

function Get-StaticcheckInfo {
    $FirstExisting = $null
    foreach ($Candidate in @(Get-GoToolCandidates -Name "staticcheck")) {
        if (-not (Test-Path -LiteralPath $Candidate -PathType Leaf)) { continue }
        try {
            $Output = & $Candidate -version 2>&1
            $ExitCode = $LASTEXITCODE
            $Line = $Output | ForEach-Object { ([string]$_).Trim() } | Where-Object { $_ } | Select-Object -First 1
            $Version = if ($ExitCode -eq 0 -and $Line) { ConvertFrom-StaticcheckVersionLine ([string]$Line) } else { "FAIL" }
        } catch {
            $Version = "FAIL"
        }
        $Info = [pscustomobject]@{ Path = [IO.Path]::GetFullPath($Candidate); Version = $Version }
        if (-not $FirstExisting) { $FirstExisting = $Info }
        if ($Version -eq $StaticcheckPin) { return $Info }
    }
    if ($FirstExisting) { return $FirstExisting }
    return [pscustomobject]@{ Path = ""; Version = "MISSING" }
}

function Get-PinnedStaticcheckExecutable {
    $Info = Get-StaticcheckInfo
    if ($Info.Path -and $Info.Version -eq $StaticcheckPin) { return $Info.Path }
    if ($Info.Path) { throw "Staticcheck version mismatch. Required $StaticcheckPin." }
    throw "Staticcheck $StaticcheckPin executable is missing. Run menu option 5 - INSTALL/UPDATE TOOLS."
}

function Get-StaticcheckVersion {
    return (Get-StaticcheckInfo).Version
}

function Get-GovulncheckInfo {
    $FirstExisting = $null
    foreach ($Candidate in @(Get-GoToolCandidates -Name "govulncheck")) {
        if (-not (Test-Path -LiteralPath $Candidate -PathType Leaf)) { continue }
        try {
            $Output = (& $Candidate -version 2>&1 | Out-String)
            $ExitCode = $LASTEXITCODE
            $Version = if ($ExitCode -eq 0 -and $Output -match "Scanner:\s+govulncheck@(v[^\s]+)") { $Matches[1] } else { "FAIL" }
        } catch {
            $Version = "FAIL"
        }
        $Info = [pscustomobject]@{ Path = [IO.Path]::GetFullPath($Candidate); Version = $Version }
        if (-not $FirstExisting) { $FirstExisting = $Info }
        if ($Version -eq $GovulncheckPin) { return $Info }
    }
    if ($FirstExisting) { return $FirstExisting }
    return [pscustomobject]@{ Path = ""; Version = "MISSING" }
}

function Get-PinnedGovulncheckExecutable {
    $Info = Get-GovulncheckInfo
    if ($Info.Path -and $Info.Version -eq $GovulncheckPin) { return $Info.Path }
    if ($Info.Path) { throw "govulncheck version mismatch. Required $GovulncheckPin." }
    throw "govulncheck $GovulncheckPin executable is missing. Run menu option 5 - INSTALL/UPDATE TOOLS."
}

function Get-GovulncheckVersion {
    return (Get-GovulncheckInfo).Version
}

function Invoke-Staticcheck {
    param([string[]]$Arguments = @("./..."))
    $StaticcheckExecutable = Get-PinnedStaticcheckExecutable
    Write-Host ("+ " + $StaticcheckExecutable + " " + ($Arguments -join " "))
    $Output = & $StaticcheckExecutable @Arguments 2>&1
    $ExitCode = $LASTEXITCODE
    if ($Output) {
        $SafeOutput = ConvertTo-SafeToolOutput (($Output | Out-String).TrimEnd())
        if ($SafeOutput) { Write-Host $SafeOutput }
    }
    if ($ExitCode -ne 0) {
        throw "staticcheck failed with exit code $ExitCode."
    }
}

function Test-WindowsTooling {
    $FailedProbes = New-Object System.Collections.Generic.List[string]
    if ((Get-VersionLine "cmd.exe" @("/d", "/c", "echo", "iris-native-probe")) -ne "iris-native-probe") {
        $FailedProbes.Add("CMD_VERSION")
    }
    if ((ConvertFrom-StaticcheckVersionLine "staticcheck.exe 2026.1 (v0.7.0)") -ne "2026.1") {
        $FailedProbes.Add("STATICCHECK_VERSION_PARSE")
    }
    try {
        # Exercise the exact production path, including the pinned resolver and
        # direct PowerShell invocation. Do not route this probe through the
        # generic ProcessStartInfo capture wrapper.
        Invoke-Staticcheck @("-version")
    } catch {
        $FailedProbes.Add("STATICCHECK_DIRECT")
    }
    $WailsMetadataProbe = @(
        ("C:\probe\wails.exe: go" + $GoPin),
        "path`tgithub.com/wailsapp/wails/v2/cmd/wails",
        "mod`tgithub.com/wailsapp/wails/v2`tv2.14.0`th1:fixture"
    )
    if ((ConvertFrom-WailsModuleMetadata -Lines $WailsMetadataProbe) -ne $WailsPin) {
        $FailedProbes.Add("WAILS_METADATA_PARSE")
    }
    try {
        $PinnedWailsExecutable = Get-PinnedWailsExecutable
        Write-Host "Wails: $WailsPin @ $PinnedWailsExecutable"
    } catch {
        $FailedProbes.Add("WAILS_RESOLVER")
    }
    try {
        [void](Get-PinnedGovulncheckExecutable)
    } catch {
        $FailedProbes.Add("GOVULNCHECK_RESOLVER")
    }
    if (-not (Test-GovulncheckNetworkFailure "fetching vulnerabilities: read tcp: wsarecv")) {
        $FailedProbes.Add("NETWORK_CLASSIFICATION")
    }
    if (Test-GovulncheckNetworkFailure "Vulnerability #1: GO-TEST-0001; see https://vuln.go.dev/ID/GO-TEST-0001.json") {
        $FailedProbes.Add("VULNERABILITY_CLASSIFICATION")
    }
    $AnsiEscape = [string][char]27
    $GitleaksAnsiProbe = "9:48AM INF 48" + $AnsiEscape + "[0m commits scanned."
    if (-not (Test-GitleaksHistoryProof $GitleaksAnsiProbe)) {
        $FailedProbes.Add("GITLEAKS_HISTORY_ANSI_PROOF")
    }
    if (Test-GitleaksHistoryProof "9:48AM INF 0 commits scanned.") {
        $FailedProbes.Add("GITLEAKS_HISTORY_ZERO_PROOF")
    }
    $SensitiveProbe = Join-Path $Root "private-project"
    $UnsafeProbe = $SensitiveProbe + " ghp_" + ("A" * 36)
    $SafeProbe = ConvertTo-SafeToolOutput $UnsafeProbe
    if ($SafeProbe -match [Regex]::Escape($SensitiveProbe) -or $SafeProbe -match "ghp_A") {
        $FailedProbes.Add("OUTPUT_REDACTION")
    }
    try {
        $CmdExecutable = (Get-Command "cmd.exe" -CommandType Application -ErrorAction Stop).Source
        $SuccessProbe = Invoke-CapturedNativeProcess `
            -File $CmdExecutable `
            -Arguments @("/d", "/s", "/c", "echo No vulnerabilities found. & exit /b 0") `
            -WorkingDirectory $Root `
            -TimeoutSeconds 30
        if ($SuccessProbe.TimedOut -or $SuccessProbe.ExitCode -ne 0 -or $SuccessProbe.Stdout -notmatch "No vulnerabilities found\.") {
            $FailedProbes.Add("CAPTURE_SUCCESS")
        }
        $FailureProbe = Invoke-CapturedNativeProcess `
            -File $CmdExecutable `
            -Arguments @("/d", "/s", "/c", "exit /b 7") `
            -WorkingDirectory $Root `
            -TimeoutSeconds 30
        if ($FailureProbe.TimedOut -or $FailureProbe.ExitCode -ne 7) {
            $FailedProbes.Add("CAPTURE_EXIT_CODE")
        }
        try {
            Invoke-Checked $CmdExecutable @("/d", "/c", "exit", "/b", "0") 30
        } catch {
            $FailedProbes.Add("CHECKED_SUCCESS")
        }
        $CheckedFailureDetected = $false
        try {
            Invoke-Checked $CmdExecutable @("/d", "/c", "exit", "/b", "7") 30
        } catch {
            $CheckedFailureDetected = $_.Exception.Message -eq "Required tool failed with exit code 7."
        }
        if (-not $CheckedFailureDetected) {
            $FailedProbes.Add("CHECKED_FAILURE")
        }
        $WindowsPowerShell = Join-Path $env:SystemRoot "System32\WindowsPowerShell\v1.0\powershell.exe"
        if (-not (Test-Path -LiteralPath $WindowsPowerShell -PathType Leaf)) {
            $FailedProbes.Add("POWERSHELL_PATH")
        } else {
            $EncodingProbe = Invoke-CapturedNativeProcess `
                -File $WindowsPowerShell `
                -Arguments @(
                    "-NoLogo",
                    "-NoProfile",
                    "-Command",
                    "[Console]::OpenStandardOutput().Write([byte[]](0xD1,0x8F,0x0A),0,3)"
                ) `
                -WorkingDirectory $Root `
                -TimeoutSeconds 30
            if ($EncodingProbe.TimedOut -or $EncodingProbe.ExitCode -ne 0 -or
                $EncodingProbe.Stdout.Trim() -ne [string][char]0x044F) {
                $FailedProbes.Add("UTF8_CAPTURE")
            }
        }
    } catch {
        $FailedProbes.Add("NATIVE_RUNNER_EXCEPTION")
    }
    if ($FailedProbes.Count -ne 0) {
        throw "Windows tooling self-test: FAIL [TOOL_PROBE] count=$($FailedProbes.Count) categories=$($FailedProbes -join ',')"
    }
    Write-Host "Windows tooling self-test: PASS" -ForegroundColor Green
}

function Get-WebView2Version {
    $Client = "{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"
    $Keys = @(
        "HKLM:\SOFTWARE\Microsoft\EdgeUpdate\Clients\$Client",
        "HKLM:\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\$Client",
        "HKCU:\Software\Microsoft\EdgeUpdate\Clients\$Client"
    )
    foreach ($Key in $Keys) {
        $Value = (Get-ItemProperty -LiteralPath $Key -Name pv -ErrorAction SilentlyContinue).pv
        if ($Value) { return [string]$Value }
    }
    return "MISSING"
}

function Get-ToolStatus {
    param([bool]$Condition)
    if ($Condition) { return "OK" }
    return "FAIL"
}

function Get-Python313Version {
    param([string]$Python)
    if (-not $Python -or -not (Test-Path -LiteralPath $Python -PathType Leaf)) { return "" }
    try {
        $ActualPythonVersion = (& $Python --version 2>&1 | Out-String).Trim()
        if ($LASTEXITCODE -ne 0 -or $ActualPythonVersion -notmatch "^Python 3\.13(?:\.|$)") {
            return ""
        }
        return $ActualPythonVersion
    } catch {
        return ""
    }
}

function Get-AuditEnvironmentHash {
    param([string]$Requirements, [string]$PythonVersion)
    $HashInput = ((Get-FileHash -LiteralPath $Requirements -Algorithm SHA256).Hash + "|" + $PythonVersion)
    $Bytes = [Text.Encoding]::UTF8.GetBytes($HashInput)
    $Hasher = [Security.Cryptography.SHA256]::Create()
    try {
        return ([BitConverter]::ToString($Hasher.ComputeHash($Bytes))).Replace("-", "").ToLowerInvariant()
    } finally {
        $Hasher.Dispose()
    }
}

function Test-AuditEnvironmentContent {
    param([string]$EnvironmentPath, [string]$ExpectedPythonVersion = "")
    if (-not $EnvironmentPath) { return $false }
    $Python = Join-Path $EnvironmentPath "Scripts\python.exe"
    $ActualPythonVersion = Get-Python313Version -Python $Python
    if (-not $ActualPythonVersion) { return $false }
    if ($ExpectedPythonVersion -and $ActualPythonVersion -ne $ExpectedPythonVersion) { return $false }
    try {
        & $Python -B (Join-Path $Root "tools\verify_python_environment.py") *> $null
        if ($LASTEXITCODE -ne 0) { return $false }
        & $Python -m pip check *> $null
        return $LASTEXITCODE -eq 0
    } catch {
        return $false
    }
}

function Test-AuditEnvironment {
    param(
        [string]$EnvironmentPath,
        [string]$ExpectedHash,
        [string]$ExpectedPythonVersion
    )
    if (-not (Test-AuditEnvironmentContent -EnvironmentPath $EnvironmentPath -ExpectedPythonVersion $ExpectedPythonVersion)) {
        return $false
    }
    $Marker = Join-Path $EnvironmentPath ".iris-requirements-sha256"
    if (-not (Test-Path -LiteralPath $Marker -PathType Leaf)) { return $false }
    try {
        return (Get-Content -LiteralPath $Marker -Raw).Trim() -eq $ExpectedHash
    } catch {
        return $false
    }
}

function Invoke-PythonVenv {
    param([string]$Python, [string]$Destination)
    Write-Host ("+ " + $Python + " -m venv " + $Destination)
    & $Python -m venv $Destination
    $ExitCode = $LASTEXITCODE
    if ($ExitCode -ne 0) {
        throw "Python venv creation failed with exit code $ExitCode."
    }
    $CreatedPython = Join-Path $Destination "Scripts\python.exe"
    if (-not (Test-Path -LiteralPath $CreatedPython -PathType Leaf)) {
        throw "Python venv creation completed without Scripts\python.exe."
    }
}

function Use-AuditEnvironment {
    param([string]$EnvironmentPath)
    $script:AuditEnv = $EnvironmentPath
    $script:AuditPython = Join-Path $EnvironmentPath "Scripts\python.exe"
}

function Add-UniquePathCandidate {
    param(
        [System.Collections.Generic.List[string]]$List,
        [string]$Path
    )
    if (-not $Path) { return }
    try {
        $Full = [IO.Path]::GetFullPath($Path)
    } catch {
        return
    }
    if (-not $List.Contains($Full)) { $List.Add($Full) }
}

function Find-Python313Executable {
    param([string[]]$AuditEnvironmentCandidates)
    $Candidates = New-Object System.Collections.Generic.List[string]

    $PythonCommand = Get-Command "python.exe" -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($PythonCommand) { Add-UniquePathCandidate -List $Candidates -Path $PythonCommand.Source }

    if ($env:LOCALAPPDATA) {
        Add-UniquePathCandidate -List $Candidates -Path (Join-Path $env:LOCALAPPDATA "Programs\Python\Python313\python.exe")
        Add-UniquePathCandidate -List $Candidates -Path (Join-Path $env:LOCALAPPDATA "Programs\Python\Python313-32\python.exe")
    }
    if ($env:ProgramFiles) {
        Add-UniquePathCandidate -List $Candidates -Path (Join-Path $env:ProgramFiles "Python313\python.exe")
    }
    if (${env:ProgramFiles(x86)}) {
        Add-UniquePathCandidate -List $Candidates -Path (Join-Path ${env:ProgramFiles(x86)} "Python313-32\python.exe")
    }

    foreach ($EnvironmentPath in $AuditEnvironmentCandidates) {
        if ($EnvironmentPath) {
            Add-UniquePathCandidate -List $Candidates -Path (Join-Path $EnvironmentPath "Scripts\python.exe")
        }
    }

    $PyLauncher = Get-Command "py.exe" -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($PyLauncher) {
        try {
            $Resolved = (& $PyLauncher.Source -3.13 -c "import sys; print(sys.executable)" 2>$null | Out-String).Trim()
            if ($LASTEXITCODE -eq 0 -and $Resolved) {
                Add-UniquePathCandidate -List $Candidates -Path $Resolved
            }
        } catch {
            # Continue with the other explicit executable candidates.
        }
    }

    foreach ($Candidate in $Candidates) {
        if (Get-Python313Version -Python $Candidate) { return $Candidate }
    }
    return ""
}

function Ensure-AuditEnvironment {
    New-Item -ItemType Directory -Force -Path $ToolRoot | Out-Null
    $Requirements = Join-Path $Root "tools\requirements-audit.txt"

    # First recover any already-working isolated environment. This path must not
    # depend on a system Python command being visible in an elevated PowerShell.
    $ExistingCandidates = New-Object System.Collections.Generic.List[string]
    if (Test-Path -LiteralPath $AuditEnvPointer -PathType Leaf) {
        try {
            $PointerCandidate = (Get-Content -LiteralPath $AuditEnvPointer -Raw).Trim()
            $PointerCandidateFull = [IO.Path]::GetFullPath($PointerCandidate)
            $ToolRootFull = [IO.Path]::GetFullPath($ToolRoot)
            if ($PointerCandidateFull.StartsWith($ToolRootFull + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
                Add-UniquePathCandidate -List $ExistingCandidates -Path $PointerCandidateFull
            }
        } catch {
            # Ignore a stale/corrupt pointer and continue with directory discovery.
        }
    }
    Add-UniquePathCandidate -List $ExistingCandidates -Path $AuditEnv
    Add-UniquePathCandidate -List $ExistingCandidates -Path (Join-Path $ToolRoot "python-audit")
    Get-ChildItem -LiteralPath $ToolRoot -Directory -Filter "python-audit*" -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTime -Descending |
        ForEach-Object { Add-UniquePathCandidate -List $ExistingCandidates -Path $_.FullName }

    foreach ($Candidate in $ExistingCandidates) {
        $CandidatePython = Join-Path $Candidate "Scripts\python.exe"
        $CandidatePythonVersion = Get-Python313Version -Python $CandidatePython
        if (-not $CandidatePythonVersion) { continue }
        if (-not (Test-AuditEnvironmentContent -EnvironmentPath $Candidate -ExpectedPythonVersion $CandidatePythonVersion)) { continue }

        # Exact current pins and pip check passed, so it is safe to adopt an older
        # pre-fingerprint environment and stamp the current deterministic marker.
        $CandidateHash = Get-AuditEnvironmentHash -Requirements $Requirements -PythonVersion $CandidatePythonVersion
        $CandidateMarker = Join-Path $Candidate ".iris-requirements-sha256"
        Set-Content -LiteralPath $CandidateMarker -Value $CandidateHash -Encoding ASCII
        if (Test-AuditEnvironment -EnvironmentPath $Candidate -ExpectedHash $CandidateHash -ExpectedPythonVersion $CandidatePythonVersion) {
            Use-AuditEnvironment -EnvironmentPath $Candidate
            Set-Content -LiteralPath $AuditEnvPointer -Value $Candidate -Encoding UTF8
            Write-Host "Python audit environment: reused validated environment." -ForegroundColor Green
            return
        }
    }

    # No reusable environment exists. Only now require a bootstrap Python 3.13.
    # Search explicit user/system installs, py.exe, and any recoverable venv Python
    # because App Execution Aliases may disappear after UAC elevation.
    $BasePython = Find-Python313Executable -AuditEnvironmentCandidates $ExistingCandidates.ToArray()
    if (-not $BasePython) {
        throw "Python 3.13 executable is missing and no validated Python audit environment is available. Run INSTALL/UPDATE TOOLS, then retry."
    }
    $PythonVersion = Get-Python313Version -Python $BasePython
    if (-not $PythonVersion) { throw "Python 3.13 executable could not be verified." }
    $Hash = Get-AuditEnvironmentHash -Requirements $Requirements -PythonVersion $PythonVersion

    $PreferredAuditEnv = Join-Path $ToolRoot ("python-audit-" + $Hash.Substring(0, 16))
    if (Test-Path -LiteralPath $PreferredAuditEnv) {
        $NewAuditEnv = Join-Path $ToolRoot (
            "python-audit-" + $Hash.Substring(0, 16) + "-repair-" + [Guid]::NewGuid().ToString("N")
        )
    } else {
        $NewAuditEnv = $PreferredAuditEnv
    }
    $NewAuditPython = Join-Path $NewAuditEnv "Scripts\python.exe"
    $NewMarker = Join-Path $NewAuditEnv ".iris-requirements-sha256"

    try {
        Invoke-PythonVenv -Python $BasePython -Destination $NewAuditEnv
        $Installed = $false
        for ($Attempt = 1; $Attempt -le 3; $Attempt++) {
            try {
                Invoke-Checked $NewAuditPython @("-m", "pip", "install", "--disable-pip-version-check", "--requirement", $Requirements) 480
                $Installed = $true
                break
            } catch {
                if ($Attempt -eq 3) { throw }
                Start-Sleep -Seconds (5 * $Attempt)
            }
        }
        if (-not $Installed) { throw "Python audit environment installation failed." }
        Invoke-Checked $NewAuditPython @("-B", (Join-Path $Root "tools\verify_python_environment.py")) 120
        Invoke-Checked $NewAuditPython @("-m", "pip", "check") 120
        Set-Content -LiteralPath $NewMarker -Value $Hash -Encoding ASCII
        if (-not (Test-AuditEnvironment -EnvironmentPath $NewAuditEnv -ExpectedHash $Hash -ExpectedPythonVersion $PythonVersion)) {
            throw "Python audit environment verification failed after installation."
        }
        Use-AuditEnvironment -EnvironmentPath $NewAuditEnv
        Set-Content -LiteralPath $AuditEnvPointer -Value $NewAuditEnv -Encoding UTF8
    } catch {
        # Best-effort cleanup only. A locked partial venv must not hide the real failure.
        if (Test-Path -LiteralPath $NewAuditEnv) {
            Remove-Item -LiteralPath $NewAuditEnv -Recurse -Force -ErrorAction SilentlyContinue
        }
        throw
    }
}

function Test-ExactGoExecutable {
    param([string]$Executable)
    if (-not $Executable -or -not (Test-Path -LiteralPath $Executable -PathType Leaf)) { return $false }
    try {
        $Output = & $Executable version 2>&1
        $ExitCode = $LASTEXITCODE
        if ($ExitCode -ne 0) { return $false }
        $Line = (($Output | Out-String).Trim())
        return ($Line -match ("^go version go" + [Regex]::Escape($GoPin) + " windows/amd64$"))
    } catch {
        return $false
    }
}

function Install-PinnedGo {
    if (-not [Environment]::Is64BitOperatingSystem) {
        throw "The Windows release toolchain requires a 64-bit Windows host."
    }

    Update-ProcessPath
    $CurrentGo = Get-Command go -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($CurrentGo -and (Test-ExactGoExecutable -Executable $CurrentGo.Source)) { return }
    if (Test-ExactGoExecutable -Executable $PinnedGoExecutable) {
        Update-ProcessPath
        return
    }

    $MetadataUri = "https://go.dev/dl/?mode=json&include=all"
    Write-Host "+ official Go release metadata: $MetadataUri"
    try {
        $MetadataResponse = Invoke-WebRequest -UseBasicParsing -Uri $MetadataUri -TimeoutSec 60
        $Releases = $MetadataResponse.Content | ConvertFrom-Json
    } catch {
        throw "Official Go release metadata could not be downloaded or parsed."
    }

    $Release = @($Releases | Where-Object { $_.version -eq ("go" + $GoPin) }) | Select-Object -First 1
    if (-not $Release) {
        throw "Pinned Go $GoPin is not present in the official go.dev release metadata yet. The release gate remains blocked."
    }
    $ArchiveName = "go$GoPin.windows-amd64.zip"
    $FileMetadata = @($Release.files | Where-Object { $_.filename -eq $ArchiveName }) | Select-Object -First 1
    if (-not $FileMetadata) {
        throw "The official Windows amd64 archive for pinned Go $GoPin is not available yet. The release gate remains blocked."
    }
    $ExpectedHash = ([string]$FileMetadata.sha256).Trim().ToLowerInvariant()
    $ExpectedSize = [Int64]$FileMetadata.size
    if ($ExpectedHash -notmatch "^[0-9a-f]{64}$" -or $ExpectedSize -le 0) {
        throw "Official Go release metadata is incomplete or invalid."
    }

    $DownloadRoot = Join-Path $env:TEMP ("iris-go-download-" + [Guid]::NewGuid().ToString("N"))
    $Staging = Join-Path $ToolRoot ("go-install-" + [Guid]::NewGuid().ToString("N"))
    $Archive = Join-Path $DownloadRoot $ArchiveName
    New-Item -ItemType Directory -Force -Path $DownloadRoot, $Staging | Out-Null
    try {
        $ArchiveUri = "https://go.dev/dl/$ArchiveName"
        Write-Host "+ download $ArchiveUri"
        Invoke-WebRequest -UseBasicParsing -Uri $ArchiveUri -OutFile $Archive -TimeoutSec 300
        $ActualSize = (Get-Item -LiteralPath $Archive).Length
        if ($ActualSize -ne $ExpectedSize) {
            throw "Downloaded Go archive size does not match official release metadata."
        }
        $ActualHash = (Get-FileHash -LiteralPath $Archive -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($ActualHash -ne $ExpectedHash) {
            throw "Downloaded Go archive checksum does not match official release metadata."
        }
        Expand-Archive -LiteralPath $Archive -DestinationPath $Staging -Force
        $StagedGo = Join-Path $Staging "go\bin\go.exe"
        if (-not (Test-ExactGoExecutable -Executable $StagedGo)) {
            throw "Downloaded Go toolchain does not report the pinned Windows amd64 version."
        }

        if (Test-Path -LiteralPath $PinnedGoDirectory) {
            Remove-Item -LiteralPath $PinnedGoDirectory -Recurse -Force
        }
        Move-Item -LiteralPath $Staging -Destination $PinnedGoDirectory
    } finally {
        if (Test-Path -LiteralPath $Staging) { Remove-Item -LiteralPath $Staging -Recurse -Force -ErrorAction SilentlyContinue }
        if (Test-Path -LiteralPath $DownloadRoot) { Remove-Item -LiteralPath $DownloadRoot -Recurse -Force -ErrorAction SilentlyContinue }
    }

    Update-ProcessPath
    $InstalledGo = Get-Command go -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $InstalledGo -or -not (Test-ExactGoExecutable -Executable $InstalledGo.Source)) {
        throw "Pinned Go installation completed but the exact toolchain is not active in PATH."
    }
    Write-Host "Go: go$GoPin @ $($InstalledGo.Source)" -ForegroundColor Green
}

function Install-Gitleaks {
    $Destination = Join-Path $ToolRoot "gitleaks-$GitleaksPin"
    $Binary = Join-Path $Destination "gitleaks.exe"
    if (Test-Path -LiteralPath $Binary -PathType Leaf) {
        $ExistingVersion = Get-VersionLine $Binary @("version")
        if ($ExistingVersion -eq $GitleaksPin) {
            $env:PATH = $Destination + ";" + $env:PATH
            return
        }
        Remove-Item -LiteralPath $Destination -Recurse -Force
    } elseif (Test-Path -LiteralPath $Destination) {
        Remove-Item -LiteralPath $Destination -Recurse -Force
    }

    $Staging = Join-Path $ToolRoot ("gitleaks-install-" + [Guid]::NewGuid().ToString("N"))
    $DownloadRoot = Join-Path $env:TEMP ("iris-gitleaks-download-" + [Guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Force -Path $Staging, $DownloadRoot | Out-Null
    $ArchiveName = "gitleaks_${GitleaksPin}_windows_x64.zip"
    $ChecksumsName = "gitleaks_${GitleaksPin}_checksums.txt"
    $Archive = Join-Path $DownloadRoot $ArchiveName
    $Checksums = Join-Path $DownloadRoot $ChecksumsName
    $Base = "https://github.com/gitleaks/gitleaks/releases/download/v$GitleaksPin"
    try {
        Invoke-WebRequest -UseBasicParsing -Uri "$Base/$ArchiveName" -OutFile $Archive
        Invoke-WebRequest -UseBasicParsing -Uri "$Base/$ChecksumsName" -OutFile $Checksums
        $ExpectedLine = Get-Content -LiteralPath $Checksums | Where-Object { $_ -match [Regex]::Escape($ArchiveName) } | Select-Object -First 1
        if (-not $ExpectedLine) { throw "Official Gitleaks checksum is missing." }
        $Expected = ($ExpectedLine -split "\s+")[0].ToLowerInvariant()
        $Actual = (Get-FileHash -LiteralPath $Archive -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($Expected -ne $GitleaksWindowsX64Sha256) { throw "Official Gitleaks checksum does not match the pinned Windows x64 digest." }
        if ($Actual -ne $Expected -or $Actual -ne $GitleaksWindowsX64Sha256) { throw "Gitleaks archive checksum verification failed." }
        Expand-Archive -LiteralPath $Archive -DestinationPath $Staging -Force
        $StagedBinary = Join-Path $Staging "gitleaks.exe"
        if ((Get-VersionLine $StagedBinary @("version")) -ne $GitleaksPin) {
            throw "Downloaded Gitleaks executable does not match the pinned version."
        }
        Move-Item -LiteralPath $Staging -Destination $Destination
    } finally {
        if (Test-Path -LiteralPath $Staging) { Remove-Item -LiteralPath $Staging -Recurse -Force -ErrorAction SilentlyContinue }
        if (Test-Path -LiteralPath $DownloadRoot) { Remove-Item -LiteralPath $DownloadRoot -Recurse -Force -ErrorAction SilentlyContinue }
    }
    $env:PATH = $Destination + ";" + $env:PATH
}

function Test-GitleaksDetection {
    $GitleaksExecutable = Join-Path $PinnedGitleaksDirectory "gitleaks.exe"
    if ((Get-VersionLine $GitleaksExecutable @("version")) -ne $GitleaksPin) {
        throw "Pinned Gitleaks executable is missing or has the wrong version."
    }
    $ProbeDirectory = Join-Path $env:TEMP ("iris-gitleaks-probe-" + [Guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Force -Path $ProbeDirectory | Out-Null
    try {
        $SyntheticSecret = "ghp_" + "wA9mK2pLxN4vRtQzY6bC8dEfGhJlM0oPq1rS"
        Set-Content -LiteralPath (Join-Path $ProbeDirectory "probe.txt") -Value ("token=" + $SyntheticSecret) -Encoding ASCII
        $Result = Invoke-CapturedNativeProcess `
            -File $GitleaksExecutable `
            -Arguments @("dir", "--no-banner", "--redact", "--exit-code", "37", $ProbeDirectory) `
            -WorkingDirectory $Root `
            -TimeoutSeconds 60
        if ($Result.TimedOut -or $Result.ExitCode -ne 37) {
            throw "Gitleaks functional detection self-test failed."
        }
    } finally {
        Remove-Item -LiteralPath $ProbeDirectory -Recurse -Force -ErrorAction SilentlyContinue
    }
    Write-Host "Gitleaks detection self-test: PASS" -ForegroundColor Green
}

function ConvertFrom-AnsiToolOutput {
    param([string]$Text)
    if (-not $Text) { return "" }
    $Escape = [string][char]27
    $CsiPattern = [Regex]::Escape($Escape) + "\[[0-?]*[ -/]*[@-~]"
    return [Regex]::Replace($Text, $CsiPattern, "")
}

function Test-GitleaksHistoryProof {
    param([string]$Text)
    $PlainText = ConvertFrom-AnsiToolOutput $Text
    return $PlainText -match "(?im)(?:^|\s)[1-9][0-9]*\s+commits\s+scanned\."
}

function Invoke-GitleaksHistoryScan {
    param([int]$TimeoutSeconds = 600)
    $GitleaksExecutable = Join-Path $PinnedGitleaksDirectory "gitleaks.exe"
    if ((Get-VersionLine $GitleaksExecutable @("version")) -ne $GitleaksPin) {
        throw "Pinned Gitleaks executable is missing or has the wrong version."
    }
    Write-Host "+ gitleaks git --no-banner --redact --log-level info ."
    $Result = Invoke-CapturedNativeProcess `
        -File $GitleaksExecutable `
        -Arguments @("git", "--no-banner", "--redact", "--log-level", "info", ".") `
        -WorkingDirectory $Root `
        -TimeoutSeconds $TimeoutSeconds
    $Combined = (([string]$Result.Stdout) + [Environment]::NewLine + ([string]$Result.Stderr)).Trim()
    if ($Combined) {
        $SafeOutput = ConvertTo-SafeToolOutput $Combined
        if ($SafeOutput) { Write-Host $SafeOutput }
    }
    if ($Result.TimedOut) { throw "Gitleaks Git-history scan timed out." }
    if ($Result.ExitCode -ne 0) { throw "Gitleaks Git-history scan failed with exit code $($Result.ExitCode)." }
    if (-not (Test-GitleaksHistoryProof $Combined)) {
        throw "Gitleaks Git-history scan did not prove that any commits were scanned."
    }
}

function Show-ToolTable {
    Update-ProcessPath
    if ((Test-Path -LiteralPath (Join-Path $PinnedGitleaksDirectory "gitleaks.exe")) -and
        (($env:PATH -split ";") -notcontains $PinnedGitleaksDirectory)) {
        $env:PATH = $PinnedGitleaksDirectory + ";" + $env:PATH
    }
    $AdministratorActual = if (Test-IsAdministrator) { "Administrator" } else { "Standard user" }
    $PowerShellActual = $PSVersionTable.PSVersion.ToString() + " / " + $AdministratorActual
    $GitActual = Get-VersionLine "git" @("--version")
    $GitHubCLIActual = Get-VersionLine "gh" @("--version")
    $GoActual = if (Get-Command go -ErrorAction SilentlyContinue) { (& go env GOVERSION).Trim() } else { "MISSING" }
    $AuditPythonVersion = Get-Python313Version -Python $AuditPython
    $PythonActual = if ($AuditPythonVersion) { "$AuditPythonVersion @ $AuditPython" } else { "MISSING" }
    $NodeActual = Get-VersionLine "node" @("--version")
    $WailsInfo = Get-WailsInfo
    $WailsActual = if ($WailsInfo.Path) { "$($WailsInfo.Version) @ $($WailsInfo.Path)" } else { $WailsInfo.Version }
    $StaticcheckInfo = Get-StaticcheckInfo
    $StaticcheckActual = if ($StaticcheckInfo.Path) { "$($StaticcheckInfo.Version) @ $($StaticcheckInfo.Path)" } else { $StaticcheckInfo.Version }
    $GovulncheckInfo = Get-GovulncheckInfo
    $GovulncheckActual = if ($GovulncheckInfo.Path) { "$($GovulncheckInfo.Version) @ $($GovulncheckInfo.Path)" } else { $GovulncheckInfo.Version }
    $GitleaksActual = Get-VersionLine (Join-Path $PinnedGitleaksDirectory "gitleaks.exe") @("version")
    $RuffPin = Get-RequirementPin "ruff"
    $BanditPin = Get-RequirementPin "bandit"
    $PipPin = Get-RequirementPin "pip"
    $PipAuditPin = Get-RequirementPin "pip-audit"
    $PlaywrightPin = Get-RequirementPin "playwright"
    $PyYAMLPin = Get-RequirementPin "pyyaml"
    $RuffActual = Get-AuditPackageVersion "ruff"
    $BanditActual = Get-AuditPackageVersion "bandit"
    $PipActual = Get-AuditPackageVersion "pip"
    $PipAuditActual = Get-AuditPackageVersion "pip-audit"
    $PlaywrightActual = Get-AuditPackageVersion "playwright"
    $PyYAMLActual = Get-AuditPackageVersion "pyyaml"
    $WebView2Actual = Get-WebView2Version
    $PlaywrightBrowser = "MISSING"
    if (Test-Path -LiteralPath $AuditPython) {
        $BrowserList = (& $AuditPython -m playwright install --list 2>&1 | Out-String)
        if ($LASTEXITCODE -eq 0 -and $BrowserList -match "chromium") { $PlaywrightBrowser = "installed" }
    }
    $Rows = @(
        [pscustomobject]@{ Tool = "PowerShell"; Required = ">=5.1 / Administrator"; Actual = $PowerShellActual; Status = (Get-ToolStatus ($PSVersionTable.PSVersion -ge [Version]"5.1" -and (Test-IsAdministrator))) },
        [pscustomobject]@{ Tool = "Git"; Required = "available"; Actual = $GitActual; Status = (Get-ToolStatus ($GitActual -notin @("MISSING", "FAIL"))) },
        [pscustomobject]@{ Tool = "GitHub CLI"; Required = "available"; Actual = $GitHubCLIActual; Status = (Get-ToolStatus ($GitHubCLIActual -notin @("MISSING", "FAIL"))) },
        [pscustomobject]@{ Tool = "Go"; Required = "go$GoPin"; Actual = $GoActual; Status = (Get-ToolStatus ($GoActual -eq "go$GoPin")) },
        [pscustomobject]@{ Tool = "Audit Python"; Required = "3.13.x isolated"; Actual = $PythonActual; Status = (Get-ToolStatus ([bool]$AuditPythonVersion)) },
        [pscustomobject]@{ Tool = "Node"; Required = "24.x"; Actual = $NodeActual; Status = (Get-ToolStatus ($NodeActual -match "^v24\.")) },
        [pscustomobject]@{ Tool = "Wails"; Required = $WailsPin; Actual = $WailsActual; Status = (Get-ToolStatus ($WailsInfo.Version -eq $WailsPin)) },
        [pscustomobject]@{ Tool = "Staticcheck"; Required = $StaticcheckPin; Actual = $StaticcheckActual; Status = (Get-ToolStatus ($StaticcheckInfo.Version -eq $StaticcheckPin)) },
        [pscustomobject]@{ Tool = "Govulncheck"; Required = $GovulncheckPin; Actual = $GovulncheckActual; Status = (Get-ToolStatus ($GovulncheckInfo.Version -eq $GovulncheckPin)) },
        [pscustomobject]@{ Tool = "Gitleaks"; Required = $GitleaksPin; Actual = $GitleaksActual; Status = (Get-ToolStatus ($GitleaksActual -eq $GitleaksPin)) },
        [pscustomobject]@{ Tool = "Ruff"; Required = $RuffPin; Actual = $RuffActual; Status = (Get-ToolStatus ($RuffActual -eq $RuffPin)) },
        [pscustomobject]@{ Tool = "Bandit"; Required = $BanditPin; Actual = $BanditActual; Status = (Get-ToolStatus ($BanditActual -eq $BanditPin)) },
        [pscustomobject]@{ Tool = "pip"; Required = $PipPin; Actual = $PipActual; Status = (Get-ToolStatus ($PipActual -eq $PipPin)) },
        [pscustomobject]@{ Tool = "pip-audit"; Required = $PipAuditPin; Actual = $PipAuditActual; Status = (Get-ToolStatus ($PipAuditActual -eq $PipAuditPin)) },
        [pscustomobject]@{ Tool = "Playwright"; Required = $PlaywrightPin; Actual = $PlaywrightActual; Status = (Get-ToolStatus ($PlaywrightActual -eq $PlaywrightPin)) },
        [pscustomobject]@{ Tool = "PyYAML"; Required = $PyYAMLPin; Actual = $PyYAMLActual; Status = (Get-ToolStatus ($PyYAMLActual -eq $PyYAMLPin)) },
        [pscustomobject]@{ Tool = "Chromium"; Required = "Playwright cache"; Actual = $PlaywrightBrowser; Status = (Get-ToolStatus ($PlaywrightBrowser -eq "installed")) },
        [pscustomobject]@{ Tool = "WebView2"; Required = "Evergreen Runtime"; Actual = $WebView2Actual; Status = (Get-ToolStatus ($WebView2Actual -ne "MISSING")) }
    )
    $Rows | Format-Table -AutoSize | Out-Host
    return -not ($Rows.Status -contains "FAIL")
}

function Install-Tools {
    if (-not (Test-IsAdministrator)) {
        throw "Administrator rights are required for tool installation. Run the root IrisTools.ps1 launcher to request UAC elevation."
    }
    if (-not (Get-Command winget -ErrorAction SilentlyContinue)) {
        throw "winget is required to install missing system tools."
    }
    Update-ProcessPath
    $AuditEnvironmentReady = $false
    try {
        Ensure-AuditEnvironment
        $AuditEnvironmentReady = $true
    } catch {
        if ($_.Exception.Message -notlike "Python 3.13 executable is missing and no validated Python audit environment is available.*") {
            throw
        }
    }
    if ((Get-VersionLine "git" @("--version")) -in @("MISSING", "FAIL")) { Invoke-Checked "winget" @("install", "--exact", "--id", "Git.Git", "--accept-package-agreements", "--accept-source-agreements") 900 }
    if (-not (Get-Command gh -ErrorAction SilentlyContinue)) { Invoke-Checked "winget" @("install", "--exact", "--id", "GitHub.cli", "--accept-package-agreements", "--accept-source-agreements") 900 }
    if (-not $AuditEnvironmentReady -and (Get-VersionLine "python" @("--version")) -notmatch "^Python 3\.13(?:\.|$)") { Invoke-Checked "winget" @("install", "--exact", "--id", "Python.Python.3.13", "--accept-package-agreements", "--accept-source-agreements") 900 }
    if ((Get-VersionLine "node" @("--version")) -notmatch "^v24\.") { Invoke-Checked "winget" @("install", "--exact", "--id", "OpenJS.NodeJS.LTS", "--accept-package-agreements", "--accept-source-agreements") 900 }
    Install-PinnedGo
    if ((Get-WebView2Version) -eq "MISSING") { Invoke-Checked "winget" @("install", "--exact", "--id", "Microsoft.EdgeWebView2Runtime", "--accept-package-agreements", "--accept-source-agreements") 900 }
    Update-ProcessPath
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw "Restart the terminal after installing Go, then run Install again." }
    $GoBin = (& go env GOBIN).Trim()
    if (-not $GoBin) { $GoBin = Join-Path ((& go env GOPATH).Trim()) "bin" }
    if ($env:PATH -notlike "*$GoBin*") { $env:PATH = $GoBin + ";" + $env:PATH }
    $WailsInfo = Get-WailsInfo
    if ($WailsInfo.Version -ne $WailsPin) {
        Invoke-Checked "go" @("install", "github.com/wailsapp/wails/v2/cmd/wails@$WailsPin") 480
        $WailsInfo = Get-WailsInfo
        if ($WailsInfo.Version -ne $WailsPin) {
            throw "Wails installation completed but pinned module metadata is not $WailsPin."
        }
    }
    if ((Get-StaticcheckVersion) -ne $StaticcheckPin) { Invoke-Checked "go" @("install", "honnef.co/go/tools/cmd/staticcheck@$StaticcheckPin") 480 }
    if ((Get-GovulncheckVersion) -ne $GovulncheckPin) { Invoke-Checked "go" @("install", "golang.org/x/vuln/cmd/govulncheck@$GovulncheckPin") 480 }
    Install-Gitleaks
    if (-not $AuditEnvironmentReady) { Ensure-AuditEnvironment }
    Invoke-Checked $AuditPython @("-m", "playwright", "install", "chromium") 900
    if (-not (Show-ToolTable)) { throw "One or more required tools are missing or incompatible. Restart the terminal if system tools were installed." }
}

function Assert-CleanTree {
    Push-Location $Root
    try {
        # A source archive can be created on Unix with core.filemode=true. After the
        # archive is extracted on NTFS, Git for Windows cannot preserve Unix execute
        # bits and otherwise reports executable shell scripts as modified even when
        # their contents are byte-for-byte identical to the index.
        if ($env:OS -eq "Windows_NT") {
            & git config --local core.filemode false
            if ($LASTEXITCODE -ne 0) {
                throw "Git could not configure Windows file-mode handling."
            }
        }
        $Status = (& git status "--porcelain=v1" "--untracked-files=all")
        if ($LASTEXITCODE -ne 0) { throw "Git status failed." }
        if ($Status) { throw "The Git working tree must be clean." }
        $Branch = (& git branch --show-current).Trim()
        if ($LASTEXITCODE -ne 0) { throw "Git branch detection failed." }
        if ($Branch -ne "main") { throw "Release operations require branch main." }
    } finally { Pop-Location }
}

function Test-Release {
    Assert-CleanTree
    Test-WindowsTooling
    Ensure-AuditEnvironment
    Push-Location $Root
    try {
        if ((& go env GOVERSION).Trim() -ne "go$GoPin") { throw "Go version mismatch." }
        $WailsExecutable = Get-PinnedWailsExecutable
        Write-Host "Wails: $WailsPin @ $WailsExecutable"
        $BeforeHead = (& git rev-parse HEAD).Trim()
        Invoke-Checked $AuditPython @("-B", "tools/repository_audit.py") 120
        $GoFiles = Get-ChildItem -LiteralPath $Root -Filter "*.go" -File | ForEach-Object { $_.FullName }
        $Unformatted = (& gofmt -l -- $GoFiles)
        if ($LASTEXITCODE -ne 0) { throw "gofmt failed." }
        if ($Unformatted) { throw "Go source is not formatted." }
        Invoke-Checked "go" @("mod", "verify") 180
        Invoke-Checked "go" @("mod", "tidy", "-diff") 180
        Invoke-Checked "go" @("build", "-trimpath", "-o", (Join-Path $env:TEMP "iris-build-probe.exe"), ".") 600
        Invoke-Checked "go" @("test", "-count=1", "./...") 900
        Invoke-Checked "go" @("vet", "./...") 600
        Invoke-Staticcheck @("./...")
        Invoke-Govulncheck
        Invoke-Checked $AuditPython @("-B", "-m", "unittest", "discover", "-s", "tools", "-p", "test_*.py") 600
        Invoke-Checked (Join-Path $AuditEnv "Scripts\ruff.exe") @("check", "--no-cache", ".") 300
        Invoke-Checked (Join-Path $AuditEnv "Scripts\ruff.exe") @("format", "--check", "--no-cache", ".") 300
        Invoke-Checked (Join-Path $AuditEnv "Scripts\bandit.exe") @("-q", "-r", "tools", "-x", "tools/test_*.py") 300
        Invoke-Checked (Join-Path $AuditEnv "Scripts\pip-audit.exe") @("--local", "--cache-dir", (Join-Path $AuditEnv "pip-audit-cache")) 600
        Invoke-Checked $AuditPython @("-B", "tools/validate_workflows.py") 120
        Invoke-Checked "node" @("--check", "web/app.js") 120
        foreach ($Audit in @("data_presentation_audit.py", "frontend_smoke_test.py")) {
            Invoke-Checked $AuditPython @("-B", ("tools/" + $Audit)) 900
        }
        $GitleaksExecutable = Join-Path $PinnedGitleaksDirectory "gitleaks.exe"
        Test-GitleaksDetection
        Invoke-Checked $GitleaksExecutable @("dir", "--no-banner", "--redact", ".") 600
        Invoke-GitleaksHistoryScan 600
        Invoke-Checked $AuditPython @("-B", "tools/repository_audit.py") 120
        Assert-CleanTree
        if ((& git rev-parse HEAD).Trim() -ne $BeforeHead) { throw "HEAD changed during the RELEASE gate." }
        Invoke-Checked $AuditPython @("-B", "tools/release_fingerprint.py", "--write") 120
        Write-Host "RELEASE gate: PASS" -ForegroundColor Green
    } finally { Pop-Location }
}

function Clear-BuildGenerated {
    foreach ($Path in @("build\bin", "build\generated", "web\wailsjs")) {
        $Target = Join-Path $Root $Path
        if (Test-Path -LiteralPath $Target) { Remove-Item -LiteralPath $Target -Recurse -Force }
    }
    $AppIcon = Join-Path $Root "build\appicon.png"
    if (Test-Path -LiteralPath $AppIcon) { Remove-Item -LiteralPath $AppIcon -Force }
}

function Build-Release {
    Assert-CleanTree
    Ensure-AuditEnvironment
    $WailsExecutable = Get-PinnedWailsExecutable
    Push-Location $Root
    try {
        Invoke-Checked $AuditPython @("-B", "tools/release_fingerprint.py", "--verify") 120
        if (-not $OutputDirectory) { $script:OutputDirectory = Join-Path (Split-Path $Root -Parent) "iris-online-database-release-$Version" }
        $OutputFull = [IO.Path]::GetFullPath($OutputDirectory)
        if ($OutputFull.StartsWith($Root + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw "Release output must be outside the source tree." }
        New-Item -ItemType Directory -Force -Path $OutputFull | Out-Null
        $Head = (& git rev-parse HEAD).Trim()
        $Targets = @(
            [pscustomobject]@{ Platform = "windows/amd64"; Suffix = "x64"; LevelName = "GOAMD64"; LevelValue = "v1" },
            [pscustomobject]@{ Platform = "windows/386"; Suffix = "x86"; LevelName = "GO386"; LevelValue = "sse2" },
            [pscustomobject]@{ Platform = "windows/arm64"; Suffix = "arm64"; LevelName = "GOARM64"; LevelValue = "v8.0" }
        )
        $EnvironmentNames = @("CGO_ENABLED", "GOAMD64", "GO386", "GOARM64")
        $SavedEnvironment = @{}
        foreach ($Name in $EnvironmentNames) {
            $SavedEnvironment[$Name] = [Environment]::GetEnvironmentVariable($Name, "Process")
        }
        $Artifacts = @()
        foreach ($Target in $Targets) {
            $Artifact = Join-Path $OutputFull "IrisOnlineDB-$Version-Windows-$($Target.Suffix).exe"
            $Artifacts += $Artifact
            Remove-Item -LiteralPath $Artifact -Force -ErrorAction SilentlyContinue
        }
        Remove-Item -LiteralPath (Join-Path $OutputFull "SHA256SUMS.txt") -Force -ErrorAction SilentlyContinue
        Clear-BuildGenerated
        try {
            foreach ($Target in $Targets) {
                Remove-Item Env:\GOAMD64 -ErrorAction SilentlyContinue
                Remove-Item Env:\GO386 -ErrorAction SilentlyContinue
                Remove-Item Env:\GOARM64 -ErrorAction SilentlyContinue
                $env:CGO_ENABLED = "0"
                Set-Item -Path "Env:$($Target.LevelName)" -Value $Target.LevelValue
                Invoke-Checked $WailsExecutable @(
                    "build",
                    "-platform", $Target.Platform,
                    "-webview2", "embed",
                    "-trimpath",
                    "-clean",
                    "-skipbindings",
                    "-s",
                    "-nosyncgomod",
                    "-m",
                    "-o", "IrisOnlineDatabase.exe",
                    "-ldflags", "-buildid= -X main.appVersion=$Version -X main.releaseMarker=IrisOnlineRelease/$Version/$Head"
                ) 1200
                $Artifact = Join-Path $OutputFull "IrisOnlineDB-$Version-Windows-$($Target.Suffix).exe"
                Copy-Item -LiteralPath (Join-Path $Root "build\bin\IrisOnlineDatabase.exe") -Destination $Artifact -Force
            }
        } finally {
            foreach ($Name in $EnvironmentNames) {
                $Value = $SavedEnvironment[$Name]
                if ($null -eq $Value) {
                    Remove-Item -Path "Env:$Name" -ErrorAction SilentlyContinue
                } else {
                    Set-Item -Path "Env:$Name" -Value $Value
                }
            }
            Clear-BuildGenerated
        }
        $ChecksumLines = foreach ($Artifact in $Artifacts) {
            $Hash = (Get-FileHash -LiteralPath $Artifact -Algorithm SHA256).Hash.ToLowerInvariant()
            "$Hash  $(Split-Path $Artifact -Leaf)"
        }
        Set-Content -LiteralPath (Join-Path $OutputFull "SHA256SUMS.txt") -Value $ChecksumLines -Encoding ASCII
        Assert-CleanTree
        Invoke-Checked $AuditPython @("-B", "tools/verify_release_assets.py", "--directory", $OutputFull, "--version", $Version) 120
        Invoke-Checked $AuditPython @("-B", "tools/verify_executables.py", "--directory", $OutputFull, "--version", $Version) 120
        Invoke-Checked $AuditPython @("-B", "tools/verify_windows_resources.py", "--directory", $OutputFull, "--version", $Version) 120
        if ((& git rev-parse HEAD).Trim() -ne $Head) { throw "HEAD changed during release artifact build." }
        Invoke-Checked $AuditPython @("-B", "tools/release_fingerprint.py", "--verify") 120
        Write-Host "Release build: PASS (Windows x64, x86, arm64)" -ForegroundColor Green
        Write-Host $OutputFull
    } finally { Pop-Location }
}

function Prepare-Release {
    # No publishable artifact is created until the complete strict gate has
    # succeeded and written a fingerprint for this exact HEAD/source tree.
    Test-Release
    Build-Release
    Write-Host "PREPARE RELEASE: PASS" -ForegroundColor Green
}

function Invoke-GitFetchMain {
    Invoke-Checked "git" @("fetch", "--prune", "origin", "main") 300
}

function Assert-ReleaseSigningIdentity {
    $GitName = (& git config --get user.name | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $GitName -ne $ReleaseGitName) { throw "Git release user.name is not configured as required." }
    $GitEmail = (& git config --get user.email | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $GitEmail -ne $ReleaseGitEmail) { throw "Git release user.email is not configured as required." }
    if (-not (Test-Path -LiteralPath $ReleaseGpgExecutable -PathType Leaf)) { throw "Required Git for Windows GPG executable is missing." }
    $Result = Invoke-CapturedNativeProcess `
        -File $ReleaseGpgExecutable `
        -Arguments @("--batch", "--with-colons", "--list-secret-keys", $ReleaseGpgFingerprint) `
        -WorkingDirectory $Root `
        -TimeoutSeconds 30
    if ($Result.TimedOut -or $Result.ExitCode -ne 0) { throw "Configured GPG release secret key could not be verified." }
    $FingerprintFound = $false
    foreach ($Line in ([string]$Result.Stdout -split "`r?`n")) {
        $Fields = $Line -split ":"
        if ($Fields.Count -gt 9 -and $Fields[0] -eq "fpr" -and $Fields[9] -eq $ReleaseGpgFingerprint) {
            $FingerprintFound = $true
            break
        }
    }
    if (-not $FingerprintFound) { throw "Configured GPG release fingerprint is unavailable in the secret keyring." }
}

function Assert-ReleaseTag {
    param(
        [Parameter(Mandatory = $true)][string]$Tag,
        [Parameter(Mandatory = $true)][string]$Head
    )
    $TagCommit = (& git rev-list -n 1 $Tag | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or -not $TagCommit) { throw "Release tag target could not be resolved." }
    if ($TagCommit -ne $Head) { throw "Release tag does not point to the tested HEAD." }

    $GitExecutable = Resolve-NativeExecutablePath -File "git"
    $VerifyResult = Invoke-CapturedNativeProcess `
        -File $GitExecutable `
        -Arguments @("-c", "gpg.program=$ReleaseGpgExecutable", "verify-tag", "--raw", $Tag) `
        -WorkingDirectory $Root `
        -TimeoutSeconds 60
    if ($VerifyResult.TimedOut -or $VerifyResult.ExitCode -ne 0) { throw "Release tag signature verification failed." }
    $SignatureOutput = ([string]$VerifyResult.Stdout) + "`n" + ([string]$VerifyResult.Stderr)
    $FingerprintPattern = "(?m)^\[GNUPG:\] VALIDSIG " + [regex]::Escape($ReleaseGpgFingerprint) + "(?:\s|$)"
    if ($SignatureOutput -notmatch $FingerprintPattern) { throw "Release tag was not signed by the configured release fingerprint." }
}

function Get-RemoteReleaseTagObjectId {
    param([Parameter(Mandatory = $true)][string]$Tag)
    $GitExecutable = Resolve-NativeExecutablePath -File "git"
    $Result = Invoke-CapturedNativeProcess `
        -File $GitExecutable `
        -Arguments @("ls-remote", "--exit-code", "--tags", "origin", "refs/tags/$Tag") `
        -WorkingDirectory $Root `
        -TimeoutSeconds 60
    if ($Result.TimedOut) { throw "Remote release tag query timed out." }
    if ($Result.ExitCode -eq 2) { return $null }
    if ($Result.ExitCode -ne 0) { throw "Remote release tag state could not be verified." }
    $Lines = @(([string]$Result.Stdout -split "`r?`n") | Where-Object { $_.Trim() })
    if ($Lines.Count -ne 1) { throw "Remote release tag query returned an unexpected result." }
    $Fields = $Lines[0] -split "\s+"
    if ($Fields.Count -lt 2 -or $Fields[1] -ne "refs/tags/$Tag" -or $Fields[0] -notmatch "^[0-9a-fA-F]{40,64}$") {
        throw "Remote release tag response was invalid."
    }
    return $Fields[0].ToLowerInvariant()
}

function Ensure-ReleaseTag {
    param(
        [Parameter(Mandatory = $true)][string]$Tag,
        [Parameter(Mandatory = $true)][string]$Head
    )
    $RemoteTagObject = Get-RemoteReleaseTagObjectId -Tag $Tag
    & git show-ref --verify --quiet "refs/tags/$Tag"
    $LocalTagExists = $LASTEXITCODE -eq 0

    if ($RemoteTagObject -and -not $LocalTagExists) {
        Invoke-Checked "git" @("fetch", "origin", "refs/tags/$Tag:refs/tags/$Tag") 120
        $LocalTagExists = $true
    }

    if (-not $LocalTagExists) {
        Invoke-Checked "git" @(
            "-c", "gpg.program=$ReleaseGpgExecutable",
            "-c", "user.signingkey=$ReleaseGpgFingerprint",
            "tag", "-s", "-u", $ReleaseGpgFingerprint,
            "-m", "Iris Online Database $Version", $Tag
        ) 120
    }

    Assert-ReleaseTag -Tag $Tag -Head $Head
    $LocalTagObject = (& git rev-parse "refs/tags/$Tag" | Out-String).Trim().ToLowerInvariant()
    if ($LASTEXITCODE -ne 0 -or $LocalTagObject -notmatch "^[0-9a-f]{40,64}$") { throw "Local release tag object could not be resolved." }

    if ($RemoteTagObject) {
        if ($RemoteTagObject -ne $LocalTagObject) { throw "Local and origin release tag objects differ." }
        return
    }

    Invoke-Checked "git" @("push", "origin", "refs/tags/$Tag:refs/tags/$Tag") 300
    $RemoteTagObject = Get-RemoteReleaseTagObjectId -Tag $Tag
    if (-not $RemoteTagObject -or $RemoteTagObject -ne $LocalTagObject) { throw "Pushed release tag could not be verified on origin." }
}

function Get-GitHubCheckRuns {
    param([string]$Head)
    $GhExecutable = Resolve-NativeExecutablePath -File "gh"
    $Result = Invoke-CapturedNativeProcess `
        -File $GhExecutable `
        -Arguments @("api", "-H", "X-GitHub-Api-Version: 2026-03-10", "repos/$RepositorySlug/commits/$Head/check-runs?filter=latest&per_page=100") `
        -WorkingDirectory $Root `
        -TimeoutSeconds 120
    if ($Result.TimedOut -or $Result.ExitCode -ne 0) { throw "GitHub check-runs query failed." }
    try {
        $Payload = ConvertFrom-Json ([string]$Result.Stdout)
        $Runs = @($Payload.check_runs)
        if ([int]$Payload.total_count -gt $Runs.Count) {
            throw "GitHub check-runs response exceeded the verified page size."
        }
        return $Runs
    } catch {
        if ($_.Exception.Message -eq "GitHub check-runs response exceeded the verified page size.") { throw }
        throw "GitHub check-runs response was invalid."
    }
}

function Publish-Commit {
    Assert-CleanTree
    Ensure-AuditEnvironment
    Push-Location $Root
    try {
        Invoke-Checked $AuditPython @("-B", "tools/release_fingerprint.py", "--verify") 120
        $Head = (& git rev-parse HEAD | Out-String).Trim()
        if ($LASTEXITCODE -ne 0 -or -not $Head) { throw "Git HEAD could not be resolved." }
        Invoke-GitFetchMain
        Invoke-Checked "git" @("push", "origin", "main:main") 300
        Invoke-GitFetchMain
        $RemoteHead = (& git rev-parse "origin/main" | Out-String).Trim()
        if ($LASTEXITCODE -ne 0 -or $Head -ne $RemoteHead) { throw "Pushed commit does not match the tested HEAD." }
        Invoke-Checked $AuditPython @("-B", "tools/release_fingerprint.py", "--verify") 120
        Write-Host "Commit published. GitHub CI and CodeQL must be PASS before GITHUB RELEASE." -ForegroundColor Green
    } finally { Pop-Location }
}

function Create-Release {
    Assert-CleanTree
    Ensure-AuditEnvironment
    if (-not (Get-Command gh -ErrorAction SilentlyContinue)) { throw "GitHub CLI (gh) is required." }
    Push-Location $Root
    try {
        Invoke-Checked $AuditPython @("-B", "tools/release_fingerprint.py", "--verify") 120
        Invoke-GitFetchMain
        $Head = (& git rev-parse HEAD | Out-String).Trim()
        $RemoteHead = (& git rev-parse "origin/main" | Out-String).Trim()
        if ($LASTEXITCODE -ne 0 -or $Head -ne $RemoteHead) { throw "HEAD is not the published origin/main commit." }
        $Checks = Get-GitHubCheckRuns -Head $Head
        $RequiredChecks = @("Linux quality and security", "Go race detector", "Native Windows Wails release matrix", "Analyze (go)", "Analyze (python)")
        foreach ($Name in $RequiredChecks) {
            $Matches = @($Checks | Where-Object { $_.name -eq $Name })
            if (-not $Matches) { throw "Required GitHub check is missing: $Name" }
            $Latest = $Matches | Sort-Object -Property id -Descending | Select-Object -First 1
            if ($Latest.status -ne "completed") { throw "Required GitHub check is still running: $Name. Run GITHUB RELEASE again after GitHub Actions finishes." }
            if ($Latest.conclusion -ne "success") { throw "Required GitHub check failed: $Name" }
        }
        if (-not $OutputDirectory) { $script:OutputDirectory = Join-Path (Split-Path $Root -Parent) "iris-online-database-release-$Version" }
        $OutputFull = [IO.Path]::GetFullPath($OutputDirectory)
        Invoke-Checked $AuditPython @("-B", "tools/verify_release_assets.py", "--directory", $OutputFull, "--version", $Version) 120
        Invoke-Checked $AuditPython @("-B", "tools/verify_executables.py", "--directory", $OutputFull, "--version", $Version) 120
        Invoke-Checked $AuditPython @("-B", "tools/verify_windows_resources.py", "--directory", $OutputFull, "--version", $Version) 120
        Invoke-Checked $AuditPython @("-B", "tools/release_fingerprint.py", "--verify") 120
        Assert-ReleaseSigningIdentity

        $Tag = "v$Version"
        Ensure-ReleaseTag -Tag $Tag -Head $Head

        $ArtifactNames = @(
            "IrisOnlineDB-$Version-Windows-x64.exe",
            "IrisOnlineDB-$Version-Windows-x86.exe",
            "IrisOnlineDB-$Version-Windows-arm64.exe"
        )
        $Artifacts = @($ArtifactNames | ForEach-Object { Join-Path $OutputFull $_ })
        $Checksums = Join-Path $OutputFull "SHA256SUMS.txt"
        $ReleaseArguments = @("release", "create", $Tag) + $Artifacts + @(
            $Checksums,
            "--verify-tag",
            "--title", "Iris Online Database $Version",
            "--notes-file", "CHANGELOG.md"
        )
        Invoke-Checked "gh" $ReleaseArguments 600
        Write-Host "GitHub release $Tag created." -ForegroundColor Green
    } finally { Pop-Location }
}

function Open-ReleaseFolder {
    if (-not $OutputDirectory) { $script:OutputDirectory = Join-Path (Split-Path $Root -Parent) "iris-online-database-release-$Version" }
    $OutputFull = [IO.Path]::GetFullPath($OutputDirectory)
    if (-not (Test-Path -LiteralPath $OutputFull -PathType Container)) { throw "Release folder does not exist yet." }
    $Explorer = Join-Path $env:SystemRoot "explorer.exe"
    & $Explorer $OutputFull
    if ($LASTEXITCODE -ne 0) { throw "Explorer could not open the release folder." }
}

if ($Action -eq "Menu") {
    Write-Host "Iris Online Database $Version release tools"
    Write-Host "1 - PREPARE RELEASE"
    Write-Host "2 - PUSH COMMIT"
    Write-Host "3 - GITHUB RELEASE"
    Write-Host "4 - CHECK TOOLS"
    Write-Host "5 - INSTALL/UPDATE TOOLS"
    Write-Host "6 - OPEN RELEASE FOLDER"
    Write-Host "0 - EXIT"
    $Choice = Read-Host "Select"
    $Action = switch ($Choice) {
        "1" { "Prepare" }
        "2" { "Publish" }
        "3" { "Release" }
        "4" { "Check" }
        "5" { "Install" }
        "6" { "Open" }
        "0" { return }
        default { throw "Unknown selection." }
    }
}

switch ($Action) {
    "Check" { if (-not (Show-ToolTable)) { throw "Tool check: FAIL" } }
    "Install" { Install-Tools }
    "Prepare" { Prepare-Release }
    "Test" { Test-Release }
    "Build" { Build-Release }
    "Publish" { Publish-Commit }
    "Release" { Create-Release }
    "Open" { Open-ReleaseFolder }
    "SelfTest" { Test-WindowsTooling }
}
