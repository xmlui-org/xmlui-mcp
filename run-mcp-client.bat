@echo off
setlocal

REM Get the absolute path to the current script directory
set "CWD=%~dp0"
REM Remove trailing backslash if present
if "%CWD:~-1%"=="\" set "CWD=%CWD:~0,-1%"

REM Use first argument as examplesDir, default to %USERPROFILE%
if "%~1"=="" (
    echo Using ~ as default example root, you can pass an alternate path to this script
    set "EXAMPLES_DIR=%USERPROFILE%"
) else (
    set "EXAMPLES_DIR=%~1"
)

echo.

REM First prepare the binaries
call "%CWD%\prepare-binaries.bat"

REM Then run the client
"%CWD%\xmlui-mcp-client.exe" "%CWD%\xmlui-mcp %CWD% %EXAMPLES_DIR% xmlui-invoice,xmlui-mastodon"
