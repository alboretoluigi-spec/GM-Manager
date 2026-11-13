<#
  Aggiornamento di Gaster Bless sul PC del Master, in un solo comando. Va eseguito come amministratore:
      powershell -ExecutionPolicy Bypass -File C:\Progetti\GM-Manager\app\strumenti\aggiorna.ps1
  Cosa fa: scarica gli aggiornamenti da GitHub (git pull), ricrea GasterBless.exe con Go e lo reinstalla con
  installa.ps1 (ferma Gaster Bless, sostituisce il programma, la riavvia). Dati e contenuti restano dove sono.
#>
$ErrorActionPreference = 'Stop'
function Ferma($t) { Write-Host "`n$t" -ForegroundColor Red; Write-Host "Aggiornamento interrotto: la Gaster Bless installata non è stata toccata." -ForegroundColor Red; exit 1 }
if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Host "Avvia PowerShell come amministratore (tasto destro -> Esegui come amministratore) e rilancia." -ForegroundColor Red
    exit 1
}
# Go portatile del progetto (vedi STATO_LAVORO.md, "Ambiente di sviluppo").
$strumenti = 'C:\Progetti\Strumenti'
$env:Path = (Join-Path $strumenti 'go\bin') + ';' + $env:Path
$env:GOPATH = Join-Path $strumenti 'gopath'
$env:GOCACHE = Join-Path $strumenti 'gocache'
$env:GOTOOLCHAIN = 'local'
if (-not (Get-Command go -ErrorAction SilentlyContinue)) { Ferma "Go non trovato in $strumenti\go." }
$app = Split-Path $PSScriptRoot -Parent
Set-Location $app

Write-Host "`n== 1. Scarico gli aggiornamenti" -ForegroundColor Cyan
& git pull
if ($LASTEXITCODE -ne 0) { Ferma "git pull non riuscito (codice $LASTEXITCODE)." }

Write-Host "`n== 2. Ricreo il programma" -ForegroundColor Cyan
& go build -o GasterBless.exe ./cmd/gasterbless
if ($LASTEXITCODE -ne 0) { Ferma "Compilazione non riuscita." }
Write-Host "GasterBless.exe pronto."

Write-Host "`n== 3. Installo la nuova versione" -ForegroundColor Cyan
& (Join-Path $PSScriptRoot 'installa.ps1')
