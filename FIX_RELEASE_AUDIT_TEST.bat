@echo off
setlocal
cd /d "%~dp0"
where py >nul 2>nul
if errorlevel 1 (
  echo [FAIL] Python launcher "py" not found.
  pause
  exit /b 1
)
py -B "%~dp0fix_release_audit_test.py"
set "RC=%ERRORLEVEL%"
if not "%RC%"=="0" (
  echo.
  echo [FAIL] Fix was not applied successfully. Existing source was restored when possible.
  pause
  exit /b %RC%
)
echo.
echo [OK] Regression test updated and verified.
pause
exit /b 0
