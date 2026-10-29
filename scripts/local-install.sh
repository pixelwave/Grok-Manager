#!/bin/bash
# Build this checkout and restart the local service. Does not publish a release.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
version="$(tr -d '[:space:]' < VERSION)"
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
CGO_ENABLED=0 go build -ldflags "-X main.Version=${version}" -o "$tmp" ./cmd/grok-manager
if [[ "$(id -u)" -eq 0 ]]; then
  dest="/usr/local/bin/grok-manager"
  if systemctl is-active --quiet grok-manager.service; then
    install -m 0755 "$tmp" "$dest"
    systemctl restart grok-manager.service
  else
    install -m 0755 "$tmp" "$dest"
    "$dest" install -p "${GROK_MANAGER_PORT:-18090}"
  fi
else
  mkdir -p "${HOME}/.local/bin"
  dest="${HOME}/.local/bin/grok-manager"
  install -m 0755 "$tmp" "$dest"
  if systemctl --user is-active --quiet grok-manager.service; then
    systemctl --user restart grok-manager.service
  else
    "$dest" install -p "${GROK_MANAGER_PORT:-18090}"
  fi
fi
echo "Grok Manager ${version} is running from this checkout."
