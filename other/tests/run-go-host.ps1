param([Parameter(Mandatory = $true)][string]$DllPath, [string]$ExpectedPluginName = '', [string]$ExpectedWorkerMarker = '')
$ErrorActionPreference = 'Stop'
$project = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$output = Join-Path $project 'temp\go-host-tests'
$dll = (Resolve-Path -LiteralPath $DllPath).Path
New-Item -ItemType Directory -Force -Path $output | Out-Null
Push-Location $project
try {
    $hostExecutable = Join-Path $output 'go_host.exe'
    & zig cc -target x86-windows-gnu -O2 -municode other/tests/go_host.c -o $hostExecutable
    if ($LASTEXITCODE -ne 0) { throw 'production Go host build failed' }
    if ($ExpectedWorkerMarker) { & $hostExecutable $dll $ExpectedPluginName $ExpectedWorkerMarker }
    elseif ($ExpectedPluginName) { & $hostExecutable $dll $ExpectedPluginName }
    else { & $hostExecutable $dll }
    if ($LASTEXITCODE -ne 0) { throw 'production Go worker integration test failed' }
} finally { Pop-Location }
