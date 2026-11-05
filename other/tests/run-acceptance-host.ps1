param([string]$DllPath = '', [string]$ExpectedPluginName = '')
$ErrorActionPreference = 'Stop'
$project = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
if (-not $DllPath) { $DllPath = Join-Path $project 'temp\acceptance-host-tests\acceptance-fixture.dll' }
$dll = (Resolve-Path -LiteralPath $DllPath -ErrorAction Stop).Path
$output = Join-Path $project 'temp\acceptance-host-tests'
New-Item -ItemType Directory -Force -Path $output | Out-Null
Push-Location $project
try {
    $executable = Join-Path $output 'acceptance_host.exe'
    & zig cc -target x86-windows-gnu -O2 -Wall -Wextra -municode other/tests/acceptance_host.c -o $executable
    if ($LASTEXITCODE -ne 0) { throw 'acceptance host compilation failed' }
    $startInfo = New-Object System.Diagnostics.ProcessStartInfo
    $startInfo.FileName = $executable
    $startInfo.Arguments = '"' + $dll + '"'
    if ($ExpectedPluginName) { $startInfo.Arguments += ' "' + $ExpectedPluginName + '"' }
    $startInfo.UseShellExecute = $false
    $startInfo.CreateNoWindow = $true
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    $process = [Diagnostics.Process]::Start($startInfo)
    $standardOutput = $process.StandardOutput.ReadToEndAsync()
    $standardError = $process.StandardError.ReadToEndAsync()
    if (-not $process.WaitForExit(45000)) {
        $process.Kill()
        throw 'acceptance host exceeded 45 seconds; lifecycle or API callback may be blocked'
    }
    if ($standardOutput.Result) { Write-Host $standardOutput.Result.TrimEnd() }
    if ($standardError.Result) { Write-Host $standardError.Result.TrimEnd() }
    if ($process.ExitCode -ne 0) { throw "acceptance host failed (exit=$($process.ExitCode))" }
} finally { Pop-Location }
