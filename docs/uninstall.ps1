# Bootstrap for Windows. The downloaded uninstaller runs in memory so the
# default execution policy, which blocks .ps1 files, can stay as it is.
$ErrorActionPreference = "Stop"
if ($env:OS -ne "Windows_NT") {
  Write-Error "This uninstaller supports Windows. On Linux or Mac, run: curl -fsSL https://pixelwave.github.io/grok-manager/uninstall.sh | bash"
  exit 1
}
$response = Invoke-WebRequest -UseBasicParsing -Uri "https://github.com/pixelwave/Grok-Manager/releases/latest/download/uninstall.ps1"
& ([scriptblock]::Create($response.Content)) @args
