#!/bin/bash
# Installs the Dashgrid monitor as a systemd service
# Usage: sudo bash setup.sh

set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then echo "Error: run as root"; exit 1; fi

DIR=$(cd "$(dirname "$0")" && pwd)
BIN="dashgrid-monitor-linux-amd64"
[ -f "$DIR/config.yaml" ] || { echo "Error: config.yaml not found in $DIR"; exit 1; }
[ -f "$DIR/$BIN" ] || { echo "Error: $BIN binary not found in $DIR"; exit 1; }
chmod +x "$DIR/$BIN"
chmod 600 "$DIR/config.yaml"

echo ">>> Removing old cron job if present..."
rm -f /etc/cron.d/dashgrid-monitor

echo ">>> Creating systemd service..."
cat > /etc/systemd/system/dashgrid-monitor.service << EOF
[Unit]
Description=Dashgrid Linux Monitor
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=$DIR/$BIN
WorkingDirectory=$DIR
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

echo ">>> Starting service..."
systemctl daemon-reload
systemctl enable --now dashgrid-monitor.service

echo ">>> Done!"
echo "Logs:   sudo journalctl -u dashgrid-monitor -f"
echo "Config: $DIR/config.yaml"
