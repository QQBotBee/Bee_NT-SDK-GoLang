param([string]$DllName = '', [string]$PluginSource = 'plugin_main.go', [string]$GoExecutable = '')
$ErrorActionPreference = 'Stop'
$project = Split-Path $PSScriptRoot -Parent
$temp = Join-Path $project 'temp\build'
$output = Join-Path $project 'build'
$savedGoos = $env:GOOS
$savedGoarch = $env:GOARCH
$savedCgo = $env:CGO_ENABLED
$savedToolchain = $env:GOTOOLCHAIN
$savedGoroot = $env:GOROOT
$savedGowork = $env:GOWORK
. (Join-Path $project 'other\build-tools.ps1')
Push-Location $project
try {
    Get-Command zig -ErrorAction Stop | Out-Null
    $env:GOWORK = 'off'
    $toolchain = Initialize-BeeNTToolchain -GoExecutable $GoExecutable -ToolchainCache (Join-Path $project 'temp\toolchains')
    Write-Host "Toolchain: $($toolchain.VersionText)"
    $hostTarget = @(& $toolchain.Executable env GOHOSTOS GOHOSTARCH)
    if ($LASTEXITCODE -ne 0 -or $hostTarget.Count -ne 2) { throw 'cannot determine Go host target' }
    $env:GOOS = $hostTarget[0]; $env:GOARCH = $hostTarget[1]; $env:CGO_ENABLED = '0'
    $pluginSourcePath = (Resolve-Path -LiteralPath $PluginSource -ErrorAction Stop).Path
    if (-not (Test-Path -LiteralPath $pluginSourcePath -PathType Leaf)) { throw 'PluginSource must be a source file' }
    New-Item -ItemType Directory -Force -Path $temp, $output | Out-Null
    Write-Host '[1/4] Generate BeeNT plugin metadata'
    & $toolchain.Executable run ./other/buildmeta $pluginSourcePath $temp
    if ($LASTEXITCODE -ne 0) { throw 'metadata generation failed' }
    if (-not $DllName) { $DllName = [IO.File]::ReadAllText((Join-Path $temp 'plugin_name.txt'), [Text.Encoding]::UTF8) }
    if ($DllName.EndsWith('.dll', [StringComparison]::OrdinalIgnoreCase)) { $DllName = $DllName.Substring(0, $DllName.Length - 4) }
    if (-not $DllName -or [IO.Path]::GetFileName($DllName) -ne $DllName -or $DllName.IndexOfAny([IO.Path]::GetInvalidFileNameChars()) -ge 0) { throw 'DLL name must be a filename without a directory' }
    $target = Join-Path $output ($DllName + '.dll')
    Write-Host '[2/4] Build Windows x86 Go worker'
    $env:GOOS = 'windows'; $env:GOARCH = '386'; $env:CGO_ENABLED = '0'
    $workerSource = Join-Path $temp ('worker-' + [Guid]::NewGuid().ToString('N'))
    Copy-BeeNTWorkerSources -ProjectPath $project -Destination $workerSource -PluginSource $pluginSourcePath
    Push-Location $workerSource
    try {
        & $toolchain.Executable build -buildvcs=false -trimpath '-ldflags=-s -w' -o (Join-Path $temp 'beent_go_worker.exe') .
        if ($LASTEXITCODE -ne 0) { throw 'Go worker build failed' }
    } finally { Pop-Location }
    Write-Host '[3/4] Embed worker resource'
    Push-Location $temp
    try { & zig rc /c 65001 /fo worker.res worker.rc; if ($LASTEXITCODE -ne 0) { throw 'resource build failed' } } finally { Pop-Location }
    Write-Host '[4/4] Build BeeNT PE32 shell DLL'
    $temporaryDll = Join-Path $temp 'plugin.dll'
    $stackProbe = Join-Path $temp 'beent_stack_probe.o'
    & zig cc -target x86-windows-gnu -c other/beent_stack_probe.S -o $stackProbe
    if ($LASTEXITCODE -ne 0) { throw 'BeeNT stack probe assembly failed' }
    $windowsHeaders = Get-BeeNTZigWindowsHeaders
    & zig cc -target x86-windows-gnu -O2 -fno-builtin -nostdlib -shared -isystem $windowsHeaders -include other/beent_crt.h other/beent_bridge.c $stackProbe (Join-Path $temp 'worker.res') other/BeeNTPlugin.def "-I$temp" -lkernel32 '-Wl,--entry,DllMain@12' -o $temporaryDll
    if ($LASTEXITCODE -ne 0) { throw 'BeeNT DLL link failed' }
    Assert-BeeNTBuild -DllPath $temporaryDll -BuiltWorkerPath (Join-Path $temp 'beent_go_worker.exe') -ExtractedWorkerPath (Join-Path $temp 'embedded_worker_verified.exe') -Toolchain $toolchain
    Move-Item -LiteralPath $temporaryDll -Destination $target -Force
    $verifiedTemp = [IO.Path]::GetFullPath($temp)
    if ($verifiedTemp -ne [IO.Path]::GetFullPath((Join-Path $project 'temp\build'))) { throw 'unexpected build scratch directory' }
    Remove-Item -LiteralPath $verifiedTemp -Recurse -Force
    Write-Host "Built: $target"
    Write-Host 'Verified embedded worker: go1.20.14, windows/386, CGO=0'
} finally {
    $env:GOOS = $savedGoos; $env:GOARCH = $savedGoarch; $env:CGO_ENABLED = $savedCgo
    $env:GOTOOLCHAIN = $savedToolchain; $env:GOROOT = $savedGoroot; $env:GOWORK = $savedGowork
    Pop-Location
}
