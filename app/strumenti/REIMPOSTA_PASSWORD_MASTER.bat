@echo off
chcp 65001 >nul
rem Nuova password per il Master di Gaster Bless (chiede i permessi di amministratore)
net session >nul 2>&1
if %errorlevel% neq 0 (
  powershell -NoProfile -Command "Start-Process -FilePath '%~f0' -Verb RunAs"
  exit /b
)
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0reimposta-password-master.ps1"
