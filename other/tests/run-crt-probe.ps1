$ErrorActionPreference = 'Stop'
$project = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
. (Join-Path $project 'other\build-tools.ps1')
$scratch = Join-Path $project 'temp\crt-tests'
New-Item -ItemType Directory -Force -Path $scratch | Out-Null
Push-Location $project
try {
    $windowsHeaders = Get-BeeNTZigWindowsHeaders
    $stackProbe = Join-Path $scratch 'beent_stack_probe.o'
    & zig cc -target x86-windows-gnu -c other/beent_stack_probe.S -o $stackProbe
    if ($LASTEXITCODE -ne 0) { throw 'freestanding stack probe compilation failed' }
    $abiProbe = Join-Path $scratch 'crt_stack_abi.o'
    & zig cc -target x86-windows-gnu -c other/tests/crt_stack_abi.S -o $abiProbe
    if ($LASTEXITCODE -ne 0) { throw 'freestanding stack ABI probe compilation failed' }
    $executable = Join-Path $scratch 'crt_probe.exe'
    & zig cc -target x86-windows-gnu -O2 -fno-builtin -nostdlib -isystem $windowsHeaders other/tests/crt_probe.c $stackProbe $abiProbe -lkernel32 '-Wl,--entry,test_start' -o $executable
    if ($LASTEXITCODE -ne 0) { throw 'freestanding regression probe compilation failed' }
    Assert-BeeNTPEImports -Imports @(Get-BeeNTPEImports -DllPath $executable)
    & $executable
    if ($LASTEXITCODE -ne 0) { throw 'freestanding regression probe failed' }
} finally { Pop-Location }
