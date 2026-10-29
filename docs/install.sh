#!/bin/bash
# Bootstrap. The release script picks the binary for this machine.
set -euo pipefail
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
curl -fsSL "https://github.com/pixelwave/Grok-Manager/releases/latest/download/install.sh" -o "$tmp"
bash "$tmp" "$@"
