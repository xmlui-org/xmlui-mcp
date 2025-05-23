@echo off
setlocal

REM Get the absolute path to the current directory
set "CWD=%~dp0"
REM Remove trailing backslash if present
if "%CWD:~-1%"=="\" set "CWD=%CWD:~0,-1%"

"%CWD%\xmlui-mcp-client.exe" "%CWD%\xmlui-mcp.exe %CWD% %CWD% xmlui-hn,xmlui-mastodon"
