param([string]$GoExecutable = '')
$ErrorActionPreference = 'Stop'
$project = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$scratch = Join-Path $project ('temp\acceptance-go-tests\workspace-' + [Guid]::NewGuid().ToString('N'))
$metadataPackage = Join-Path $scratch 'other\buildmeta'
New-Item -ItemType Directory -Force -Path $metadataPackage | Out-Null
# Go 1.20 vet cannot open a virtual new file supplied through -overlay. Use a
# physical isolated package so both test and vet inspect the acceptance source.
Get-ChildItem -LiteralPath $project -Filter '*.go' -File | Copy-Item -Destination $scratch
Copy-Item -LiteralPath (Join-Path $project 'go.mod'), (Join-Path $project 'go.sum') -Destination $scratch
Get-ChildItem -LiteralPath (Join-Path $project 'other\buildmeta') -Filter '*.go' -File | Copy-Item -Destination $metadataPackage
Copy-Item -LiteralPath (Join-Path $project 'other\tests\fixtures\acceptance\plugin_main.go.txt') -Destination (Join-Path $scratch 'plugin_main.go') -Force
Copy-Item -LiteralPath (Join-Path $project 'other\tests\fixtures\acceptance\plugin_main_test.go.txt') -Destination (Join-Path $scratch 'acceptance_plugin_test.go')
. (Join-Path $project 'other\build-tools.ps1')
$savedGoos = $env:GOOS
$savedGoarch = $env:GOARCH
$savedCgo = $env:CGO_ENABLED
$savedToolchain = $env:GOTOOLCHAIN
$savedRoot = $env:GOROOT
$savedGowork = $env:GOWORK
Push-Location $scratch
try {
    $env:GOOS = 'windows'
    $env:CGO_ENABLED = '0'
    $env:GOWORK = 'off'
    $toolchain = Initialize-BeeNTToolchain -GoExecutable $GoExecutable -ToolchainCache (Join-Path $project 'temp\toolchains')
    $compiler = $toolchain.Executable
    & $compiler version
    if ($LASTEXITCODE -ne 0) { throw 'cannot run selected Go compiler' }
    foreach ($architecture in @('amd64', '386')) {
        $env:GOARCH = $architecture
        Write-Host "Acceptance Go tests: windows/$architecture"
        & $compiler test -count=1 ./...
        if ($LASTEXITCODE -ne 0) { throw "acceptance Go tests failed: $architecture" }
        & $compiler vet ./...
        if ($LASTEXITCODE -ne 0) { throw "acceptance Go vet failed: $architecture" }
    }
} finally {
    $env:GOOS = $savedGoos
    $env:GOARCH = $savedGoarch
    $env:CGO_ENABLED = $savedCgo
    $env:GOTOOLCHAIN = $savedToolchain
    $env:GOROOT = $savedRoot
    $env:GOWORK = $savedGowork
    Pop-Location
}
