@echo off
setlocal EnableExtensions DisableDelayedExpansion
cd /d "%~dp0"
chcp 936 >nul

echo ========================================
echo BeeNT Go Plugin Builder
echo ========================================
echo.

set "REQUIRED_GO_VERSION=go1.20.14"
set "BEE_GO_EXE="
set "BEE_GO_VERSION="
set "GOTOOLCHAIN=local"
set "GOWORK=off"
set "GOROOT="
for /f "delims=" %%G in ('where go.exe 2^>nul') do if not defined BEE_GO_EXE set "BEE_GO_EXE=%%G"
if not defined BEE_GO_EXE goto :missing_go
for /f "tokens=3" %%V in ('call "%BEE_GO_EXE%" version 2^>nul') do set "BEE_GO_VERSION=%%V"
if not "%BEE_GO_VERSION%"=="%REQUIRED_GO_VERSION%" goto :wrong_go
echo Compiler: %BEE_GO_EXE% (%BEE_GO_VERSION%)
where zig >nul 2>nul || goto :missing_zig

if exist temp rmdir /s /q temp
mkdir temp || goto :fail
if not exist build mkdir build || goto :fail

echo [1/4] generating plugin metadata...
"%BEE_GO_EXE%" run .\other\buildmeta plugin_main.go temp > temp\plugin_name.txt
if errorlevel 1 goto :fail
set /p "PLUGIN_NAME=" < temp\plugin_name.txt
if not defined PLUGIN_NAME goto :metadata_empty
del /q temp\plugin_name.txt >nul 2>nul

set "DLL_NAME=%~1"
if not defined DLL_NAME set "DLL_NAME=%PLUGIN_NAME%"
if /i "%DLL_NAME:~-4%"==".dll" set "DLL_NAME=%DLL_NAME:~0,-4%"
if not defined DLL_NAME goto :invalid_name

echo [2/4] building Windows 32-bit Go worker...
set GOOS=windows
set GOARCH=386
set CGO_ENABLED=0
"%BEE_GO_EXE%" build -buildvcs=false -trimpath -ldflags="-s -w" -o temp\beent_go_worker.exe .
set "BUILD_RESULT=%ERRORLEVEL%"
del /q worker_runtime.go >nul 2>nul
if not "%BUILD_RESULT%"=="0" goto :fail
set "BEE_WORKER_GO_VERSION="
for /f "tokens=2" %%V in ('call "%BEE_GO_EXE%" version temp\beent_go_worker.exe 2^>nul') do set "BEE_WORKER_GO_VERSION=%%V"
if not "%BEE_WORKER_GO_VERSION%"=="%REQUIRED_GO_VERSION%" goto :wrong_worker_go

echo [3/4] compiling embedded worker resource...
pushd temp
zig rc /c 65001 /fo worker.res worker.rc
if errorlevel 1 (popd & goto :fail)
popd

echo [4/4] linking pure C PE32 plugin DLL...
zig cc -target x86-windows-gnu -O2 -shared other\beent_bridge.c temp\worker.res other\BeeNTPlugin.def -Itemp -lkernel32 -o "build\%DLL_NAME%.dll" || goto :fail

del /q build\*.lib build\*.pdb >nul 2>nul
rmdir /s /q temp
echo.
echo build succeeded: build\%DLL_NAME%.dll
pause
exit /b 0

:missing_go
echo error: Go 1.20.14 was not found on PATH.
goto :download_go

:wrong_go
echo error: this project requires exactly Go 1.20.14, current compiler is %BEE_GO_VERSION%
echo compiler path: %BEE_GO_EXE%
goto :download_go

:download_go
echo download Go 1.20.14 from https://go.dev/dl/#go1.20.14
echo put its bin directory ahead of other Go versions in PATH, then open a new CMD.
echo check with: where go.exe   and   go version
goto :error_exit

:wrong_worker_go
echo error: the worker was not built with Go 1.20.14, build aborted.
goto :fail

:missing_zig
echo error: zig was not found on PATH.
echo download it from https://ziglang.org/download/
goto :error_exit

:metadata_empty
echo error: metadata generation returned an empty plugin name.
goto :fail

:invalid_name
echo error: DLL file name is invalid.
goto :fail

:fail
if exist worker_runtime.go del /q worker_runtime.go >nul 2>nul
if exist temp rmdir /s /q temp
echo error: build failed, see the messages above.

:error_exit
echo.
pause
exit /b 1
