#!/bin/bash
# Remove Grok Manager. Config stays unless --purge is set.
#   curl -fsSL https://pixelwave.github.io/grok-manager/uninstall.sh | bash
#   curl -fsSL https://pixelwave.github.io/grok-manager/uninstall.sh | bash -s -- --purge
set -euo pipefail
if [[ "$(uname -s)" != "Linux" && "$(uname -s)" != "Darwin" ]]; then
  echo "This uninstaller supports Linux and Mac. On Windows, run: irm https://pixelwave.github.io/grok-manager/uninstall.ps1 | iex" >&2
  exit 1
fi
if [[ "$(id -u)" -eq 0 ]]; then
  bin="/usr/local/bin/grok-manager"
else
  bin="${HOME}/.local/bin/grok-manager"
fi
if [[ ! -x "$bin" ]]; then
  echo "Grok Manager is not installed." >&2
  exit 1
fi
if [[ "${1:-}" == "--purge" ]]; then
  "$bin" uninstall --purge
else
  "$bin" uninstall
fi
