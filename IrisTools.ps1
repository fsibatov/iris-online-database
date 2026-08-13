param(
    [ValidateSet("Menu", "Check", "Install", "Test", "Build", "Publish", "Release")]
    [string]$Action = "Menu",
    [string]$OutputDirectory = ""
)

$ErrorActionPreference = "Stop"
$Script = Join-Path $PSScriptRoot "scripts\windows\IrisTools.ps1"
if (-not (Test-Path -LiteralPath $Script -PathType Leaf)) {
    throw "scripts\windows\IrisTools.ps1 is missing."
}
& $Script -Action $Action -OutputDirectory $OutputDirectory
