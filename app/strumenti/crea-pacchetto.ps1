<#
  Crea un pacchetto per installare Gaster Bless su un altro PC (come il "📦 Crea pacchetto" della vecchia app).
      powershell -ExecutionPolicy Bypass -File .\crea-pacchetto.ps1 [-ConManuali] [-Destinazione D:\]
  Il pacchetto contiene GasterBless.exe, gli script di installazione, la configurazione d'esempio, le guide e
  una copia dei contenuti di GasterBlessDati (Video, Immagini, Musica, Canti; i Manuali solo con -ConManuali).
  NON contiene account, schede, background né backup: per spostare anche quelli usa un backup e il codice
  di ripristino (vedi README).
#>
param([switch]$ConManuali, [string]$Destinazione = (Join-Path ([Environment]::GetFolderPath('MyDocuments')) 'GasterBless\Pacchetti'),
      [string]$CartellaMedia = (Join-Path ([Environment]::GetFolderPath('MyDocuments')) 'GasterBless\GasterBlessDati'))
$ErrorActionPreference = 'Stop'
$radice = Split-Path $PSScriptRoot -Parent
$dest = Join-Path $Destinazione ("Gaster Bless-pacchetto-" + (Get-Date -Format 'yyyy-MM-dd'))
New-Item -ItemType Directory -Force -Path $dest | Out-Null
$exe = @((Join-Path $radice 'GasterBless.exe'), (Join-Path $env:ProgramFiles 'GasterBless\GasterBless.exe')) | Where-Object { Test-Path $_ } | Select-Object -First 1
if (-not $exe) { Write-Host "GasterBless.exe non trovato." -ForegroundColor Red; exit 1 }
Copy-Item $exe $dest
Copy-Item (Join-Path $PSScriptRoot '*') $dest -Exclude 'crea-pacchetto.ps1'
foreach ($f in 'gasterbless.conf.esempio','README.md') { $p = Join-Path $radice $f; if (Test-Path $p) { Copy-Item $p $dest } }
$g = Join-Path $radice 'guide'; if (Test-Path $g) { Copy-Item $g $dest -Recurse }
$cont = Join-Path $dest 'Contenuti\GasterBlessDati'
foreach ($sub in 'Immagini','Video','Musica') {
    $p = Join-Path $CartellaMedia $sub
    if (Test-Path $p) { Write-Host "Copia di $sub…"; & robocopy $p (Join-Path $cont $sub) /E /NFL /NDL /NJH /NJS | Out-Null }
}
if ($ConManuali) { $p = Join-Path $CartellaMedia 'Manuali'; if (Test-Path $p) { & robocopy $p (Join-Path $cont 'Manuali') /E /NFL /NDL /NJH /NJS | Out-Null } }
Write-Host "Pacchetto pronto: $dest" -ForegroundColor Green
Write-Host "Sul PC nuovo: copia la cartella Contenuti\GasterBlessDati in Documenti\GasterBless\, poi esegui installa.ps1 come amministratore."
