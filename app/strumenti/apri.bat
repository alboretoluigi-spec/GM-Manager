@echo off
rem Apre Gaster Bless nel browser; se il servizio e' fermo lo avvia.
sc.exe query GasterBless | find "RUNNING" >nul || (sc.exe start GasterBless >nul 2>&1 & timeout /t 2 /nobreak >nul)
start "" "https://localhost:7700/"
