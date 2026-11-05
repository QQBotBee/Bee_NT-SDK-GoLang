$ErrorActionPreference = 'Stop'
$project = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
. (Join-Path $project 'other\build-tools.ps1')

function Expect-Rejected([string]$Name, [scriptblock]$Action) {
    $rejected = $false
    try { & $Action | Out-Null } catch { $rejected = $true }
    if (-not $rejected) { throw "FAIL: $Name was accepted" }
}

$valid = @('C:\test worker.exe: go1.20.14', "`tpath`tbeentpluginsdk", "`tbuild`tGOOS=windows", "`tbuild`tGOARCH=386", "`tbuild`tCGO_ENABLED=0")
Assert-BeeNTWorkerBuildInfo -BuildInfo $valid
Expect-Rejected 'newer Go compiler' { Assert-BeeNTWorkerBuildInfo -BuildInfo @($valid -replace 'go1.20.14', 'go1.27.3') }
Expect-Rejected 'other Go major/minor compiler' { Assert-BeeNTWorkerBuildInfo -BuildInfo @($valid -replace 'go1.20.14', 'go1.22.12') }
Expect-Rejected 'wrong patch compiler' { Assert-BeeNTWorkerBuildInfo -BuildInfo @($valid -replace 'go1.20.14', 'go1.20.13') }
Expect-Rejected 'wrong architecture' { Assert-BeeNTWorkerBuildInfo -BuildInfo @($valid -replace 'GOARCH=386', 'GOARCH=amd64') }
Expect-Rejected 'CGO enabled' { Assert-BeeNTWorkerBuildInfo -BuildInfo @($valid -replace 'CGO_ENABLED=0', 'CGO_ENABLED=1') }
Expect-Rejected 'wrong OS' { Assert-BeeNTWorkerBuildInfo -BuildInfo @($valid -replace 'GOOS=windows', 'GOOS=linux') }
Expect-Rejected 'removed worker version override' { Assert-BeeNTWorkerBuildInfo -BuildInfo $valid -RequiredVersion 'go1.20.14' }
Assert-BeeNTPEImports -Imports @('KERNEL32.dll')
Expect-Rejected 'legacy CRT import' { Assert-BeeNTPEImports -Imports @('KERNEL32.dll', 'msvcrt.dll') }
Expect-Rejected 'arbitrary external import' { Assert-BeeNTPEImports -Imports @('KERNEL32.dll', 'unexpected.dll') }
Expect-Rejected 'empty shell import table' { Assert-BeeNTPEImports -Imports @() }
Expect-Rejected 'UCRT API set import' { Assert-BeeNTPEImports -Imports @('api-ms-win-crt-runtime-l1-1-0.dll') }
Expect-Rejected 'UCRT direct import' { Assert-BeeNTPEImports -Imports @('ucrtbase.dll') }
Expect-Rejected 'VC runtime import' { Assert-BeeNTPEImports -Imports @('VCRUNTIME140.dll') }
Expect-Rejected 'removed compiler version override' { Initialize-BeeNTToolchain -RequiredVersion 'go1.20.14' }
if ((Get-Command (Join-Path $project 'other\build.ps1')).Parameters.ContainsKey('GoVersion')) { throw 'build.ps1 still permits toolchain version selection' }
Assert-BeeNTLegacyPEVersion -Header ([pscustomobject]@{ os_version = '6.1'; subsystem_version = '6.1' })
Expect-Rejected 'Windows 10 PE OS requirement' { Assert-BeeNTLegacyPEVersion -Header ([pscustomobject]@{ os_version = '10.0'; subsystem_version = '6.1' }) }
Expect-Rejected 'Windows 10 PE subsystem requirement' { Assert-BeeNTLegacyPEVersion -Header ([pscustomobject]@{ os_version = '6.1'; subsystem_version = '10.0' }) }

$savedToolchain = $env:GOTOOLCHAIN
$savedRoot = $env:GOROOT
try {
    $env:GOTOOLCHAIN = 'local'
    $pathGo = (Get-Command go -CommandType Application -ErrorAction Stop).Source
    $pathGoVersion = (& $pathGo version | Out-String).Trim()
    if ($pathGoVersion -notmatch '^go version go1\.20\.14 ') {
        Expect-Rejected 'explicit newer Go executable' { Initialize-BeeNTToolchain -GoExecutable $pathGo }
    }
} finally {
    $env:GOTOOLCHAIN = $savedToolchain
    $env:GOROOT = $savedRoot
}
Write-Host 'PASS: exact Go toolchain, Windows/386, CGO-disabled, kernel32-only shell and legacy PE version gates'
