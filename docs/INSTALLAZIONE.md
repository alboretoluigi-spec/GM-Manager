# Installare GASTER BLESS – MASTER SUITE sul PC del Master (Windows)

Due modi. Sul PC del Master, che ha già Git, Go (in `C:\Progetti\Strumenti\go`) e l'accesso a GitHub per Barovia, si
usa il **modo A**: è quello con il tasto «Aggiorna». Gaster Bless usa la porta **7700** e cartelle sue: non tocca
Barovia, e le due app possono essere accese insieme.

## A. Dalla copia di GitHub (con il tasto «Aggiorna»)
1. Apri **PowerShell normale** (non amministratore) e scarica il progetto:
```
git clone https://github.com/alboretoluigi-spec/GM-Manager.git C:\Progetti\GM-Manager
```
2. Apri **Terminale (amministratore)** (tasto destro su Start) e lancia l'installazione: scarica le novità, compila
   il programma e lo installa.
```
powershell -ExecutionPolicy Bypass -File C:\Progetti\GM-Manager\app\strumenti\aggiorna.ps1
```
3. Apri l'icona **GasterBless** sul Desktop, oppure `https://localhost:7700`, e crea l'account del Master.
4. Da allora: Impostazioni → «Aggiornamento di Gaster Bless» → «🔍 Controlla novità» e «⟳ Aggiorna».

## B. Dal pacchetto completo (per un altro PC)
1. Sul PC del Master, in **Terminale (amministratore)**, crea il pacchetto (zip in Documenti\GasterBless\Pacchetti):
```
powershell -ExecutionPolicy Bypass -File C:\Progetti\GM-Manager\app\strumenti\crea-pacchetto.ps1
```
2. Sull'altro PC: estrai lo zip, poi in **Terminale (amministratore)** (cambia il percorso con quello dello zip estratto):
```
powershell -ExecutionPolicy Bypass -File C:\GasterBless-pacchetto\installa.ps1
```
   Per aggiornare, si installa un pacchetto più nuovo sopra quello vecchio: i dati restano.

## Cosa installa
Programma in `C:\Program Files\GasterBless`, dati in `C:\ProgramData\GasterBless`, servizio Windows «GasterBless»
(parte all'accensione, account senza privilegi), icone, regola del firewall solo per la rete di casa, operazioni
pianificate `\GasterBless\Aggiorna` e `\GasterBless\Controlla` (solo nel modo A).

## Togliere
```
powershell -ExecutionPolicy Bypass -File "C:\Program Files\GasterBless\disinstalla.ps1"
```
I dati restano in `C:\ProgramData\GasterBless`.

## Attenzione (13/11/2026)
Il tasto «Aggiorna» installa solo se **tutte** le prove automatiche passano. Oggi 19 prove del server sono ancora
rosse (usano dati tolti nella fase 3b): finché non sono sistemate, «Aggiorna» si ferma ai test e non installa nulla
(la versione installata resta com'è). L'installazione del punto A.2 non fa le prove e funziona.
macOS: il programma si compila (`GOOS=darwin`), ma installatore e servizio per Mac sono da fare (fase 5).
