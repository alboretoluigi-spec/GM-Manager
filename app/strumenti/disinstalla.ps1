<#
  Disinstallazione di Gaster Bless (nuova app). Va eseguito come amministratore.
      powershell -ExecutionPolicy Bypass -File .\disinstalla.ps1
  Toglie servizio, programma, icone e regole del firewall. I DATI (C:\ProgramData\GasterBless: account,
  schede, backup, e in Media video, immagini, musica e manuali) NON vengono cancellati, a meno di
  usare -CancellaDati.
#>
param([switch]$CancellaDati)
$ErrorActionPreference = 'Continue'
$prog = Join-Path $env:ProgramFiles 'GasterBless'
$dati = Join-Path $env:ProgramData 'GasterBless'
if (Get-Service Gaster Bless -ErrorAction SilentlyContinue) { Stop-Service Gaster Bless -Force; & sc.exe delete Gaster Bless | Out-Host }
Get-NetFirewallRule -DisplayName 'Gaster Bless*' -ErrorAction SilentlyContinue | Remove-NetFirewallRule
Get-ChildItem Cert:\LocalMachine\Root | Where-Object { $_.Subject -match 'GasterBless' } | Remove-Item
Remove-Item (Join-Path ([Environment]::GetFolderPath('CommonDesktopDirectory')) 'GasterBless.lnk') -ErrorAction SilentlyContinue
Remove-Item (Join-Path $env:ProgramData 'Microsoft\Windows\Start Menu\Programs\GasterBless') -Recurse -ErrorAction SilentlyContinue
Remove-Item $prog -Recurse -Force -ErrorAction SilentlyContinue
if ($CancellaDati) {
    $r = Read-Host "Scrivi CANCELLA per eliminare anche account, schede, backup e contenuti (video, immagini, musica) in $dati"
    if ($r -ceq 'CANCELLA') { Remove-Item $dati -Recurse -Force; Write-Host "Dati eliminati." } else { Write-Host "Dati conservati." }
} else { Write-Host "Dati conservati in $dati (account, schede, backup)." }
Write-Host "Gaster Bless è stato disinstallato."
