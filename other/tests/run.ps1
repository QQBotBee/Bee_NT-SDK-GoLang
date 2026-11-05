param([string]$GoExecutable = '')
$ErrorActionPreference = 'Stop'
$project = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
. (Join-Path $project 'other\build-tools.ps1')
$output = Join-Path $project 'temp\bridge-tests'
New-Item -ItemType Directory -Force -Path $output | Out-Null
$savedGoos = $env:GOOS
$savedGoarch = $env:GOARCH
$savedCgo = $env:CGO_ENABLED
$savedToolchain = $env:GOTOOLCHAIN
$savedRoot = $env:GOROOT
$savedGowork = $env:GOWORK
Push-Location $project
try {
    $env:GOOS = $null; $env:GOARCH = $null; $env:CGO_ENABLED = '0'; $env:GOWORK = 'off'
    $toolchain = Initialize-BeeNTToolchain -GoExecutable $GoExecutable -ToolchainCache (Join-Path $project 'temp\toolchains')
    & $toolchain.Executable run ./other/buildmeta other/tests/fixture_plugin.txt $output
    if ($LASTEXITCODE -ne 0) { throw 'metadata generation failed' }
    & zig cc -target x86-windows-gnu -O2 other/tests/fixture_worker.c -o (Join-Path $output 'beent_go_worker.exe')
    if ($LASTEXITCODE -ne 0) { throw 'fixture worker build failed' }
    Push-Location $output
    try { & zig rc /c 65001 /fo worker.res worker.rc; if ($LASTEXITCODE -ne 0) { throw 'resource build failed' } } finally { Pop-Location }
    $windowsHeaders = Get-BeeNTZigWindowsHeaders
    $stackProbe = Join-Path $output 'beent_stack_probe.o'
    & zig cc -target x86-windows-gnu -c other/beent_stack_probe.S -o $stackProbe
    if ($LASTEXITCODE -ne 0) { throw 'bridge stack probe build failed' }
    & zig cc -target x86-windows-gnu -O2 -fno-builtin -nostdlib -shared -isystem $windowsHeaders -include other/beent_crt.h -DEVENT_TIMEOUT_MS=1000 -DSTOP_TIMEOUT_MS=100 -DWRITE_TIMEOUT_MS=200 other/beent_bridge.c $stackProbe (Join-Path $output 'worker.res') other/BeeNTPlugin.def "-I$output" -lkernel32 '-Wl,--entry,DllMain@12' -o (Join-Path $output 'fixture.dll')
    if ($LASTEXITCODE -ne 0) { throw 'bridge DLL build failed' }
    Assert-BeeNTPEImports -Imports @(Get-BeeNTPEImports -DllPath (Join-Path $output 'fixture.dll'))
    & zig cc -target x86-windows-gnu -O2 other/tests/host.c -o (Join-Path $output 'host.exe')
    if ($LASTEXITCODE -ne 0) { throw 'test host build failed' }
    & (Join-Path $output 'host.exe') (Join-Path $output 'fixture.dll')
    if ($LASTEXITCODE -ne 0) { throw 'bridge integration test failed' }
    & (Join-Path $output 'host.exe') (Join-Path $output 'fixture.dll') blocked-core
    if ($LASTEXITCODE -ne 0) { throw 'blocked core integration test failed' }
    & (Join-Path $output 'host.exe') (Join-Path $output 'fixture.dll') stalled-worker
    if ($LASTEXITCODE -ne 0) { throw 'stalled worker integration test failed' }
    & (Join-Path $output 'host.exe') (Join-Path $output 'fixture.dll') concurrent-disable
    if ($LASTEXITCODE -ne 0) { throw 'concurrent disable integration test failed' }
    & (Join-Path $output 'host.exe') (Join-Path $output 'fixture.dll') concurrent-uninstall
    if ($LASTEXITCODE -ne 0) { throw 'concurrent uninstall integration test failed' }
} finally {
    $env:GOOS = $savedGoos; $env:GOARCH = $savedGoarch; $env:CGO_ENABLED = $savedCgo
    $env:GOTOOLCHAIN = $savedToolchain; $env:GOROOT = $savedRoot; $env:GOWORK = $savedGowork
    Pop-Location
}
