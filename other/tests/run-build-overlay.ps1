param([string]$GoExecutable = '')
$ErrorActionPreference = 'Stop'
$project = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$scratch = Join-Path $project 'temp\overlay-tests'
New-Item -ItemType Directory -Force -Path $scratch | Out-Null
$original = Join-Path $project 'plugin_main.go'
$originalHash = (Get-FileHash -LiteralPath $original -Algorithm SHA256).Hash
$alternate = Join-Path $scratch 'alternate plugin_main.go.txt'
$source = [IO.File]::ReadAllText($original, [Text.Encoding]::UTF8)
$replacement = [regex]::Replace($source, '(?m)(^\s*PluginName\s*=\s*)"(?:[^"\\]|\\.)*"', '$1"BeeNT Overlay Smoke"')
if ($replacement -eq $source) { throw 'test fixture could not replace PluginName' }
$markerReplacement = [regex]::Replace($replacement, '(?s)(func onInitialize\(\)\s*\{\s*logPlugin\(")(.*?)("\))', '$1$2 Overlay worker marker$3')
if ($markerReplacement -eq $replacement) { throw 'test fixture could not add worker marker' }
[IO.File]::WriteAllText($alternate, $markerReplacement, (New-Object Text.UTF8Encoding($false)))
Push-Location $project
$savedToolchain = $env:GOTOOLCHAIN
$savedRoot = $env:GOROOT
try {
    $buildArguments = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', './other/build.ps1', '-PluginSource', $alternate, '-DllName', 'BeeNT-Overlay-Smoke.dll')
    if ($GoExecutable) { $buildArguments += @('-GoExecutable', $GoExecutable) }
    & powershell.exe @buildArguments
    if ($LASTEXITCODE -ne 0) { throw 'overlay build failed' }
    if ((Get-FileHash -LiteralPath $original -Algorithm SHA256).Hash -ne $originalHash) { throw 'overlay build changed plugin_main.go' }
    $dllPath = Join-Path $project 'build\BeeNT-Overlay-Smoke.dll'
    . (Join-Path $project 'other\build-tools.ps1')
    Assert-BeeNTPEImports -Imports @(Get-BeeNTPEImports -DllPath $dllPath)
    Assert-BeeNTLegacyPEVersion -Header (Get-BeeNTPEHeader -Path $dllPath)
    $worker = Join-Path $scratch 'embedded-worker.exe'
    [IO.File]::WriteAllBytes($worker, (Get-BeeNTWorkerResource -DllPath $dllPath))
    Assert-BeeNTLegacyPEVersion -Header (Get-BeeNTPEHeader -Path $worker)
    $toolchain = Initialize-BeeNTToolchain -GoExecutable $GoExecutable -ToolchainCache (Join-Path $project 'temp\toolchains')
    $buildInfo = @(& $toolchain.Executable version -m $worker)
    if ($LASTEXITCODE -ne 0) { throw 'cannot inspect overlay DLL embedded worker' }
    Assert-BeeNTWorkerBuildInfo -BuildInfo $buildInfo
    if (Test-Path -LiteralPath ($dllPath + '.build.json')) { throw 'build published an unwanted manifest file' }
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File ./other/tests/run-go-host.ps1 -DllPath './build/BeeNT-Overlay-Smoke.dll' -ExpectedPluginName 'BeeNT Overlay Smoke' -ExpectedWorkerMarker 'Overlay worker marker'
    if ($LASTEXITCODE -ne 0) { throw 'overlay DLL failed production worker smoke test' }
    Write-Host 'PASS: alternate source path with spaces, production worker, original source unchanged, actual DLL worker verified as go1.20.14'
} finally {
    $env:GOTOOLCHAIN = $savedToolchain; $env:GOROOT = $savedRoot
    Pop-Location
}
