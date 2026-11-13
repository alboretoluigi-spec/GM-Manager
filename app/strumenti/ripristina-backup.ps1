<#
  Ripristino di un backup di Gaster Bless sul PC del Master (doppio clic su RIPRISTINA_BACKUP.bat, da amministratore).
  1. mostra i backup disponibili (il più recente per primo) e ne fai scegliere uno;
  2. chiede il codice di ripristino (quello su carta) senza mostrarlo;
  3. ferma Gaster Bless, ripristina, riavvia Gaster Bless.
  Lo stato di prima non viene cancellato: finisce in C:\ProgramData\GasterBless\backup\prima-del-ripristino-...
  I contenuti (video, immagini, musica) non sono nel backup e non vengono toccati.
#>
$ErrorActionPreference = 'Continue'
function Ferma($t) { Write-Host "`n$t" -ForegroundColor Red; Read-Host "`nPremi invio per chiudere"; exit 1 }
if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Ferma "Servono i permessi di amministratore: usa RIPRISTINA_BACKUP.bat."
}
$exe  = Join-Path $env:ProgramFiles 'GasterBless\GasterBless.exe'
$conf = Join-Path $env:ProgramData 'GasterBless\gasterbless.conf'
$dir  = Join-Path $env:ProgramData 'GasterBless\backup'
if (-not (Test-Path $exe)) { Ferma "Gaster Bless non è installata in $exe." }
$file = @(Get-ChildItem -LiteralPath $dir -Filter 'gasterbless-*.bak' -File -ErrorAction SilentlyContinue | Sort-Object LastWriteTime -Descending)
if ($file.Count -eq 0) { Ferma "Nessun backup in $dir. Creane uno da Gaster Bless: Impostazioni > Backup." }

Write-Host "Ripristino di un backup di Gaster Bless" -ForegroundColor Cyan
Write-Host "Attenzione: i dati di Gaster Bless tornano com'erano al momento del backup (lo stato attuale viene messo da parte).`n"
$max = [Math]::Min(15, $file.Count)
for ($i = 0; $i -lt $max; $i++) {
    Write-Host ("  [{0}] {1}  ({2:dd/MM/yyyy HH:mm}, {3:N1} MB)" -f ($i + 1), $file[$i].Name, $file[$i].LastWriteTime, ($file[$i].Length / 1MB))
}
$scelta = Read-Host "`nNumero del backup da ripristinare (invio = 1, il più recente)"
if (-not $scelta) { $scelta = '1' }
$n = 0
if (-not [int]::TryParse($scelta, [ref]$n) -or $n -lt 1 -or $n -gt $max) { Ferma "Scelta non valida: nulla è cambiato." }
$bak = $file[$n - 1]

$sic = Read-Host "Codice di ripristino (quello scritto su carta; non compare mentre scrivi)" -AsSecureString
$b = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($sic)
try { $codice = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($b) } finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($b) }
if (-not $codice.Trim()) { Ferma "Codice vuoto: nulla è cambiato." }

$ok = Read-Host "Ripristino $($bak.Name): Gaster Bless verrà fermata per qualche secondo. Scrivi SI per procedere"
if ($ok -ne 'SI' -and $ok -ne 'si' -and $ok -ne 'Si') { Ferma "Annullato: nulla è cambiato." }

Write-Host "`nFermo Gaster Bless..."
Stop-Service Gaster Bless -ErrorAction SilentlyContinue
for ($i = 0; $i -lt 30 -and (Get-Service Gaster Bless -ErrorAction SilentlyContinue).Status -ne 'Stopped'; $i++) { Start-Sleep 1 }
Start-Sleep 2

$env:GASTERBLESS_CODICE_RIPRISTINO = $codice
try { $risultato = ((& $exe -config $conf ripristina $bak.FullName 2>&1) | ForEach-Object { "$_" }) -join "`r`n"; $esito = $LASTEXITCODE }
finally { Remove-Item Env:\GASTERBLESS_CODICE_RIPRISTINO -ErrorAction SilentlyContinue; $codice = $null }

Write-Host "Riavvio Gaster Bless..."
Start-Service Gaster Bless -ErrorAction SilentlyContinue
if ($esito -ne 0) { Ferma ($risultato.Trim() + "`n`nGasterBless è stata riavviata con i dati di prima (nulla è cambiato).") }
Write-Host "`n$($risultato.Trim())" -ForegroundColor Green
Write-Host "Gaster Bless è di nuovo accesa: apri l'app e controlla che i dati siano quelli del backup."
Read-Host "`nPremi invio per chiudere"
