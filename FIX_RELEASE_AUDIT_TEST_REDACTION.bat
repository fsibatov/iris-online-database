@echo off
setlocal
cd /d "%~dp0"
py -B "%~dp0fix_release_audit_test_redaction.py"
set "RC=%ERRORLEVEL%"
echo.
if not "%RC%"=="0" (
  echo FIX FAILED. Read the message above.
) else (
  echo FIX COMPLETE.
)
pause
exit /b %RC%
