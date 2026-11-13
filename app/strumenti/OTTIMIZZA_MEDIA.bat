@echo off
chcp 65001 >nul
rem Copie leggere di immagini e video della cartella Media (chiede i permessi di amministratore)
net session >nul 2>&1
if %errorlevel% neq 0 (
  powershell -NoProfile -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
  exit /b
)
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0ottimizza-media.ps1"
