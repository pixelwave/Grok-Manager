# Install or update Grok Manager for Windows.
#   irm https://pixelwave.github.io/grok-manager/install.ps1 | iex
#   & ([scriptblock]::Create((irm https://pixelwave.github.io/grok-manager/install.ps1))) -p 6969
$ErrorActionPreference = "Stop"
if ($env:OS -ne "Windows_NT") {
  Write-Error "This installer supports Windows. On Linux or Mac, run: curl -fsSL https://pixelwave.github.io/grok-manager/install.sh | bash"
  exit 1
}
$port = 18090
if ($env:GROK_MANAGER_PORT) { $port = [int]$env:GROK_MANAGER_PORT }
for ($i = 0; $i -lt $args.Count; $i++) {
  if ($args[$i] -eq "-p" -and ($i + 1) -lt $args.Count) { $port = [int]$args[$i + 1] }
}
$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
  "AMD64" { "amd64" }
  "ARM64" { "arm64" }
  default { "" }
}
if (-not $arch) {
  Write-Error "unsupported architecture: $env:PROCESSOR_ARCHITECTURE"
  exit 1
}
$asset = "grok-manager_windows_$arch.exe"
$base = "https://github.com/pixelwave/Grok-Manager/releases/latest/download"
$tmp = Join-Path $env:TEMP ("grok-manager-download-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
  Invoke-WebRequest -UseBasicParsing -Uri "$base/$asset" -OutFile (Join-Path $tmp $asset)
  Invoke-WebRequest -UseBasicParsing -Uri "$base/SHA256SUMS" -OutFile (Join-Path $tmp "SHA256SUMS")
  $hash = (Get-FileHash -Algorithm SHA256 (Join-Path $tmp $asset)).Hash.ToLower()
  $wanted = ""
  foreach ($line in Get-Content (Join-Path $tmp "SHA256SUMS")) {
    $parts = $line -split "\s+"
    if ($parts.Length -ge 2 -and $parts[1] -eq $asset) { $wanted = $parts[0].ToLower() }
  }
  if (-not $wanted -or $wanted -ne $hash) { Write-Error "checksum did not match"; exit 1 }
  $destDir = Join-Path $env:LOCALAPPDATA "grok-manager"
  New-Item -ItemType Directory -Force -Path $destDir | Out-Null
  $dest = Join-Path $destDir "grok-manager.exe"
  Copy-Item (Join-Path $tmp $asset) $dest -Force
} finally {
  Remove-Item -Recurse -Force $tmp
}
& $dest install -p $port
Write-Host "Open a new terminal, then run: grok-manager status"
