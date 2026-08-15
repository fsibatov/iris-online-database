param(
    [ValidateSet("Menu", "Check", "Install", "Prepare", "Test", "Build", "Publish", "Release", "Open", "SelfTest")]
    [string]$Action = "Menu",
    [string]$OutputDirectory = "",
    [switch]$ElevatedSession
)

$ErrorActionPreference = "Stop"

function Test-IsAdministrator {
    $Identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $Principal = New-Object Security.Principal.WindowsPrincipal($Identity)
    return $Principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function ConvertTo-PowerShellLiteral {
    param([string]$Value)
    return "'" + $Value.Replace("'", "''") + "'"
}

function Restart-AsAdministrator {
    $Launcher = (Resolve-Path -LiteralPath $PSCommandPath).Path
    $Invocation = "& " + (ConvertTo-PowerShellLiteral $Launcher) + " -Action " + (ConvertTo-PowerShellLiteral $Action) + " -ElevatedSession"
    if ($OutputDirectory) {
        $Invocation += " -OutputDirectory " + (ConvertTo-PowerShellLiteral $OutputDirectory)
    }
    $EncodedCommand = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($Invocation))
    $WindowsPowerShell = Join-Path $env:SystemRoot "System32\WindowsPowerShell\v1.0\powershell.exe"
    try {
        $Process = Start-Process -FilePath $WindowsPowerShell -Verb RunAs -ArgumentList @(
            "-NoLogo",
            "-NoProfile",
            "-ExecutionPolicy", "Bypass",
            "-EncodedCommand", $EncodedCommand
        ) -WorkingDirectory $PSScriptRoot -Wait -PassThru
    } catch {
        throw "Administrator rights are required. UAC elevation was cancelled or failed."
    }
    return $Process.ExitCode
}

if ($Action -eq "Menu") {
    $Version = (Get-Content -LiteralPath (Join-Path $PSScriptRoot "VERSION") -Raw).Trim()
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

if ($Action -in @("Check", "Install") -and -not (Test-IsAdministrator)) {
    Write-Host "Requesting administrator rights through Windows UAC..." -ForegroundColor Yellow
    $ElevationExitCode = Restart-AsAdministrator
    if ($ElevationExitCode -ne 0) {
        throw "The elevated tool action failed. Review the administrator window output."
    }
    return
}

$Script = Join-Path $PSScriptRoot "scripts\windows\IrisTools.ps1"
if (-not (Test-Path -LiteralPath $Script -PathType Leaf)) {
    throw "scripts\windows\IrisTools.ps1 is missing."
}
$FailureMessage = ""
try {
    & $Script -Action $Action -OutputDirectory $OutputDirectory
} catch {
    $FailureMessage = $_.Exception.Message
}
if ($ElevatedSession) {
    Write-Host ""
    [void](Read-Host "Press Enter to close the administrator window")
}
if ($FailureMessage) {
    throw $FailureMessage
}
