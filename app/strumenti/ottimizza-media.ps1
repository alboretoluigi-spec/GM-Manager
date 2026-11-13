<#
  Media più leggeri (2026-11-08): crea copie leggere di immagini e video pesanti della cartella Media di Gaster Bless
  (in Media\_leggere), con ffmpeg. Gli originali non vengono toccati: Gaster Bless manda ai dispositivi la copia leggera
  quando c'è. Si può ripetere quando si aggiungono contenuti: rifà solo i file nuovi o cambiati.
  Uso: doppio clic su OTTIMIZZA_MEDIA.bat (chiede i permessi di amministratore). Gaster Bless può restare accesa.
  Serve ffmpeg in C:\Progetti\Strumenti\ffmpeg (istruzioni in guide\GUIDA_MASTER.txt, «Media più leggeri»).
#>
$ErrorActionPreference = 'Continue'
function Ferma($t) { Write-Host "`n$t" -ForegroundColor Red; Read-Host "`nPremi invio per chiudere"; exit 1 }
if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Ferma "Servono i permessi di amministratore: usa OTTIMIZZA_MEDIA.bat."
}
$exe  = Join-Path $env:ProgramFiles 'GasterBless\GasterBless.exe'
$conf = Join-Path $env:ProgramData 'GasterBless\gasterbless.conf'
if (-not (Test-Path $exe))  { Ferma "Gaster Bless non è installata in $exe." }
if (-not (Test-Path $conf)) { Ferma "Configurazione non trovata: $conf." }
$aiuto = (& $exe -h 2>&1 | Out-String)
if ($aiuto -notmatch 'ottimizza-media') { Ferma "La Gaster Bless installata è vecchia: prima aggiornala con strumenti\aggiorna.ps1." }
# ffmpeg: in C:\Progetti\Strumenti (anche in sottocartelle, come le crea lo zip), installato con winget, o nel PATH.
$ff = $null
$cerca = @('C:\Progetti\Strumenti', (Join-Path $env:LOCALAPPDATA 'Microsoft\WinGet\Links'), (Join-Path $env:LOCALAPPDATA 'Microsoft\WinGet\Packages'))
foreach ($dir in $cerca) {
    if (-not $ff -and (Test-Path $dir)) {
        $x = Get-ChildItem $dir -Recurse -Filter ffmpeg.exe -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($x) { $ff = $x.FullName }
    }
}
if (-not $ff) { $c = Get-Command ffmpeg.exe -ErrorAction SilentlyContinue; if ($c) { $ff = $c.Source } }
if (-not $ff) { Ferma "ffmpeg non trovato (cercato in C:\Progetti\Strumenti, nelle installazioni di winget e nel PATH). Installalo con:  winget install --id Gyan.FFmpeg  oppure scaricalo come dice la GUIDA_MASTER, «Media più leggeri»." }
Write-Host "Media più leggeri (ffmpeg: $ff)" -ForegroundColor Cyan
Write-Host "Può richiedere parecchi minuti la prima volta (i video sono i più lenti). Non chiudere la finestra.`n"
& $exe -config $conf ottimizza-media -ffmpeg $ff 2>&1 | ForEach-Object { "$_" }
$codice = $LASTEXITCODE
# Il servizio di Gaster Bless deve poter leggere le copie leggere (di solito le eredita dalla cartella Media).
Write-Host ""
if ($codice -ne 0) { Ferma "Qualcosa non è andato: leggi i messaggi qui sopra." }
Write-Host "Fatto. I dispositivi ricevono già le copie leggere (al massimo dopo un'ora per i file già visti: Ctrl+F5 per vederle subito)." -ForegroundColor Green
Read-Host "`nPremi invio per chiudere"
