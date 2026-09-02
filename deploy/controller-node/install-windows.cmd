@echo off
setlocal EnableExtensions
cd /d "%~dp0"

rem Reopen once with admin rights: setup registers a Windows startup service.
net session >nul 2>&1
if not "%errorlevel%"=="0" (
  powershell -NoProfile -ExecutionPolicy Bypass -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
  exit /b
)

rem The PostgreSQL runtime embedded by the Controller requires the supported
rem Microsoft Visual C++ x64 runtime. Fresh Windows Server/VM images often do
rem not include it; without this preflight initdb fails with 0xC0000135.
reg query "HKLM\SOFTWARE\Microsoft\VisualStudio\14.0\VC\Runtimes\x64" /v Installed 2>nul | find "0x1" >nul
if not errorlevel 1 goto vc_runtime_ready

echo.
echo Installing the required Microsoft Visual C++ runtime...
set "VC_REDIST=%TEMP%\ClubPay-vc_redist.x64.exe"
powershell -NoProfile -ExecutionPolicy Bypass -Command "Invoke-WebRequest -UseBasicParsing -Uri 'https://aka.ms/vs/17/release/vc_redist.x64.exe' -OutFile '%VC_REDIST%'"
if errorlevel 1 (
  echo Could not download the Microsoft Visual C++ runtime.
  echo Check the Internet connection and start this installer again.
  pause
  exit /b 1
)
"%VC_REDIST%" /install /quiet /norestart
set "VC_REDIST_EXIT=%errorlevel%"
if "%VC_REDIST_EXIT%"=="0" goto vc_runtime_ready
if "%VC_REDIST_EXIT%"=="3010" goto vc_runtime_ready
echo Microsoft Visual C++ runtime installation failed with code %VC_REDIST_EXIT%.
pause
exit /b 1

:vc_runtime_ready
if exist "controller.env" (
  schtasks.exe /Query /TN "ClubPay Controller Node" >nul 2>&1
  if not errorlevel 1 (
    echo This Controller is already configured.
    echo Open http://localhost:8080/api/node/status to check it.
    pause
    exit /b 0
  )

  rem A previous setup may have reached the config step but failed before it
  rem registered the startup task (for example while preparing PostgreSQL).
  echo Existing Controller configuration found, but the startup task is missing.
  echo Repairing the Windows startup task...
  ClubPay.Controller.exe --install --config "controller.env"
  if errorlevel 1 (
    echo.
    echo Repair did not finish. Start ClubPay.Controller.exe manually to see the error.
    pause
    exit /b 1
  )
  schtasks.exe /Run /TN "ClubPay Controller Node"
  if errorlevel 1 (
    echo.
    echo The startup task was created but could not be started.
    pause
    exit /b 1
  )
  echo.
  echo Ready. Open http://localhost:8080/api/node/status to check it.
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
