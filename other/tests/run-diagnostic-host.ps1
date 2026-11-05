param([ValidateSet('all', 'stderr-exit', 'callback-error', 'invalid-resource')][string]$Case = 'all', [string]$GoExecutable = '')
$ErrorActionPreference = 'Stop'
$project = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
. (Join-Path $project 'other\build-tools.ps1')
$scratch = Join-Path $project 'temp\diagnostic-tests'
New-Item -ItemType Directory -Force -Path $scratch | Out-Null
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
    & $toolchain.Executable run ./other/buildmeta other/tests/fixture_plugin.txt $scratch
    if ($LASTEXITCODE -ne 0) { throw 'diagnostic metadata generation failed' }
    & zig cc -target x86-windows-gnu -static -O2 other/tests/diagnostic_worker.c -o (Join-Path $scratch 'beent_go_worker.exe')
    if ($LASTEXITCODE -ne 0) { throw 'diagnostic fixture worker compilation failed' }
    Push-Location $scratch
    try { & zig rc /c 65001 /fo worker.res worker.rc; if ($LASTEXITCODE -ne 0) { throw 'diagnostic Worker resource build failed' } } finally { Pop-Location }
    $dll = Join-Path $scratch 'diagnostic.dll'
    $windowsHeaders = Get-BeeNTZigWindowsHeaders
    $stackProbe = Join-Path $scratch 'beent_stack_probe.o'
    & zig cc -target x86-windows-gnu -c other/beent_stack_probe.S -o $stackProbe
    if ($LASTEXITCODE -ne 0) { throw 'diagnostic stack probe compilation failed' }
    & zig cc -target x86-windows-gnu -O2 -fno-builtin -nostdlib -shared -isystem $windowsHeaders -include other/beent_crt.h -DEVENT_TIMEOUT_MS=2000 -DSTOP_TIMEOUT_MS=200 -DWRITE_TIMEOUT_MS=500 other/beent_bridge.c $stackProbe (Join-Path $scratch 'worker.res') other/BeeNTPlugin.def "-I$scratch" -lkernel32 '-Wl,--entry,DllMain@12' -o $dll
    if ($LASTEXITCODE -ne 0) { throw 'diagnostic DLL compilation failed' }
    Assert-BeeNTPEImports -Imports @(Get-BeeNTPEImports -DllPath $dll)
    $invalidWorker = Join-Path $scratch 'invalid-worker.exe'
    [IO.File]::WriteAllText($invalidWorker, 'This resource deliberately is not a Windows executable.', [Text.Encoding]::ASCII)
    $invalidResource = Join-Path $scratch 'invalid-worker.rc'
    [IO.File]::WriteAllText($invalidResource, '101 RCDATA "invalid-worker.exe"', [Text.Encoding]::ASCII)
    Push-Location $scratch
    try { & zig rc /c 65001 /fo invalid-worker.res invalid-worker.rc; if ($LASTEXITCODE -ne 0) { throw 'invalid Worker resource build failed' } } finally { Pop-Location }
    $invalidDll = Join-Path $scratch 'invalid-resource.dll'
    & zig cc -target x86-windows-gnu -O2 -fno-builtin -nostdlib -shared -isystem $windowsHeaders -include other/beent_crt.h -DEVENT_TIMEOUT_MS=2000 -DSTOP_TIMEOUT_MS=200 -DWRITE_TIMEOUT_MS=500 other/beent_bridge.c $stackProbe (Join-Path $scratch 'invalid-worker.res') other/BeeNTPlugin.def "-I$scratch" -lkernel32 '-Wl,--entry,DllMain@12' -o $invalidDll
    if ($LASTEXITCODE -ne 0) { throw 'invalid-resource DLL compilation failed' }
    $executable = Join-Path $scratch 'diagnostic_host.exe'
    & zig cc -target x86-windows-gnu -static -O2 -Wall -Wextra -municode other/tests/diagnostic_host.c -o $executable
    if ($LASTEXITCODE -ne 0) { throw 'diagnostic host compilation failed' }
    $cases = if ($Case -eq 'all') { @('stderr-exit', 'callback-error', 'invalid-resource') } else { @($Case) }
    $failed = @()
    foreach ($item in $cases) {
        Write-Host "Diagnostic case: $item"
        $target = if ($item -eq 'invalid-resource') { $invalidDll } else { $dll }
        & $executable $target $item
        if ($LASTEXITCODE -ne 0) { $failed += $item }
        $logDirectory = Join-Path $scratch 'plugin_data\BeeNT_Bridge_Test'
        foreach ($logName in @('bridge.log', 'worker-stderr.log')) {
            $logPath = Join-Path $logDirectory $logName
            if (Test-Path -LiteralPath $logPath) { Copy-Item -LiteralPath $logPath -Destination (Join-Path $scratch "$item-$logName") -Force }
        }
    }
    if ($failed.Count) { throw "diagnostic cases failed: $($failed -join ', ')" }
} finally {
    $env:GOOS = $savedGoos; $env:GOARCH = $savedGoarch; $env:CGO_ENABLED = $savedCgo
    $env:GOTOOLCHAIN = $savedToolchain; $env:GOROOT = $savedRoot; $env:GOWORK = $savedGowork
    Pop-Location
}
