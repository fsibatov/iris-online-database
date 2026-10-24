@echo off
setlocal EnableExtensions DisableDelayedExpansion
call "%~dp0scripts\windows\00_RELEASE_WINDOWS.bat"
exit /b %errorlevel%
