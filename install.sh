#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
install -m 644 "$ROOT/nginx/grok-manager.conf" /etc/nginx/sites-available/grok-manager
ln -sfn /etc/nginx/sites-available/grok-manager /etc/nginx/sites-enabled/grok-manager

cat > /etc/systemd/system/grok-manager.service << EOF
[Unit]
Description=Grok Manager session portal
After=network.target

[Service]
Type=simple
WorkingDirectory=$ROOT
ExecStart=/usr/bin/python3 $ROOT/server.py
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now grok-manager.service
nginx -t
systemctl reload nginx
echo "Grok Manager is listening on port 8090"
