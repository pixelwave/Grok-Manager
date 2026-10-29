# Remove Grok Manager on Windows. Config stays unless --purge is set.
$ErrorActionPreference = "Stop"
if ($env:OS -ne "Windows_NT") {
  Write-Error "This uninstaller supports Windows. On Linux or Mac, run: curl -fsSL https://pixelwave.github.io/grok-manager/uninstall.sh | bash"
  exit 1
}
$dest = Join-Path $env:LOCALAPPDATA "grok-manager\grok-manager.exe"
if (-not (Test-Path $dest)) {
  Write-Error "Grok Manager is not installed."
  exit 1
}
if ($args -contains "--purge") { & $dest uninstall --purge } else { & $dest uninstall }
