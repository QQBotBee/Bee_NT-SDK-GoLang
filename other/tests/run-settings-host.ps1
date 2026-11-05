param([Parameter(Mandatory = $true)][string]$DllPath, [Parameter(Mandatory = $true)][string]$ExpectedPluginName)
$ErrorActionPreference = 'Stop'
$project = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$output = Join-Path $project 'temp\settings-host-tests'
$dll = (Resolve-Path -LiteralPath $DllPath).Path
New-Item -ItemType Directory -Force -Path $output | Out-Null
Push-Location $project
try {
    $hostExecutable = Join-Path $output 'settings_host.exe'
    & zig cc -target x86-windows-gnu -O2 -municode other/tests/settings_host.c -o $hostExecutable
    if ($LASTEXITCODE -ne 0) { throw 'settings host build failed' }
    & $hostExecutable $dll $ExpectedPluginName
    if ($LASTEXITCODE -ne 0) { throw 'final DLL settings window lifecycle failed' }
} finally { Pop-Location }
