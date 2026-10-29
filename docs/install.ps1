# Bootstrap for Windows. Forwards -p when this file is invoked as a script.
$ErrorActionPreference = "Stop"
if ($env:OS -ne "Windows_NT") {
  Write-Error "This installer supports Windows. On Linux or Mac, run: curl -fsSL https://pixelwave.github.io/grok-manager/install.sh | bash"
  exit 1
}
$dest = Join-Path $env:TEMP ("grok-manager-install-" + [guid]::NewGuid() + ".ps1")
Invoke-WebRequest -UseBasicParsing -Uri "https://github.com/pixelwave/Grok-Manager/releases/latest/download/install.ps1" -OutFile $dest
try { & $dest @args } finally { Remove-Item -Force $dest }
