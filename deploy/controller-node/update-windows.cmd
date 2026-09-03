@echo off
setlocal EnableExtensions
cd /d "%~dp0"

rem The updater preserves controller.env and the embedded PostgreSQL data.
rem It only replaces the executable, local PWA and SQL migrations in the
rem Controller installation already registered with Windows Startup.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0update-windows.ps1"
if errorlevel 1 (
  echo.
  echo Controller update did not finish. Read the message above.
  pause
)
