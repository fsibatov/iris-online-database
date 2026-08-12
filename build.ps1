$ErrorActionPreference = "Stop"

$Version = "1.1.0"
$ExpectedGo = "go$((Get-Content (Join-Path $PSScriptRoot '.go-version') -Raw).Trim())"
$ActualGo = ((& go version) -split '\s+')[2]
$IsDiagnostic = $ActualGo -ne $ExpectedGo

if ($IsDiagnostic -and $env:IRIS_ALLOW_UNSUPPORTED_GO -ne "1") {
    throw "Требуется $ExpectedGo, но обнаружен $ActualGo. Для диагностической сборки задайте IRIS_ALLOW_UNSUPPORTED_GO=1."
}

if (-not $IsDiagnostic -and $env:IRIS_SKIP_CHECKS -eq "1") {
    throw "IRIS_SKIP_CHECKS=1 разрешён только для диагностической сборки. Сборка для публикации должна пройти все проверки."
}

$EnvironmentNames = @("CGO_ENABLED", "GOOS", "GOARCH", "GOAMD64", "GO386")
$SavedEnvironment = @{}
foreach ($Name in $EnvironmentNames) {
    $SavedEnvironment[$Name] = [Environment]::GetEnvironmentVariable($Name, "Process")
}

try {
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    Remove-Item Env:GOAMD64 -ErrorAction SilentlyContinue
    Remove-Item Env:GO386 -ErrorAction SilentlyContinue

    if ($env:IRIS_SKIP_CHECKS -ne "1") {
        $env:CGO_ENABLED = "0"
        & go test -count=1 ./...
        if ($LASTEXITCODE -ne 0) { throw "go test завершился с ошибкой." }

        $CompilerName = $env:CC
        if ([string]::IsNullOrWhiteSpace($CompilerName)) { $CompilerName = "gcc" }
        $Compiler = Get-Command $CompilerName -ErrorAction SilentlyContinue
        if ($null -eq $Compiler) {
            throw "Для go test -race требуется GCC/CGO. Установите GCC или укажите в CC доступный C-компилятор."
        }
        $env:CGO_ENABLED = "1"
        & go test -race -count=1 ./...
        if ($LASTEXITCODE -ne 0) { throw "go test -race завершился с ошибкой." }

        $env:CGO_ENABLED = "0"
        & go vet ./...
        if ($LASTEXITCODE -ne 0) { throw "go vet завершился с ошибкой." }
        & node --check web/app.js
        if ($LASTEXITCODE -ne 0) { throw "node --check завершился с ошибкой." }
        & python -B -m unittest discover -s tools -p "test_*.py"
        if ($LASTEXITCODE -ne 0) { throw "Python-тесты завершились с ошибкой." }
    }

    $ResourceSpecs = @(
        @{ Arch = "amd64"; File = "resource_windows_amd64.syso" },
        @{ Arch = "386"; File = "resource_windows_386.syso" },
        @{ Arch = "arm64"; File = "resource_windows_arm64.syso" }
    )
    foreach ($Resource in $ResourceSpecs) {
        & python (Join-Path $PSScriptRoot "tools\generate_windows_resources.py") `
            --icon (Join-Path $PSScriptRoot "resources\icon.ico") `
            --manifest (Join-Path $PSScriptRoot "resources\app.manifest") `
            --arch $Resource.Arch `
            --output (Join-Path $PSScriptRoot $Resource.File)
        if ($LASTEXITCODE -ne 0) { throw "Не удалось создать Windows-ресурс $($Resource.File)." }
    }

    $Output = Join-Path $PSScriptRoot "dist"
    New-Item -ItemType Directory -Force -Path $Output | Out-Null
    if ($IsDiagnostic) {
        $Marker = "IrisOnlineDiagnostic/$Version/$ActualGo"
        $NameSuffix = "-diagnostic-$ActualGo"
    } else {
        $Marker = "IrisOnlineRelease/$Version"
        $NameSuffix = ""
    }
    $ldflags = "-s -w -H windowsgui -buildid= -X main.appVersion=$Version -X main.releaseMarker=$Marker"
    $env:CGO_ENABLED = "0"
    $env:GOOS = "windows"

    $targets = @(
        @{ Arch = "amd64"; Name = "x64"; Extra = @{ GOAMD64 = "v1" } },
        @{ Arch = "386"; Name = "x86"; Extra = @{ GO386 = "softfloat" } },
        @{ Arch = "arm64"; Name = "arm64"; Extra = @{} }
    )

    foreach ($target in $targets) {
        Remove-Item Env:GOAMD64 -ErrorAction SilentlyContinue
        Remove-Item Env:GO386 -ErrorAction SilentlyContinue
        $env:GOARCH = $target.Arch
        foreach ($entry in $target.Extra.GetEnumerator()) {
            Set-Item -Path "Env:$($entry.Key)" -Value $entry.Value
        }
        $file = Join-Path $Output "IrisOnlineDB-$Version$NameSuffix-Windows-$($target.Name).exe"
        & go build -buildvcs=false -trimpath -ldflags $ldflags -o $file .
        if ($LASTEXITCODE -ne 0) { throw "Сборка $($target.Name) завершилась с ошибкой." }
    }

    if ($IsDiagnostic) {
        Write-Host "Собрано Iris Online ${Version}: диагностические Windows x64, x86 и ARM64 ($ActualGo)."
    } else {
        Write-Host "Собрано Iris Online ${Version}: Windows x64, x86 и ARM64 ($ActualGo)."
    }
}
finally {
    Remove-Item (Join-Path $PSScriptRoot "resource_windows_*.syso") -Force -ErrorAction SilentlyContinue
    foreach ($Name in $EnvironmentNames) {
        $Value = $SavedEnvironment[$Name]
        if ($null -eq $Value) {
            Remove-Item -Path "Env:$Name" -ErrorAction SilentlyContinue
        } else {
            Set-Item -Path "Env:$Name" -Value $Value
        }
    }
}
