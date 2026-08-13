@echo off
chcp 65001 >nul
powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0IrisTools.ps1" -Action Test
set "code=%errorlevel%"
echo.
if not "%code%"=="0" echo Script finished with an error. Read the message above.
pause
exit /b %code%
