param(
    [ValidateSet("Menu", "Check", "Install", "Test", "Build", "Publish", "Release", "SelfTest")]
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
    Write-Host "0 - Check tools"
    Write-Host "1 - Install/update tools"
    Write-Host "2 - Strict RELEASE gate"
    Write-Host "3 - Build EXE and SHA256"
    Write-Host "4 - Push tested commit"
    Write-Host "5 - Create signed tag and GitHub release"
    $Choice = Read-Host "Select"
    $Action = switch ($Choice) {
        "0" { "Check" }
        "1" { "Install" }
        "2" { "Test" }
        "3" { "Build" }
        "4" { "Publish" }
        "5" { "Release" }
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
