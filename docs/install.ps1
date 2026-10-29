# Bootstrap for Windows. The downloaded installer runs in memory so the
# default execution policy, which blocks .ps1 files, can stay as it is.
$ErrorActionPreference = "Stop"
if ($env:OS -ne "Windows_NT") {
  Write-Error "This installer supports Windows. On Linux or Mac, run: curl -fsSL https://pixelwave.github.io/grok-manager/install.sh | bash"
  exit 1
}
$response = Invoke-WebRequest -UseBasicParsing -Uri "https://github.com/pixelwave/Grok-Manager/releases/latest/download/install.ps1"
$text = $response.Content
if ($text -is [byte[]]) { $text = [System.Text.Encoding]::UTF8.GetString($text) }
& ([scriptblock]::Create([string]$text)) @args
