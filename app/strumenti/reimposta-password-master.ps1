<#
  Nuova password per il Master di Gaster Bless, quando quella vecchia è persa.
  Funziona solo sul PC su cui gira Gaster Bless e solo da amministratore (doppio clic su
  REIMPOSTA_PASSWORD_MASTER.bat). Gaster Bless può restare accesa. Tutti i dispositivi del Master
  (PC, portatile, telefono) dovranno rientrare con la nuova password. Dati e backup non cambiano.
#>
# 'Continue': in PowerShell 5.1 con 'Stop' i messaggi di GasterBless.exe su stderr fermerebbero lo script.
$ErrorActionPreference = 'Continue'
function Ferma($t) { Write-Host "`n$t" -ForegroundColor Red; Read-Host "`nPremi invio per chiudere"; exit 1 }
if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Ferma "Servono i permessi di amministratore: usa REIMPOSTA_PASSWORD_MASTER.bat."
}
$exe  = Join-Path $env:ProgramFiles 'GasterBless\GasterBless.exe'
$conf = Join-Path $env:ProgramData 'GasterBless\gasterbless.conf'
if (-not (Test-Path $exe))  { Ferma "Gaster Bless non è installata in $exe." }
if (-not (Test-Path $conf)) { Ferma "Configurazione non trovata: $conf." }
$aiuto = (& $exe -h 2>&1 | Out-String)
if ($aiuto -notmatch 'reimposta-master') { Ferma "La Gaster Bless installata è vecchia: prima aggiornala con strumenti\aggiorna.ps1." }

function Testo($sicuro) {
    $b = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($sicuro)
    try { return [Runtime.InteropServices.Marshal]::PtrToStringBSTR($b) } finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($b) }
}
Write-Host "Nuova password del Master di Gaster Bless" -ForegroundColor Cyan
Write-Host "Almeno 10 caratteri. Consiglio: tre o quattro parole con un trattino, per esempio Corvo-Lago-Nebbia-7."
Write-Host "Mentre scrivi non compare nulla: è normale.`n"
$p1 = Testo (Read-Host "Nuova password" -AsSecureString)
$p2 = Testo (Read-Host "Ripetila" -AsSecureString)
if ($p1 -ne $p2) { Ferma "Le due password non coincidono: nulla è cambiato." }
if (-not $p1) { Ferma "Password vuota: nulla è cambiato." }

if ($p1.Length -lt 10) { Ferma "La password deve avere almeno 10 caratteri (ne hai scritti $($p1.Length)): nulla è cambiato." }
# La password passa a GasterBless.exe in una variabile visibile solo a questo processo (non con una pipe:
# PowerShell 5.1 ci aggiungerebbe un carattere invisibile).
$env:GASTERBLESS_NUOVA_PASSWORD = $p1
try { $risultato = ((& $exe -config $conf reimposta-master 2>&1) | ForEach-Object { "$_" }) -join "`r`n"; $codice = $LASTEXITCODE }
finally { Remove-Item Env:\GASTERBLESS_NUOVA_PASSWORD -ErrorAction SilentlyContinue; $p1 = $null; $p2 = $null }
Write-Host ""
if ($codice -ne 0) { Ferma $risultato.Trim() }
Write-Host $risultato.Trim() -ForegroundColor Green
Write-Host "`nSalvala nel gestore delle password del browser quando te lo propone, e tienine una copia su carta"
Write-Host "insieme al codice di ripristino."
Read-Host "`nPremi invio per chiudere"
