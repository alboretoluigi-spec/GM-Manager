<#
  Aggiornamento automatico di Gaster Bless da GitHub (tasto «Aggiorna» del pannello del Master, vedi docs/PIANO_AGGIORNA.md).
  Va eseguito come amministratore. Per ora si lancia a mano; più avanti lo lancerà l'operazione pianificata
  «\GasterBless\Aggiorna», quando il Master preme il tasto.

  Prova (non ferma e non sostituisce nulla: scarica solo l'elenco delle novità, fa i test e compila in una cartella a parte):
      powershell -ExecutionPolicy Bypass -File C:\Progetti\Progetto_GasterBless\04_nuova_app\strumenti\aggiorna_auto.ps1 -Prova
  Solo l'elenco delle novità (lo usa l'operazione «\GasterBless\Controlla», tasto «Controlla novità»):
      powershell -ExecutionPolicy Bypass -File C:\Progetti\Progetto_GasterBless\04_nuova_app\strumenti\aggiorna_auto.ps1 -SoloControllo
  Aggiornamento vero:
      powershell -ExecutionPolicy Bypass -File C:\Progetti\Progetto_GasterBless\04_nuova_app\strumenti\aggiorna_auto.ps1

  Passi: novità da GitHub (solo avanzamento lineare, mai fusioni) -> go vet e go test -> compilazione in una cartella a
  parte -> Gaster Bless ferma -> copia di sicurezza del database e del programma -> nuovo programma -> Gaster Bless riparte ->
  controllo che risponda. Se non risponde: torna da sola alla versione precedente (programma e, se la versione nuova
  aveva migrazioni, anche il database).
  Stato in C:\ProgramData\GasterBless\aggiornamento\stato.json, registro completo nella stessa cartella.
  aggiorna.ps1 (manuale) resta come strada di riserva.
#>
param(
    [switch]$Prova,   # fa tutto tranne fermare, sostituire e riavviare Gaster Bless
    [switch]$Forza,   # ricompila e reinstalla anche se su GitHub non ci sono novità
    [switch]$SoloControllo  # scarica solo l'elenco delle novità e lo scrive in controllo.json
)
$ErrorActionPreference = 'Stop'
# git scrive in UTF-8: senza questa riga PowerShell lo legge con la codifica della console e «» o le lettere accentate
# nei titoli delle novità diventano caratteri storti (visto il 2026-11-11).
try { [Console]::OutputEncoding = New-Object Text.UTF8Encoding $false } catch { }

$strumenti = $PSScriptRoot
$app       = Split-Path $strumenti -Parent            # ...\04_nuova_app
$repo      = Split-Path $app -Parent                  # ...\Progetto_Gaster Bless
$prog      = Join-Path $env:ProgramFiles 'GasterBless'
$exe       = Join-Path $prog 'GasterBless.exe'
$exePrec   = Join-Path $prog 'Gaster Bless.precedente.exe'
$dati      = Join-Path $env:ProgramData 'GasterBless'
$conf      = Join-Path $dati 'gasterbless.conf'
$cartAgg   = Join-Path $dati 'aggiornamento'
$fileStato = Join-Path $cartAgg $(if ($SoloControllo) { 'controllo.json' } else { 'stato.json' })
$blocco    = Join-Path $cartAgg 'in_corso.lock'
$adesso    = Get-Date -Format 'yyyyMMdd-HHmmss'
$registro  = Join-Path $cartAgg "registro-$adesso.txt"
$copiaDB   = Join-Path $cartAgg "prima-$adesso"
$nuovoExe  = Join-Path $cartAgg 'nuovo\GasterBless.exe'

# Situazione, aggiornata a ogni passo e scritta in stato.json (la leggerà il pannello di Gaster Bless).
$S = [ordered]@{
    inizio = (Get-Date -Format s); fine = ''; prova = [bool]$Prova; soloControllo = [bool]$SoloControllo; passo = 'avvio'; esito = 'in_corso'
    messaggio = ''; da = ''; a = ''; novita = @(); migrazioni = $false; registro = $registro
}
$fermato = $false; $sostituito = $false; $applicato = $false; $schemaCambiato = $false; $vecchio = ''

function Scrivi-Stato {
    try {
        $j = $S | ConvertTo-Json -Depth 4
        [IO.File]::WriteAllText($fileStato, $j, (New-Object Text.UTF8Encoding $false))
    } catch { Write-Host "Non riesco a scrivere $fileStato : $($_.Exception.Message)" -ForegroundColor Yellow }
}
function Passo($nome, $testo) { $S.passo = $nome; Scrivi-Stato; Write-Host "`n== $testo" -ForegroundColor Cyan }

# Lancia un programma esterno senza che i messaggi su stderr (normali per git e go) fermino lo script.
function Esegui($cosa, [string[]]$argomenti) {
    $prima = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
    & $cosa @argomenti 2>&1 | ForEach-Object { "$_" } | Out-Host
    $codice = $LASTEXITCODE
    $ErrorActionPreference = $prima
    return $codice
}
function Leggi($cosa, [string[]]$argomenti) {
    $prima = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
    $out = & $cosa @argomenti 2>$null
    $codice = $LASTEXITCODE
    $ErrorActionPreference = $prima
    if ($codice -ne 0) { return $null }
    return $out
}

function Porta {
    $p = 7600
    if (Test-Path $conf) {
        $t = Get-Content $conf -Raw -Encoding UTF8
        if ($t -match '(?m)^\s*porta_https\s*=\s*(\d+)') { $p = [int]$Matches[1] }
    }
    return $p
}
# Gaster Bless risponde? /api/v1/io risponde anche senza accesso. Certificato del PC: controllo senza verifica, solo in locale.
function Risponde([int]$secondi) {
    $url = "https://127.0.0.1:$(Porta)/api/v1/io"
    for ($i = 0; $i -lt $secondi; $i += 3) {
        $sv = Get-Service Gaster Bless -ErrorAction SilentlyContinue
        if ($sv -and $sv.Status -eq 'Running') {
            $codice = Leggi 'curl.exe' @('-k', '-s', '-o', 'NUL', '-w', '%{http_code}', '--max-time', '5', $url)
            if ("$codice" -eq '200') { return $true }
        }
        Start-Sleep 3
    }
    return $false
}
function Ferma-Gaster Bless {
    $sv = Get-Service Gaster Bless -ErrorAction SilentlyContinue
    if ($sv -and $sv.Status -ne 'Stopped') { Stop-Service Gaster Bless -Force -ErrorAction SilentlyContinue }
    for ($i = 0; $i -lt 30 -and (Get-Process -Name Gaster Bless -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $exe }); $i++) { Start-Sleep 1 }
    Get-Process -Name Gaster Bless -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $exe } | Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep 1
}
function Copia-DB($da, $a) {
    foreach ($n in 'gasterbless.db', 'gasterbless.db-wal', 'gasterbless.db-shm') {
        $f = Join-Path $da $n
        if (Test-Path $f) { Copy-Item $f (Join-Path $a $n) -Force }
        elseif ($da -eq $copiaDB) { Remove-Item (Join-Path $a $n) -Force -ErrorAction SilentlyContinue }
    }
}

# Rimette la versione precedente (programma e, se servono, i dati) e riavvia Gaster Bless.
function Torna-Indietro($motivo) {
    Write-Host "`nLa nuova versione non funziona ($motivo): torno alla precedente." -ForegroundColor Yellow
    Ferma-Gaster Bless
    if (Test-Path $exePrec) { Copy-Item $exePrec $exe -Force }
    # Il database si rimette sempre com'era un attimo prima (Gaster Bless era ferma): la versione nuova potrebbe aver
    # applicato migrazioni anche se GitHub non ne mostrava (per esempio con -Forza su una Gaster Bless installata vecchia).
    if (Test-Path (Join-Path $copiaDB 'gasterbless.db')) {
        Copia-DB $copiaDB $dati
        Write-Host "Database rimesso com'era prima dell'aggiornamento."
    }
    try { Start-Service Gaster Bless } catch { }
    $ok = Risponde 60
    if ($vecchio) {
        $null = Esegui 'git' @('-C', $repo, 'reset', '--keep', $vecchio)   # così il prossimo tentativo ritrova le novità
    }
    $S.esito = 'ripristinato'
    $S.messaggio = "Nuova versione non avviata ($motivo). Rimessa la versione precedente" + $(if ($ok) { ': Gaster Bless risponde.' } else { ', ma Gaster Bless NON risponde: guarda il registro.' })
    Write-Host $S.messaggio -ForegroundColor $(if ($ok) { 'Yellow' } else { 'Red' })
}

function Ferma($testo) {
    Write-Host "`n$testo" -ForegroundColor Red
    if ($sostituito) { Torna-Indietro $testo }
    else {
        if ($fermato) { try { Start-Service Gaster Bless } catch { }; $null = Risponde 60 }
        if ($applicato -and $vecchio) { $null = Esegui 'git' @('-C', $repo, 'reset', '--keep', $vecchio) }
        $S.esito = 'fallito'
        $S.messaggio = "$testo La Gaster Bless installata non è stata cambiata."
        Write-Host "La Gaster Bless installata non è stata cambiata." -ForegroundColor Red
    }
    throw 'FERMATO'
}

# ---------------------------------------------------------------------------------------------------------------
if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Host "Avvia PowerShell come amministratore (clic destro su Start -> Terminale (amministratore)) e rilancia." -ForegroundColor Red
    exit 1
}
New-Item -ItemType Directory -Force -Path $cartAgg | Out-Null
if ((Test-Path $blocco) -and ((Get-Date) - (Get-Item $blocco).LastWriteTime).TotalMinutes -lt 30) {
    Write-Host "Un altro aggiornamento è già in corso (iniziato $((Get-Item $blocco).LastWriteTime)). Riprova tra qualche minuto." -ForegroundColor Yellow
    exit 1
}
Set-Content -Path $blocco -Value $PID
try { Start-Transcript -Path $registro | Out-Null } catch { }
$codiceUscita = 0
try {
    if ($Prova) { Write-Host "MODALITÀ DI PROVA: Gaster Bless non verrà fermata né sostituita." -ForegroundColor Magenta }

    Passo 'controlli' '1. Controlli'
    $strGo = 'C:\Progetti\Strumenti'
    $env:Path = (Join-Path $strGo 'go\bin') + ';' + $env:Path
    $env:GOPATH = Join-Path $strGo 'gopath'
    $env:GOCACHE = Join-Path $strGo 'gocache'
    $env:GOTOOLCHAIN = 'local'
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) { Ferma "Go non trovato in $strGo\go." }
    if (-not (Get-Command git -ErrorAction SilentlyContinue)) { Ferma "Git non trovato." }
    if (-not (Get-Command curl.exe -ErrorAction SilentlyContinue)) { Ferma "curl.exe non trovato (c'è di serie in Windows 10 e 11)." }
    if (-not (Get-Service Gaster Bless -ErrorAction SilentlyContinue)) { Ferma "Il servizio Gaster Bless non esiste: prima va installato con aggiorna.ps1." }
    if (-not (Test-Path $exe)) { Ferma "Programma non trovato: $exe" }
    Write-Host "Repository: $repo"
    Write-Host "Programma:  $exe"
    Write-Host ("Gaster Bless adesso " + $(if (Risponde 6) { 'risponde.' } else { 'NON risponde.' }))
    $modificati = Leggi 'git' @('-C', $repo, 'status', '--porcelain', '--untracked-files=no')
    if ($modificati) {
        $modificati | ForEach-Object { Write-Host "  modificato: $_" -ForegroundColor Yellow }
        Ferma "Nella cartella del progetto ci sono file modificati a mano: l'aggiornamento automatico non li sovrascrive. Salvali o annullali (git status) e riprova."
    }

    Passo 'novita' '2. Novità da GitHub'
    $vecchio = "$(Leggi 'git' @('-C', $repo, 'rev-parse', 'HEAD'))".Trim()
    $S.da = $vecchio
    if ((Esegui 'git' @('-C', $repo, 'fetch', '--quiet')) -ne 0) {
        Ferma "Non riesco a scaricare da GitHub (rete assente o accesso a GitHub non riuscito con questo utente di Windows)."
    }
    $remoto = "$(Leggi 'git' @('-C', $repo, 'rev-parse', '@{u}'))".Trim()
    if (-not $remoto) { Ferma "Il ramo locale non è collegato a GitHub (manca il ramo remoto)." }
    $S.a = $remoto
    $elenco = Leggi 'git' @('-C', $repo, 'log', '--format=%h  %s', "HEAD..$remoto")
    $S.novita = @($elenco | Where-Object { $_ })
    if ((Esegui 'git' @('-C', $repo, 'merge-base', '--is-ancestor', 'HEAD', $remoto)) -ne 0) {
        Ferma "La copia sul PC e GitHub sono andate in direzioni diverse: serve una sistemazione a mano (git status, git log)."
    }
    if ($SoloControllo) {
        $S.esito = 'controllo'
        $S.messaggio = $(if ($S.novita.Count) { "Su GitHub ci sono $($S.novita.Count) novità." } else { 'Gaster Bless è già aggiornata: su GitHub non ci sono novità.' })
        Write-Host $S.messaggio -ForegroundColor Green
        $S.novita | ForEach-Object { Write-Host "  $_" }
        throw 'FINE'
    }
    if ($S.novita.Count -eq 0 -and -not $Forza -and -not $Prova) {
        $S.esito = 'nessuna_novita'; $S.messaggio = 'Gaster Bless è già aggiornata: su GitHub non ci sono novità.'
        Write-Host $S.messaggio -ForegroundColor Green
        throw 'FINE'
    }
    Write-Host "$($S.novita.Count) novità:"
    $S.novita | ForEach-Object { Write-Host "  $_" }
    $diff = Leggi 'git' @('-C', $repo, 'diff', '--name-only', $vecchio, $remoto)
    $cambiati = @($diff | Where-Object { $_ })
    $schemaCambiato = [bool]($cambiati | Where-Object { $_ -like '04_nuova_app/internal/db/schema/*' })
    $installaCambiato = [bool]($cambiati | Where-Object { $_ -eq '04_nuova_app/strumenti/installa.ps1' })
    $S.migrazioni = $schemaCambiato
    if ($schemaCambiato) { Write-Host "Ci sono modifiche al database (migrazioni)." -ForegroundColor Yellow }
    if ($Prova) {
        Write-Host "Prova: le novità NON vengono applicate; test e compilazione si fanno sui file attuali." -ForegroundColor Magenta
    } else {
        if ((Esegui 'git' @('-C', $repo, 'merge', '--ff-only', '--quiet', $remoto)) -ne 0) { Ferma "Non riesco ad applicare le novità (git merge --ff-only)." }
        $applicato = $true
        Write-Host "Novità applicate alla cartella del progetto."
    }

    Set-Location $app
    Passo 'test' '3. Controlli automatici (go vet e go test: qualche minuto)'
    if ((Esegui 'go' @('vet', './...')) -ne 0) { Ferma "go vet ha trovato problemi." }
    if ((Esegui 'go' @('test', './...')) -ne 0) { Ferma "Alcuni test automatici non sono passati: la nuova versione non viene installata." }
    Write-Host "Test superati." -ForegroundColor Green

    Passo 'compila' '4. Compilazione'
    $versione = "$(Leggi 'git' @('-C', $repo, 'describe', '--always', '--dirty'))".Trim() + "-$adesso"
    New-Item -ItemType Directory -Force -Path (Split-Path $nuovoExe) | Out-Null
    if ((Esegui 'go' @('build', '-ldflags', "-X main.Versione=$versione", '-o', $nuovoExe, './cmd/gasterbless')) -ne 0) {
        Ferma "Compilazione non riuscita."
    }
    $v = "$(Leggi $nuovoExe @('versione'))".Trim()
    if (-not $v) { Ferma "Il programma appena compilato non si avvia." }
    Write-Host "Pronto: $v"

    if ($Prova) {
        $S.esito = 'prova_ok'
        $S.messaggio = "Prova riuscita: test superati e programma compilato ($v). Gaster Bless non è stata toccata."
        Write-Host "`n$($S.messaggio)" -ForegroundColor Green
        throw 'FINE'
    }

    Passo 'installa' '5. Installazione (Gaster Bless si ferma per circa un minuto)'
    Ferma-Gaster Bless; $fermato = $true
    New-Item -ItemType Directory -Force -Path $copiaDB | Out-Null
    Copia-DB $dati $copiaDB
    Copy-Item $exe $exePrec -Force
    Write-Host "Copia di sicurezza: database in $copiaDB, programma in $exePrec"
    Copy-Item $nuovoExe (Join-Path $app 'GasterBless.exe') -Force   # come aggiorna.ps1: installa.ps1 prende l'exe da qui
    if (Test-Path (Join-Path $strumenti 'GasterBless.exe')) { Copy-Item $nuovoExe (Join-Path $strumenti 'GasterBless.exe') -Force }  # installa.ps1 guarda prima qui
    $sostituito = $true
    if ($installaCambiato) {
        Write-Host "installa.ps1 è cambiato: reinstallazione completa." -ForegroundColor Yellow
        $prima = $ErrorActionPreference; $ErrorActionPreference = 'Continue'
        # -DaAggiornamentoAuto: installa.ps1 non ricrea le operazioni pianificate mentre una di esse è in corso.
        & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $strumenti 'installa.ps1') -DaAggiornamentoAuto | Out-Host
        $codice = $LASTEXITCODE; $ErrorActionPreference = $prima
        if ($codice -ne 0) { Ferma "installa.ps1 non è riuscito (codice $codice)." }
    } else {
        Copy-Item $nuovoExe $exe -Force
        foreach ($f in 'avvia.bat', 'ferma.bat', 'apri.bat', 'disinstalla.ps1', 'gasterbless.ico') {
            $p = Join-Path $strumenti $f; if (Test-Path $p) { Copy-Item $p (Join-Path $prog $f) -Force }
        }
        try { Start-Service Gaster Bless } catch { Write-Host "Avvio: $($_.Exception.Message)" -ForegroundColor Yellow }
    }

    Passo 'verifica' '6. Gaster Bless risponde?'
    if (-not (Risponde 90)) { Ferma "Dopo 90 secondi Gaster Bless non risponde." }
    $S.esito = 'riuscito'
    $S.messaggio = "Aggiornamento riuscito: $($S.novita.Count) novità, versione $v."
    Write-Host "`n$($S.messaggio)" -ForegroundColor Green
    # Si tengono solo le ultime 5 copie di sicurezza del database.
    Get-ChildItem $cartAgg -Directory -Filter 'prima-*' | Sort-Object Name -Descending | Select-Object -Skip 5 | Remove-Item -Recurse -Force -ErrorAction SilentlyContinue
}
catch {
    if ("$_" -ne 'FINE') {
        $codiceUscita = 1
        if ("$_" -ne 'FERMATO') {
            Write-Host "`nErrore inatteso: $($_.Exception.Message)" -ForegroundColor Red
            try { Ferma "Errore inatteso: $($_.Exception.Message)." } catch { }
        }
    }
}
finally {
    $S.fine = (Get-Date -Format s)
    Scrivi-Stato
    Remove-Item $blocco -Force -ErrorAction SilentlyContinue
    try { Stop-Transcript | Out-Null } catch { }
    Get-ChildItem $cartAgg -Filter 'registro-*.txt' | Sort-Object Name -Descending | Select-Object -Skip 20 | Remove-Item -Force -ErrorAction SilentlyContinue
}
Write-Host "Registro: $registro"
exit $codiceUscita
