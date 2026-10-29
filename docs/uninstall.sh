#!/bin/bash
set -euo pipefail
tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
curl -fsSL "https://github.com/pixelwave/Grok-Manager/releases/latest/download/uninstall.sh" -o "$tmp"
bash "$tmp" "$@"
