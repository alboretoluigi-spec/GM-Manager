@echo off
rem Avvia il servizio Gaster Bless (funziona anche senza diritti di amministratore) e apre l'app.
sc.exe start Gaster Bless >nul 2>&1
timeout /t 2 /nobreak >nul
start "" "https://localhost:7600/"
