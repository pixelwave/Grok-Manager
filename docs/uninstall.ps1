$ErrorActionPreference = "Stop"
if ($env:OS -ne "Windows_NT") {
  Write-Error "This uninstaller supports Windows. On Linux or Mac, run: curl -fsSL https://pixelwave.github.io/grok-manager/uninstall.sh | bash"
  exit 1
}
$dest = Join-Path $env:TEMP ("grok-manager-uninstall-" + [guid]::NewGuid() + ".ps1")
Invoke-WebRequest -UseBasicParsing -Uri "https://github.com/pixelwave/Grok-Manager/releases/latest/download/uninstall.ps1" -OutFile $dest
try { & $dest @args } finally { Remove-Item -Force $dest }
