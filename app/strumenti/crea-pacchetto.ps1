<#
  Crea il PACCHETTO COMPLETO di GASTER BLESS – MASTER SUITE da installare su un PC Windows (zip con tutto il necessario).
      powershell -ExecutionPolicy Bypass -File .\crea-pacchetto.ps1 [-Destinazione D:\]
  Il pacchetto contiene: GasterBless.exe, gli script di installazione e di gestione, l'icona, la configurazione
  d'esempio, le guide in PDF e LEGGIMI_INSTALLAZIONE.txt. NON contiene account, schede, documenti, backup né contenuti
  del Master: per spostare anche quelli si usano un backup e il codice di ripristino.
  Il tasto «Aggiorna» dell'app funziona solo sulle installazioni fatte dalla cartella del progetto (copia di GitHub):
  con il pacchetto si aggiorna installando un pacchetto più nuovo sopra quello vecchio (i dati restano).
#>
param([string]$Destinazione = (Join-Path ([Environment]::GetFolderPath('MyDocuments')) 'GasterBless\Pacchetti'))
$ErrorActionPreference = 'Stop'
$app = Split-Path $PSScriptRoot -Parent
$exe = @((Join-Path $app 'GasterBless.exe'), (Join-Path $PSScriptRoot 'GasterBless.exe'), (Join-Path $env:ProgramFiles 'GasterBless\GasterBless.exe')) |
    Where-Object { Test-Path $_ } | Select-Object -First 1
if (-not $exe) { Write-Host "GasterBless.exe non trovato: prima crealo (aggiorna.ps1 oppure  go build -o GasterBless.exe ./cmd/gasterbless)." -ForegroundColor Red; exit 1 }
$versione = "$(& $exe versione 2>$null)".Trim()
$nome = "GasterBless-pacchetto-" + (Get-Date -Format 'yyyy-MM-dd-HHmm')
$dest = Join-Path $Destinazione $nome
New-Item -ItemType Directory -Force -Path $dest | Out-Null
Copy-Item $exe (Join-Path $dest 'GasterBless.exe')
foreach ($f in 'installa.ps1','disinstalla.ps1','avvia.bat','ferma.bat','apri.bat','gasterbless.ico','LEGGIMI_INSTALLAZIONE.txt',
               'reimposta-password-master.ps1','REIMPOSTA_PASSWORD_MASTER.bat','ripristina-backup.ps1','RIPRISTINA_BACKUP.bat',
               'ottimizza-media.ps1','OTTIMIZZA_MEDIA.bat') {
    $p = Join-Path $PSScriptRoot $f; if (Test-Path $p) { Copy-Item $p $dest }
}
$p = Join-Path $app 'gasterbless.conf.esempio'; if (Test-Path $p) { Copy-Item $p $dest }
New-Item -ItemType Directory -Force -Path (Join-Path $dest 'guide') | Out-Null
Get-ChildItem (Join-Path $app 'guide') -Filter '*.pdf' -ErrorAction SilentlyContinue | Copy-Item -Destination (Join-Path $dest 'guide')
[IO.File]::WriteAllText((Join-Path $dest 'VERSIONE.txt'), "GASTER BLESS – MASTER SUITE`r`n$versione`r`nPacchetto creato il $(Get-Date -Format 'dd/MM/yyyy HH:mm')`r`n", (New-Object Text.UTF8Encoding $false))
$zip = "$dest.zip"
if (Test-Path $zip) { Remove-Item $zip -Force }
Compress-Archive -Path (Join-Path $dest '*') -DestinationPath $zip
Write-Host "Pacchetto pronto:" -ForegroundColor Green
Write-Host "  cartella: $dest"
Write-Host "  zip:      $zip"
Write-Host "Sul PC di destinazione: estrai lo zip, poi tasto destro su PowerShell -> Esegui come amministratore e:"
Write-Host "  powershell -ExecutionPolicy Bypass -File <cartella estratta>\installa.ps1"
