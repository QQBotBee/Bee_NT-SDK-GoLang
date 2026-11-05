@echo off
setlocal EnableExtensions DisableDelayedExpansion
cd /d "%~dp0"
set "BEENT_NO_PAUSE="
if /i "%~1"=="--no-pause" set "BEENT_NO_PAUSE=1"
if defined BEENT_NO_PAUSE (
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0other\build.ps1" %2
) else (
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0other\build.ps1" %1
)
set "BEENT_BUILD_RESULT=%ERRORLEVEL%"
if not defined BEENT_NO_PAUSE pause
exit /b %BEENT_BUILD_RESULT%
