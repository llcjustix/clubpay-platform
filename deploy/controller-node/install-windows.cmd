@echo off
setlocal
cd /d "%~dp0"

if not exist "controller.env" (
  copy /Y "controller.env.example" "controller.env" >nul
  echo.
  echo Edit controller.env first: replace every CHANGE_ME value and set this node's LAN IP.
  notepad "controller.env"
  pause
)

docker compose --env-file controller.env -f postgres-compose.yml up -d
if errorlevel 1 (
  echo PostgreSQL did not start. Install Docker Desktop and start it, then run this file again.
  exit /b 1
)

ClubPay.Controller.exe --config "%~dp0controller.env" --install
if errorlevel 1 exit /b 1

schtasks /Run /TN "ClubPay Controller Node"
echo.
echo Controller installed. Open http://localhost:8080/api/node/status to verify it.
pause
