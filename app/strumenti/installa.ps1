<#
  Installazione di Gaster Bless (nuova app) sul PC del Master — Windows 10/11.
  Va eseguito come amministratore, dalla cartella che contiene GasterBless.exe e questo script:
      powershell -ExecutionPolicy Bypass -File .\installa.ps1
  Cosa fa (ogni passo è scritto a video; si può rilanciare senza danni, per esempio per aggiornare):
    1. copia il programma in C:\Program Files\GasterBless;
    2. crea C:\ProgramData\GasterBless (dati, backup, certificati, contenuti) e il file gasterbless.conf;
    3. COPIA tutto il contenuto della vecchia cartella Documenti\GasterBless\GasterBlessDati (video, immagini,
       musica, canti, manuali, impostazioni e gruppi della vecchia app) in C:\ProgramData\GasterBless\Media
       e controlla che la copia sia completa. La vecchia cartella non viene toccata: si potrà eliminare
       dopo, con lo strumento di rimozione (05_strumenti\rimozione);
    4. crea il servizio "GasterBless" con l'account virtuale NT SERVICE\GasterBless (nessun privilegio di
       amministratore) e l'avvio automatico;
    5. permessi: i dati sono leggibili solo da Gaster Bless e dagli amministratori; la cartella dei contenuti
       anche dall'utente di questo PC (per aggiungere video e immagini a mano);
    6. permette di avviare e fermare il servizio anche senza essere amministratori;
    7. regole del firewall solo per la rete privata e per la VPN (100.64.0.0/10);
    8. avvia il servizio, fa conoscere il certificato di Gaster Bless a questo PC e crea le icone;
    9. crea le operazioni pianificate «\GasterBless\Aggiorna» e «\GasterBless\Controlla» per il tasto «Aggiorna» del Master
       (girano con l'utente collegato al PC, solo quando è collegato, e partono solo quando le chiede Gaster Bless: nessun
       orario). Il servizio riceve solo il permesso di avviarle.
  Non apre porte sul router, non configura VPN o servizi esterni.
#>
param(
    [string]$VecchiaCartella = '',  # la cartella GasterBlessDati della vecchia app (se vuoto: la cerca da solo)
    [int]$Porta = 7600,
    [switch]$DaAggiornamentoAuto    # lanciato da aggiorna_auto.ps1: non ricrea le operazioni pianificate (una è in corso)
)
$ErrorActionPreference = 'Stop'
function Passo($t) { Write-Host "`n== $t" -ForegroundColor Cyan }
function Ferma($t) { Write-Host "`n$t" -ForegroundColor Red; Write-Host "Installazione interrotta: niente è andato perso, si può rilanciare." -ForegroundColor Red; exit 1 }

if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Host "Avvia questo script come amministratore (tasto destro su PowerShell -> Esegui come amministratore)." -ForegroundColor Red
    exit 1
}
$sorgente = Join-Path $PSScriptRoot 'GasterBless.exe'
if (-not (Test-Path $sorgente)) { $sorgente = Join-Path (Split-Path $PSScriptRoot -Parent) 'GasterBless.exe' }
if (-not (Test-Path $sorgente)) { Ferma "GasterBless.exe non trovato accanto allo script: prima crealo con  go build -o GasterBless.exe ./cmd/gasterbless" }

$prog  = Join-Path $env:ProgramFiles 'GasterBless'
$dati  = Join-Path $env:ProgramData 'GasterBless'
$media = Join-Path $dati 'Media'
$conf  = Join-Path $dati 'gasterbless.conf'
$exe   = Join-Path $prog 'GasterBless.exe'
$conto = 'NT SERVICE\GasterBless'

# La vecchia cartella GasterBlessDati: indicata a mano, oppure nei Documenti dell'utente (anche OneDrive).
if (-not $VecchiaCartella) {
    $candidati = @((Join-Path ([Environment]::GetFolderPath('MyDocuments')) 'GasterBless\GasterBlessDati'))
    $candidati += Get-ChildItem 'C:\Users' -Directory -ErrorAction SilentlyContinue | ForEach-Object {
        Join-Path $_.FullName 'Documents\GasterBless\GasterBlessDati'
        Get-ChildItem $_.FullName -Directory -Filter 'OneDrive*' -ErrorAction SilentlyContinue | ForEach-Object {
            Join-Path $_.FullName 'Documents\GasterBless\GasterBlessDati'; Join-Path $_.FullName 'Documenti\GasterBless\GasterBlessDati' }
    }
    $VecchiaCartella = $candidati | Where-Object { (Test-Path (Join-Path $_ 'Video')) -or (Test-Path (Join-Path $_ 'gruppi.json')) } | Select-Object -First 1
}

# La porta deve essere libera: la vecchia Gaster Bless usa la stessa.
$servizio = Get-Service -Name Gaster Bless -ErrorAction SilentlyContinue
$occupata = Get-NetTCPConnection -LocalPort $Porta -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
if ($occupata -and -not ($servizio -and $servizio.Status -eq 'Running')) {
    $chi = (Get-Process -Id $occupata.OwningProcess -ErrorAction SilentlyContinue).Path
    Ferma "La porta $Porta è già usata da: $chi`nChiudi la vecchia Gaster Bless (o la prova aperta in PowerShell, con Ctrl+C) e rilancia."
}
if ($VecchiaCartella) { $VecchiaCartella = $VecchiaCartella.TrimEnd('\') }

Passo "1. Programma in $prog"
if ($servizio -and $servizio.Status -ne 'Stopped') { Stop-Service Gaster Bless -Force -ErrorAction SilentlyContinue }
# Si aspetta che il programma sia chiuso davvero (chiude con calma le connessioni aperte), poi lo si
# sostituisce. Se dopo 30 secondi è ancora aperto, lo si chiude a forza.
for ($i = 0; $i -lt 30 -and (Get-Process -Name Gaster Bless -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $exe }); $i++) { Start-Sleep 1 }
Get-Process -Name Gaster Bless -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $exe } | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep 1
New-Item -ItemType Directory -Force -Path $prog | Out-Null
Copy-Item $sorgente $exe -Force
foreach ($f in 'avvia.bat','ferma.bat','apri.bat','disinstalla.ps1','gasterbless.ico') {
    $p = Join-Path $PSScriptRoot $f; if (Test-Path $p) { Copy-Item $p (Join-Path $prog $f) -Force }
}
Write-Host "Programma copiato."

Passo "2. Dati e configurazione in $dati"
New-Item -ItemType Directory -Force -Path $dati, $media | Out-Null
if (-not (Test-Path $conf)) {
    $esempio = Join-Path $PSScriptRoot 'gasterbless.conf.esempio'
    if (-not (Test-Path $esempio)) { $esempio = Join-Path (Split-Path $PSScriptRoot -Parent) 'gasterbless.conf.esempio' }
    $testo = if (Test-Path $esempio) { Get-Content $esempio -Raw -Encoding UTF8 } else { "" }
    $testo = $testo -replace '(?m)^#?\s*cartella_media\s*=.*$', "cartella_media = $media"
    $testo = $testo -replace '(?m)^#?\s*cartella_dati\s*=.*$', "cartella_dati = $dati"
    $testo = $testo -replace '(?m)^#?\s*porta_https\s*=.*$', "porta_https = $Porta"
    if ($testo -notmatch 'cartella_media') { $testo += "`r`ncartella_media = $media" }
    if ($testo -notmatch 'cartella_dati')  { $testo += "`r`ncartella_dati = $dati" }
    [IO.File]::WriteAllText($conf, $testo, (New-Object Text.UTF8Encoding $false))
    Write-Host "Creato $conf"
} else {
    # Una configurazione fatta con la versione precedente puntava ancora alla vecchia cartella: si corregge.
    $testo = Get-Content $conf -Raw -Encoding UTF8
    if ($testo -match '(?m)^\s*cartella_media\s*=\s*(.+?)\s*$' -and $Matches[1].TrimEnd('\') -ne $media) {
        $testo = $testo -replace '(?m)^\s*cartella_media\s*=.*$', "cartella_media = $media"
        [IO.File]::WriteAllText($conf, $testo, (New-Object Text.UTF8Encoding $false))
        Write-Host "Configurazione aggiornata: i contenuti ora stanno in $media"
    } else { Write-Host "Configurazione esistente lasciata com'è: $conf" }
}

Passo "3. Copia dei contenuti della vecchia Gaster Bless in $media"
if (-not $VecchiaCartella -or -not (Test-Path $VecchiaCartella)) {
    Write-Host "Vecchia cartella GasterBlessDati non trovata: nessun contenuto da copiare." -ForegroundColor Yellow
    Write-Host "Se esiste in un altro posto, rilancia con:  -VecchiaCartella 'D:\percorso\GasterBlessDati'" -ForegroundColor Yellow
} else {
    Write-Host "Da: $VecchiaCartella"
    $file = Get-ChildItem $VecchiaCartella -Recurse -File -Force -ErrorAction SilentlyContinue
    $totale = ($file | Measure-Object Length -Sum).Sum; if (-not $totale) { $totale = 0 }
    $gia = (Get-ChildItem $media -Recurse -File -Force -ErrorAction SilentlyContinue | Measure-Object Length -Sum).Sum; if (-not $gia) { $gia = 0 }
    $serve = [Math]::Max(0, $totale - $gia) + 1GB   # quanto manca da copiare, più 1 GB di margine
    $disco = (Get-PSDrive ($media.Substring(0, 1))).Free
    Write-Host ("{0} file, {1:N1} GB. Spazio libero sul disco: {2:N1} GB." -f $file.Count, ($totale / 1GB), ($disco / 1GB))
    if ($disco -lt $serve) { Ferma ("Spazio insufficiente: libera almeno {0:N1} GB sul disco C: e rilancia." -f (($serve - $disco) / 1GB)) }
    Write-Host "Copia in corso (con molti video può volerci qualche minuto)..."
    # robocopy: copia anche file grandi, riprende se interrotto, non cancella nulla nella destinazione.
    & robocopy $VecchiaCartella.TrimEnd('\') $media /E /COPY:DT /DCOPY:T /R:2 /W:2 /NFL /NDL /NP /NJH | Out-Host
    if ($LASTEXITCODE -ge 8) { Ferma "La copia non è riuscita (codice robocopy $LASTEXITCODE). La vecchia cartella è intatta." }
    # Verifica: ogni file della vecchia cartella deve esserci nella nuova, con la stessa dimensione.
    $mancanti = @()
    foreach ($f in $file) {
        $rel = $f.FullName.Substring($VecchiaCartella.TrimEnd('\').Length + 1)
        $n = Get-Item -LiteralPath (Join-Path $media $rel) -Force -ErrorAction SilentlyContinue
        if (-not $n -or $n.Length -ne $f.Length) { $mancanti += $rel }
    }
    if ($mancanti.Count) {
        $mancanti | Select-Object -First 20 | ForEach-Object { Write-Host "  manca o è diverso: $_" -ForegroundColor Red }
        Ferma ("{0} file non copiati correttamente. Rilancia lo script: la copia riprende da dove si è fermata." -f $mancanti.Count)
    }
    $verbale = Join-Path $media 'COPIA_DA_GASTERBLESSDATI.txt'
    $righe = @("Copia completata il $(Get-Date -Format 'dd/MM/yyyy HH:mm')", "Da: $VecchiaCartella", "A:  $media",
        ("File: {0}, {1:N1} GB, tutti verificati (stessa dimensione)." -f $file.Count, ($totale / 1GB)), "",
        "La vecchia cartella non serve più a Gaster Bless: si può eliminare con 05_strumenti\rimozione,",
        "dopo aver controllato che la nuova app funzioni e aver importato i dati (Impostazioni -> Importa).")
    [IO.File]::WriteAllLines($verbale, $righe, (New-Object Text.UTF8Encoding $false))
    Write-Host ("Copia verificata: {0} file su {0} presenti e uguali." -f $file.Count) -ForegroundColor Green
}
foreach ($sub in 'Immagini','Video','Musica','Manuali','Musica\Combattimento','Musica\Canti\Viandante','Musica\Canti\Bardo') {
    New-Item -ItemType Directory -Force -Path (Join-Path $media $sub) | Out-Null
}

Passo "4. Servizio Windows 'GasterBless'"
# Il servizio si crea con New-Service: con sc.exe, PowerShell 5.1 rovina le virgolette del percorso.
$bin = "`"$exe`" -config `"$conf`""
$descr = "Gaster Bless: mappa, iniziativa e schede quinta edizione per la campagna del Master."
if (-not $servizio) {
    New-Service -Name Gaster Bless -BinaryPathName $bin -DisplayName 'GasterBless' -StartupType Automatic -Description $descr | Out-Null
} else {
    Set-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Services\GasterBless' -Name ImagePath -Value $bin
    Set-Service Gaster Bless -StartupType Automatic
}
if (-not (Get-Service Gaster Bless -ErrorAction SilentlyContinue)) { Ferma "Il servizio 'GasterBless' non è stato creato." }
# Account virtuale senza privilegi (non ha password: la gestisce Windows).
& sc.exe config Gaster Bless obj= $conto | Out-Host
if ($LASTEXITCODE -ne 0) { Ferma "Non riesco a impostare l'account del servizio ($conto): codice $LASTEXITCODE." }
# Ripartenza automatica (rivista il 2026-11-08 dopo i blocchi in standby): 3 tentativi (5 s, 10 s, 30 s), il conto si
# azzera dopo un'ora; con failureflag vale anche quando Gaster Bless si ferma da sola con un errore (non solo se va in crash).
& sc.exe failure Gaster Bless reset= 3600 actions= restart/5000/restart/10000/restart/30000 | Out-Null
& sc.exe failureflag Gaster Bless 1 | Out-Null
Write-Host "Servizio creato: parte da solo all'accensione del PC, con l'account $conto."

Passo "5. Permessi"
# Dati (database, chiave, backup): solo SYSTEM, amministratori e il servizio. Si usano i codici (SID)
# dei gruppi, che non cambiano con la lingua di Windows.
& icacls $dati /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' "${conto}:(OI)(CI)M" /Q | Out-Null
& icacls "$dati\*" /reset /T /C /Q | Out-Null
# Contenuti: anche l'utente che usa il PC, per aggiungere video e immagini a mano.
$utente = (Get-CimInstance Win32_ComputerSystem).UserName
if ($utente) { & icacls $media /grant "${utente}:(OI)(CI)M" /Q | Out-Null; Write-Host "Accesso ai contenuti anche per $utente." }
Write-Host "Permessi impostati."

Passo "6. Avvio e arresto senza diritti di amministratore"
$sd = (& sc.exe sdshow Gaster Bless | Where-Object { $_ -match 'D:' }) -join ''
if ($sd -and $sd -notmatch '\(A;;RPWPLC;;;IU\)') {
    $sd = $sd -replace 'D:', 'D:(A;;RPWPLC;;;IU)'
    & sc.exe sdset Gaster Bless $sd | Out-Host
}

Passo "7. Firewall (solo rete privata e VPN)"
Get-NetFirewallRule -DisplayName 'Gaster Bless (*' -ErrorAction SilentlyContinue | Remove-NetFirewallRule
New-NetFirewallRule -DisplayName 'Gaster Bless (rete di casa)' -Direction Inbound -Program $exe -Action Allow -Profile Private | Out-Null
New-NetFirewallRule -DisplayName 'Gaster Bless (VPN)' -Direction Inbound -Program $exe -Action Allow -RemoteAddress '100.64.0.0/10' | Out-Null
Write-Host "Regole create. Nessuna porta viene aperta sul router."

Passo "8. Avvio, certificato e icone"
try { Start-Service Gaster Bless } catch {
    $log = Join-Path $dati 'registro\gasterbless.log'
    Write-Host "Il servizio non è partito: $($_.Exception.Message)" -ForegroundColor Red
    if (Test-Path $log) { Write-Host "Ultime righe del registro ($log):"; Get-Content $log -Tail 15 | Out-Host }
    Ferma "Mandami queste righe: le correggo e poi si rilancia lo script."
}
Write-Host "Servizio avviato."
$ca = Join-Path $dati 'certificati\gasterbless-ca.crt'
for ($i = 0; $i -lt 20 -and -not (Test-Path $ca); $i++) { Start-Sleep 1 }
if (Test-Path $ca) {
    Import-Certificate -FilePath $ca -CertStoreLocation Cert:\LocalMachine\Root | Out-Null
    Write-Host "Certificato di Gaster Bless riconosciuto da questo PC."
} else { Write-Host "Il certificato non è ancora pronto: controlla il registro in $dati\registro\gasterbless.log" -ForegroundColor Yellow }
$desk = [Environment]::GetFolderPath('CommonDesktopDirectory')
$start = Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs\GasterBless'
New-Item -ItemType Directory -Force -Path $start | Out-Null
$sh = New-Object -ComObject WScript.Shell
# GasterBless.exe non ha un'icona dentro: si usa gasterbless.ico (stessa immagine dell'app sui telefoni).
$ico = Join-Path $prog 'gasterbless.ico'; if (-not (Test-Path $ico)) { $ico = "$exe" }
foreach ($icona in @(
    @{ Nome = 'GasterBless';         Bat = 'apri.bat';  Dove = @($desk, $start) },
    @{ Nome = 'Avvia Gaster Bless';   Bat = 'avvia.bat'; Dove = @($start) },
    @{ Nome = 'Ferma Gaster Bless';   Bat = 'ferma.bat'; Dove = @($start) })) {
    foreach ($d in $icona.Dove) {
        $l = $sh.CreateShortcut((Join-Path $d ($icona.Nome + '.lnk')))
        $l.TargetPath = Join-Path $prog $icona.Bat; $l.WorkingDirectory = $prog; $l.IconLocation = "$ico,0"; $l.Save()
    }
}
$l = $sh.CreateShortcut((Join-Path $start 'Contenuti di Gaster Bless (video, immagini, musica).lnk')); $l.TargetPath = $media; $l.Save()
# Windows ricorda le icone vecchie: gli si chiede di ricaricarle.
& "$env:SystemRoot\System32\ie4uinit.exe" -show 2>$null

Passo "9. Operazioni pianificate per il tasto «Aggiorna»"
$script = Join-Path $PSScriptRoot 'aggiorna_auto.ps1'
if ($DaAggiornamentoAuto) {
    Write-Host "Lanciato dall'aggiornamento automatico: operazioni lasciate come sono."
} elseif (-not (Test-Path $script)) {
    Write-Host "aggiorna_auto.ps1 non trovato accanto a questo script: il tasto «Aggiorna» resterà non disponibile." -ForegroundColor Yellow
} else {
    try {
        # L'utente collegato al PC: le sue credenziali GitHub servono a git. Deve essere amministratore.
        $chi = (Get-CimInstance Win32_ComputerSystem).UserName
        if (-not $chi) { $chi = [Security.Principal.WindowsIdentity]::GetCurrent().Name }
        $sid = (New-Object Security.Principal.NTAccount $conto).Translate([Security.Principal.SecurityIdentifier]).Value
        $pr = New-ScheduledTaskPrincipal -UserId $chi -LogonType Interactive -RunLevel Highest
        $st = New-ScheduledTaskSettingsSet -MultipleInstances IgnoreNew -ExecutionTimeLimit (New-TimeSpan -Minutes 40) -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries
        $svc = New-Object -ComObject Schedule.Service
        $svc.Connect()
        foreach ($op in @(@{ Nome = 'Aggiorna'; Arg = ''; Descr = 'Gaster Bless: aggiornamento da GitHub (tasto «Aggiorna» del Master).' },
                          @{ Nome = 'Controlla'; Arg = ' -SoloControllo'; Descr = 'Gaster Bless: elenco delle novità su GitHub (tasto «Controlla novità»).' })) {
            $az = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument ("-NoProfile -ExecutionPolicy Bypass -WindowStyle Minimized -File `"$script`"" + $op.Arg)
            Register-ScheduledTask -TaskPath '\GasterBless\' -TaskName $op.Nome -Action $az -Principal $pr -Settings $st -Description $op.Descr -Force | Out-Null
            # Permesso di leggere e avviare l'operazione per il servizio Gaster Bless (e nient'altro).
            $t = $svc.GetFolder('\GasterBless').GetTask($op.Nome)
            $sd = $t.GetSecurityDescriptor(4)
            if ($sd -notmatch [regex]::Escape($sid)) { $t.SetSecurityDescriptor($sd + "(A;;GRGX;;;$sid)", 0) }
        }
        $cart = $svc.GetFolder('\GasterBless')
        $sd = $cart.GetSecurityDescriptor(4)
        if ($sd -notmatch [regex]::Escape($sid)) { $cart.SetSecurityDescriptor($sd + "(A;;GRGX;;;$sid)", 0) }
        Write-Host "Operazioni create per l'utente $chi. Il tasto «Aggiorna» funziona quando $chi è collegato a Windows su questo PC."
    } catch {
        Write-Host "Operazioni pianificate non create: $($_.Exception.Message)" -ForegroundColor Yellow
        Write-Host "Gaster Bless funziona lo stesso; per aggiornare resta aggiorna.ps1." -ForegroundColor Yellow
    }
}

Write-Host "`nFatto. Apri Gaster Bless dall'icona sul Desktop (https://localhost:$Porta)." -ForegroundColor Green
Write-Host "1. Al primo avvio crea l'account del Master."
Write-Host "2. Impostazioni -> Importa dalla vecchia Gaster Bless: prima '1. Prova', poi '2. Importa'."
Write-Host "3. Quando tutto funziona, la vecchia app e la cartella GasterBlessDati si eliminano con 05_strumenti\rimozione."
