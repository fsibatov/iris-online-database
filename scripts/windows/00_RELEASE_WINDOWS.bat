@echo off
setlocal EnableExtensions DisableDelayedExpansion

set "HERE=%~dp0"
set "REPO="
if exist "%HERE%iris-online-database\IrisTools.ps1" set "REPO=%HERE%iris-online-database"
if not defined REPO if exist "%HERE%..\..\IrisTools.ps1" for %%I in ("%HERE%..\..") do set "REPO=%%~fI"
set "PS=%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe"

if not defined REPO (
  echo ERROR: iris-online-database repository was not found next to this launcher.
  pause
  exit /b 1
)
if not exist "%PS%" (
  echo ERROR: Windows PowerShell 5.1 executable was not found.
  pause
  exit /b 1
)

pushd "%REPO%"
if errorlevel 1 (
  echo ERROR: Could not enter repository directory.
  pause
  exit /b 1
)
set "LAST_ACTION_RC=0"

:menu
cls
echo Iris Online Database Windows Release
echo.
echo 1 - PREPARE RELEASE
echo 2 - PUSH COMMIT
echo 3 - GITHUB RELEASE
echo 4 - CHECK TOOLS
echo 5 - INSTALL/UPDATE TOOLS
echo 6 - OPEN RELEASE FOLDER
echo 0 - EXIT
echo.
set "CHOICE="
set /p "CHOICE=Select: "

if "%CHOICE%"=="1" goto action_prepare
if "%CHOICE%"=="2" goto action_publish
if "%CHOICE%"=="3" goto action_release
if "%CHOICE%"=="4" goto action_check
if "%CHOICE%"=="5" goto action_install
if "%CHOICE%"=="6" goto open_release
if "%CHOICE%"=="0" goto success_exit

echo Invalid selection.
pause
goto menu

:action_prepare
set "ACTION=Prepare"
goto run_action

:action_publish
set "ACTION=Publish"
goto run_action

:action_release
set "ACTION=Release"
goto run_action

:action_check
set "ACTION=Check"
goto run_action

:action_install
set "ACTION=Install"
goto run_action

:run_action
"%PS%" -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%REPO%\IrisTools.ps1" -Action "%ACTION%"
set "RC=%ERRORLEVEL%"
set "LAST_ACTION_RC=%RC%"
if not "%RC%"=="0" goto failed

echo.
echo Action completed successfully.
pause
goto menu

:open_release
set "VERSION="
set /p "VERSION="<"%REPO%\VERSION"
for %%I in ("%REPO%\..") do set "REPARENT=%%~fI"
set "RELEASEDIR=%REPARENT%\iris-online-database-release-%VERSION%"
if not exist "%RELEASEDIR%\" (
  echo Release folder does not exist yet:
  echo %RELEASEDIR%
  pause
  goto menu
)
start "" "%SystemRoot%\explorer.exe" "%RELEASEDIR%"
if errorlevel 1 goto failed
pause
goto menu

:failed
if not defined RC set "RC=%ERRORLEVEL%"
if "%RC%"=="0" set "RC=1"
set "LAST_ACTION_RC=%RC%"
echo.
echo FAILED. Exit code: %RC%
echo The window will remain open so the failure can be reviewed.
echo No later release action has been started automatically.
echo.
pause
goto menu

:success_exit
popd
exit /b %LAST_ACTION_RC%
