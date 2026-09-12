import os
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
POWERSHELL = (
    str(
        Path(os.environ.get("SYSTEMROOT", r"C:\Windows"))
        / "System32/WindowsPowerShell/v1.0/powershell.exe"
    )
    if os.name == "nt"
    else shutil.which("pwsh")
)
PROBE = r"""
param([string]$Root, [string]$ToolRoot)
$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest
$Tokens = $null
$Errors = $null
$ScriptPath = Join-Path $Root "scripts/windows/IrisTools.ps1"
$Ast = [Management.Automation.Language.Parser]::ParseFile($ScriptPath, [ref]$Tokens, [ref]$Errors)
if ($Errors.Count) { throw "PowerShell source contains syntax errors." }
foreach ($Name in @("Resolve-NativeExecutablePath", "Initialize-WindowsLegacy")) {
    $Definitions = @($Ast.FindAll({
        param($Node)
        $Node -is [Management.Automation.Language.FunctionDefinitionAst] -and $Node.Name -eq $Name
    }, $true))
    if ($Definitions.Count -ne 1) { throw "Expected one definition of $Name." }
    Invoke-Expression $Definitions[0].Extent.Text
}
$AuditPython = "audit-python.exe"
function Get-Command {
    [CmdletBinding()]
    param([string]$Name, [string]$CommandType)
    if ($Name -ne "go" -or $CommandType -ne "Application") {
        throw "Unexpected executable lookup."
    }
    foreach ($Source in $script:Candidates) {
        [pscustomobject]@{ Source = $Source }
    }
}
function Invoke-Checked {
    param([string]$File, [string[]]$Arguments, [int]$TimeoutSeconds)
    if ($File -ne $AuditPython -or $TimeoutSeconds -ne 120) {
        throw "Unexpected preparation command."
    }
    $script:ReceivedArguments = @($Arguments)
}
$First = "C:\Program Files\Go\bin\go.exe"
$Second = "C:\Older Go\bin\go.exe"
$Cases = @(
    [pscustomobject]@{ Paths = @($First, $Second); Expected = $First }
    [pscustomobject]@{ Paths = @($Second, $First); Expected = $Second }
    [pscustomobject]@{ Paths = @($First); Expected = $First }
)
foreach ($Case in $Cases) {
    $script:Candidates = $Case.Paths
    $script:ReceivedArguments = @()
    $Result = @(Initialize-WindowsLegacy)
    if ($script:ReceivedArguments.Count -ne 6) {
        throw "Preparation must receive exactly six arguments."
    }
    if ($script:ReceivedArguments[2] -ne "--go" -or
        $script:ReceivedArguments[3] -cne $Case.Expected -or
        $script:ReceivedArguments[4] -ne "--directory") {
        throw "Preparation must receive the first Go executable as one argument."
    }
    if ($Result.Count -ne 1 -or -not $Result[0].Overlay.EndsWith("overlay.json")) {
        throw "Preparation must return one overlay descriptor."
    }
}
$script:Candidates = @()
$script:ReceivedArguments = @()
$MissingDetected = $false
try { Initialize-WindowsLegacy | Out-Null } catch { $MissingDetected = $true }
if (-not $MissingDetected -or $script:ReceivedArguments.Count) {
    throw "Missing Go must be detected before running Python."
}
Write-Output "Go executable resolution: PASS"
"""


class WindowsToolResolutionTests(unittest.TestCase):
    @unittest.skipUnless(POWERSHELL, "PowerShell is unavailable")
    def test_legacy_preparation_resolves_one_go_executable(self):
        with tempfile.TemporaryDirectory(prefix="iris-go-resolution-") as directory:
            probe = Path(directory) / "probe.ps1"
            probe.write_text(PROBE, encoding="utf-8-sig")
            result = subprocess.run(
                [
                    POWERSHELL,
                    "-NoLogo",
                    "-NoProfile",
                    "-NonInteractive",
                    "-ExecutionPolicy",
                    "Bypass",
                    "-File",
                    str(probe),
                    str(ROOT),
                    directory,
                ],
                capture_output=True,
                text=True,
                encoding="utf-8",
                errors="replace",
                timeout=60,
                check=False,
            )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("Go executable resolution: PASS", result.stdout)


if __name__ == "__main__":
    unittest.main()
