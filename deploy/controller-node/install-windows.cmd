@echo off
setlocal EnableExtensions
cd /d "%~dp0"

rem Reopen once with admin rights: setup registers a Windows startup service.
net session >nul 2>&1
if not "%errorlevel%"=="0" (
  powershell -NoProfile -ExecutionPolicy Bypass -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
  exit /b
)

if exist "controller.env" (
  echo This Controller is already configured.
  echo Open http://localhost:8080/api/node/status to check it.
  pause
  exit /b 0
)

echo.
echo ClubPay Controller Node setup
echo Generate a one-time activation code in ClubPay Web Admin first.
set /p ACTIVATION_CODE=Paste activation code:
if "%ACTIVATION_CODE%"=="" (
  echo Activation code is required.
  pause
  exit /b 1
)

ClubPay.Controller.exe --setup --activation-code "%ACTIVATION_CODE%"
if errorlevel 1 (
  echo.
  echo Setup did not finish. The activation code is valid for 30 minutes and can be used only once.
  pause
  exit /b 1
)

echo.
echo Ready. This Controller now starts automatically with Windows.
pause
