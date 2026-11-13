@echo off
rem Ferma il servizio Gaster Bless (per esempio prima di un ripristino da backup).
sc.exe stop Gaster Bless
echo Gaster Bless e' stato fermato. Per riavviarlo usa "Avvia Gaster Bless".
pause
