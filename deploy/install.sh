#!/bin/bash
# Install or update Grok Manager for Linux and Mac.
#   curl -fsSL https://pixelwave.github.io/grok-manager/install.sh | bash
#   curl -fsSL https://pixelwave.github.io/grok-manager/install.sh | bash -s -- -p 6969
set -euo pipefail

asset_for() {
  local os="$1" arch="$2"
  case "$os" in
    Linux)
      case "$arch" in
        x86_64) echo grok-manager_linux_amd64 ;;
        aarch64|arm64) echo grok-manager_linux_arm64 ;;
        *) echo "unsupported architecture: $arch" >&2; return 1 ;;
      esac
      ;;
    Darwin)
      case "$arch" in
        x86_64) echo grok-manager_darwin_amd64 ;;
        arm64) echo grok-manager_darwin_arm64 ;;
        *) echo "unsupported architecture: $arch" >&2; return 1 ;;
      esac
      ;;
    *)
      echo "This installer supports Linux and Mac. On Windows, run: irm https://pixelwave.github.io/grok-manager/install.ps1 | iex" >&2
      return 1
      ;;
  esac
}

if [[ "${1:-}" == "--print-asset" ]]; then
  asset_for "$2" "$3"
  exit
fi

port="${GROK_MANAGER_PORT:-18090}"
while [[ $# -gt 0 ]]; do
  case "$1" in
    -p)
      port="$2"
      shift 2
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

asset="$(asset_for "$(uname -s)" "$(uname -m)")"
base="https://github.com/pixelwave/Grok-Manager/releases/latest/download"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
curl -fsSL "$base/$asset" -o "$tmp/$asset"
curl -fsSL "$base/SHA256SUMS" -o "$tmp/SHA256SUMS"
(
  cd "$tmp"
  grep -F " $asset" SHA256SUMS | sha256sum -c -
)
if [[ "$(id -u)" -eq 0 ]]; then
  dest="/usr/local/bin/grok-manager"
else
  mkdir -p "${HOME}/.local/bin"
  dest="${HOME}/.local/bin/grok-manager"
fi
install -m 0755 "$tmp/$asset" "$dest"
"$dest" install -p "$port"
