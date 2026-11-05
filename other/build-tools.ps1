# Build-time helpers; this script does not change global Go settings or PATH.

function Get-BeeNTSha256([string]$Path) {
    $stream = [IO.File]::OpenRead((Resolve-Path -LiteralPath $Path).Path)
    $algorithm = [Security.Cryptography.SHA256]::Create()
    try { return [BitConverter]::ToString($algorithm.ComputeHash($stream)).Replace('-', '').ToLowerInvariant() }
    finally { $algorithm.Dispose(); $stream.Dispose() }
}

function Get-BeeNTZigWindowsHeaders {
    $zigEnvironment = (& zig env | Out-String)
    if ($LASTEXITCODE -ne 0 -or $zigEnvironment -notmatch '(?m)^\s*(?:\.lib_dir\s*=|"lib_dir"\s*:)\s*("(?:[^"\\]|\\.)*")') { throw 'cannot resolve Zig library directory' }
    $zigLibraryDirectory = $Matches[1] | ConvertFrom-Json
    $headers = Join-Path $zigLibraryDirectory 'libc\include\any-windows-any'
    if (-not (Test-Path -LiteralPath (Join-Path $headers 'windows.h') -PathType Leaf)) { throw "Zig Windows headers are missing: $headers" }
    return $headers
}

function Get-BeeNTGoToolchain([string]$ToolchainCache) {
    $version = 'go1.20.14'
    $sha256 = '0e0d0190406ead891d94ecf00f961bb5cfa15ddd47499d2649f12eee80aee110'
    $directory = Join-Path $ToolchainCache $version
    $compiler = Join-Path $directory 'go\bin\go.exe'
    if (Test-Path -LiteralPath $compiler -PathType Leaf) { return $compiler }
    New-Item -ItemType Directory -Force -Path $directory | Out-Null
    $archive = Join-Path $directory 'go1.20.14.windows-amd64.zip'
    if (-not (Test-Path -LiteralPath $archive -PathType Leaf)) {
        $download = $archive + '.download'
        Write-Host "Downloading official $version Windows/amd64 toolchain to $directory"
        $oldProtocol = [Net.ServicePointManager]::SecurityProtocol
        try {
            [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
            Invoke-WebRequest -UseBasicParsing -Uri 'https://dl.google.com/go/go1.20.14.windows-amd64.zip' -TimeoutSec 60 -OutFile $download
        } finally { [Net.ServicePointManager]::SecurityProtocol = $oldProtocol }
        if ((Get-BeeNTSha256 -Path $download) -ne $sha256) { throw 'official Go 1.20.14 archive SHA256 mismatch' }
        Move-Item -LiteralPath $download -Destination $archive -Force
    }
    # The digest is the official go.dev download metadata, not a checksum supplied by the archive.
    if ((Get-BeeNTSha256 -Path $archive) -ne $sha256) { throw 'cached Go 1.20.14 archive SHA256 mismatch' }
    Expand-Archive -LiteralPath $archive -DestinationPath $directory -Force
    if (-not (Test-Path -LiteralPath $compiler -PathType Leaf)) { throw 'Go 1.20.14 archive has no compiler executable' }
    return $compiler
}

function Initialize-BeeNTToolchain {
    [CmdletBinding()]
    param([string]$GoExecutable = '', [string]$ToolchainCache = '')
    $requiredVersion = 'go1.20.14'
    if ([string]::IsNullOrWhiteSpace($GoExecutable)) {
        if (-not $ToolchainCache) { $ToolchainCache = Join-Path (Split-Path $PSScriptRoot -Parent) 'temp\toolchains' }
        $GoExecutable = Get-BeeNTGoToolchain -ToolchainCache $ToolchainCache
    }
    $bootstrapGo = (Get-Command $GoExecutable -CommandType Application -ErrorAction Stop).Source
    $env:GOROOT = $null
    $env:GOTOOLCHAIN = 'local'
    $versionText = (& $bootstrapGo version | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $versionText -notmatch ('^go version ' + [regex]::Escape($requiredVersion) + ' windows/(amd64|386|arm64)$')) {
        throw "BeeNT requires exactly $requiredVersion for its worker. Selected executable reported: $versionText. Supply -GoExecutable with the matching go.exe or allow the official Go download."
    }
    $toolchainRoot = (& $bootstrapGo env GOROOT | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or -not $toolchainRoot) { throw 'cannot resolve the selected Go toolchain root' }
    $compiler = Join-Path $toolchainRoot 'bin\go.exe'
    if (-not (Test-Path -LiteralPath $compiler -PathType Leaf)) { throw "selected Go compiler does not exist: $compiler" }
    # Invoke the resolved binary directly and prohibit all automatic upgrades after selection.
    $env:GOTOOLCHAIN = 'local'
    $resolvedVersion = (& $compiler version | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $resolvedVersion -ne $versionText) { throw 'resolved Go compiler does not match the selected toolchain' }
    [pscustomobject]@{ Executable = $compiler; Version = $requiredVersion; VersionText = $resolvedVersion; Root = $toolchainRoot }
}

function Assert-BeeNTWorkerBuildInfo {
    [CmdletBinding()]
    param([string[]]$BuildInfo)
    if (-not $BuildInfo -or $BuildInfo[0] -notmatch ':\s+go1\.20\.14\s*$') {
        throw "embedded Go worker must report exactly go1.20.14; got: $($BuildInfo | Select-Object -First 1)"
    }
    $text = $BuildInfo -join "`n"
    foreach ($setting in @('GOOS=windows', 'GOARCH=386', 'CGO_ENABLED=0')) {
        if ($text -notmatch ('(?m)^\s*build\s+' + [regex]::Escape($setting) + '\s*$')) {
            throw "embedded Go worker build information is missing required setting: $setting"
        }
    }
}

function Assert-BeeNTPEImports([string[]]$Imports) {
    if ($Imports.Count -ne 1 -or $Imports[0] -ine 'KERNEL32.dll') {
        throw "BeeNT shell must import only KERNEL32.dll; actual imports: $($Imports -join ', ')"
    }
}

function Get-BeeNTPEHeader([string]$Path) {
    $data = [IO.File]::ReadAllBytes((Resolve-Path -LiteralPath $Path).Path)
    if ($data.Length -lt 256 -or [BitConverter]::ToUInt16($data, 0) -ne 0x5a4d) { throw 'file is not a DOS/PE image' }
    $pe = [BitConverter]::ToInt32($data, 0x3c)
    if ($pe -lt 0 -or [int64]$pe + 144 -ge $data.Length -or [BitConverter]::ToUInt32($data, $pe) -ne 0x4550) { throw 'file has an invalid PE header' }
    $optional = $pe + 24
    $machine = [BitConverter]::ToUInt16($data, $pe + 4)
    $magic = [BitConverter]::ToUInt16($data, $optional)
    if ($machine -ne 0x14c -or $magic -ne 0x10b) { throw 'BeeNT shell and worker must be actual x86 PE32 images' }
    [pscustomobject]@{
        machine = ('0x{0:x4}' -f $machine)
        optional_header = ('0x{0:x4}' -f $magic)
        os_version = ('{0}.{1}' -f [BitConverter]::ToUInt16($data, $optional + 40), [BitConverter]::ToUInt16($data, $optional + 42))
        subsystem_version = ('{0}.{1}' -f [BitConverter]::ToUInt16($data, $optional + 48), [BitConverter]::ToUInt16($data, $optional + 50))
    }
}

function Assert-BeeNTLegacyPEVersion([object]$Header) {
    if ([version]$Header.os_version -gt [version]'6.3' -or [version]$Header.subsystem_version -gt [version]'6.3') {
        throw "Server 2012 R2 artifact requires PE OS/subsystem versions no higher than 6.3; got OS=$($Header.os_version), subsystem=$($Header.subsystem_version)"
    }
}

function Get-BeeNTPEImports([string]$DllPath) {
    $data = [IO.File]::ReadAllBytes((Resolve-Path -LiteralPath $DllPath).Path)
    if ($data.Length -lt 256 -or [BitConverter]::ToUInt16($data, 0) -ne 0x5a4d) { throw 'shell is not a DOS/PE image' }
    $pe = [BitConverter]::ToInt32($data, 0x3c)
    if ($pe -lt 0 -or $pe + 144 -ge $data.Length -or [BitConverter]::ToUInt32($data, $pe) -ne 0x4550) { throw 'shell has an invalid PE header' }
    if ([BitConverter]::ToUInt16($data, $pe + 4) -ne 0x14c) { throw 'BeeNT shell must have x86 PE machine 0x014c' }
    $optional = $pe + 24
    if ([BitConverter]::ToUInt16($data, $optional) -ne 0x10b) { throw 'BeeNT shell must be PE32' }
    $sections = [BitConverter]::ToUInt16($data, $pe + 6)
    $sectionTable = $optional + [BitConverter]::ToUInt16($data, $pe + 20)
    $importRva = [BitConverter]::ToUInt32($data, $optional + 104)
    if ($importRva -eq 0) { return @() }
    $rvaOffset = {
        param([uint32]$Rva)
        for ($index = 0; $index -lt $sections; $index++) {
            $section = $sectionTable + 40 * $index
            if ($section + 40 -gt $data.Length) { throw 'invalid PE section table' }
            $virtualSize = [BitConverter]::ToUInt32($data, $section + 8)
            $virtualAddress = [BitConverter]::ToUInt32($data, $section + 12)
            $rawSize = [BitConverter]::ToUInt32($data, $section + 16)
            $rawOffset = [BitConverter]::ToUInt32($data, $section + 20)
            if ($Rva -ge $virtualAddress -and [uint64]$Rva -lt [uint64]$virtualAddress + [Math]::Max($virtualSize, $rawSize)) {
                $offset = [uint64]$rawOffset + $Rva - $virtualAddress
                if ($offset -ge $data.Length) { throw 'PE import RVA is outside the image' }
                return [int]$offset
            }
        }
        throw 'PE import RVA has no file section'
    }
    $descriptorStart = & $rvaOffset $importRva
    $names = @()
    for ($index = 0; $index -lt 1024; $index++) {
        $descriptor = $descriptorStart + $index * 20
        if ($descriptor + 20 -gt $data.Length) { throw 'PE import descriptor is truncated' }
        $nameRva = [BitConverter]::ToUInt32($data, $descriptor + 12)
        if ($nameRva -eq 0) { return $names }
        $start = & $rvaOffset $nameRva
        $end = $start
        while ($end -lt $data.Length -and $data[$end] -ne 0 -and $end - $start -lt 512) { $end++ }
        if ($end -eq $data.Length -or $end - $start -ge 512) { throw 'PE import DLL name is unterminated' }
        $names += [Text.Encoding]::ASCII.GetString($data, $start, $end - $start)
    }
    throw 'PE image contains too many import descriptors'
}

function Get-BeeNTWorkerResource([string]$DllPath) {
    if (-not ('BeeNT.Build.ResourceNative' -as [type])) {
        Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
namespace BeeNT.Build {
    public static class ResourceNative {
        [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
        public static extern IntPtr LoadLibraryExW(string path, IntPtr file, uint flags);
        [DllImport("kernel32.dll", SetLastError = true)]
        public static extern IntPtr FindResourceW(IntPtr module, IntPtr name, IntPtr type);
        [DllImport("kernel32.dll", SetLastError = true)]
        public static extern uint SizeofResource(IntPtr module, IntPtr resource);
        [DllImport("kernel32.dll", SetLastError = true)]
        public static extern IntPtr LoadResource(IntPtr module, IntPtr resource);
        [DllImport("kernel32.dll", SetLastError = true)]
        public static extern IntPtr LockResource(IntPtr resource);
        [DllImport("kernel32.dll", SetLastError = true)]
        public static extern bool FreeLibrary(IntPtr module);
    }
}
'@
    }
    $path = (Resolve-Path -LiteralPath $DllPath -ErrorAction Stop).Path
    # Resource-only mapping neither runs DllMain nor starts the embedded executable.
    $module = [BeeNT.Build.ResourceNative]::LoadLibraryExW($path, [IntPtr]::Zero, 0x22)
    if ($module -eq [IntPtr]::Zero) { throw "cannot map DLL resources (Win32=$([Runtime.InteropServices.Marshal]::GetLastWin32Error())): $path" }
    try {
        $resource = [BeeNT.Build.ResourceNative]::FindResourceW($module, [IntPtr]101, [IntPtr]10)
        if ($resource -eq [IntPtr]::Zero) { throw 'DLL has no Worker RCDATA resource 101' }
        $size = [BeeNT.Build.ResourceNative]::SizeofResource($module, $resource)
        if ($size -eq 0 -or $size -gt [int]::MaxValue) { throw 'DLL Worker resource has invalid size' }
        $loaded = [BeeNT.Build.ResourceNative]::LoadResource($module, $resource)
        if ($loaded -eq [IntPtr]::Zero) { throw 'cannot load DLL Worker resource' }
        $pointer = [BeeNT.Build.ResourceNative]::LockResource($loaded)
        if ($pointer -eq [IntPtr]::Zero) { throw 'cannot read DLL Worker resource' }
        $bytes = New-Object byte[] ([int]$size)
        [Runtime.InteropServices.Marshal]::Copy($pointer, $bytes, 0, [int]$size)
        return ,$bytes
    } finally { [BeeNT.Build.ResourceNative]::FreeLibrary($module) | Out-Null }
}

function Assert-BeeNTBuild([string]$DllPath, [string]$BuiltWorkerPath, [string]$ExtractedWorkerPath, [object]$Toolchain) {
    $shellHeader = Get-BeeNTPEHeader -Path $DllPath
    $imports = @(Get-BeeNTPEImports -DllPath $DllPath)
    Assert-BeeNTPEImports -Imports $imports
    [IO.File]::WriteAllBytes($ExtractedWorkerPath, (Get-BeeNTWorkerResource -DllPath $DllPath))
    $compiledHash = Get-BeeNTSha256 -Path $BuiltWorkerPath
    $embeddedHash = Get-BeeNTSha256 -Path $ExtractedWorkerPath
    if ($compiledHash -ne $embeddedHash) { throw 'DLL embeds a different Worker from the executable just compiled' }
    $workerHeader = Get-BeeNTPEHeader -Path $ExtractedWorkerPath
    Assert-BeeNTLegacyPEVersion -Header $shellHeader
    Assert-BeeNTLegacyPEVersion -Header $workerHeader
    $buildInfo = @(& $Toolchain.Executable version -m $ExtractedWorkerPath)
    if ($LASTEXITCODE -ne 0) { throw 'cannot read embedded Worker Go build information' }
    Assert-BeeNTWorkerBuildInfo -BuildInfo $buildInfo
    if ($Toolchain.Version -ne 'go1.20.14') { throw 'build requires the fixed Go 1.20.14 toolchain' }
}
