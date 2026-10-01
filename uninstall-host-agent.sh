#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)

if [ "$(id -u)" -ne 0 ]; then
    echo "uninstall-host-agent.sh must be run as root" >&2
    exit 1
fi

echo "Removing AWG DocUI management components only. Running VPN interfaces will NOT be stopped or deleted."
# Refuse the stop if local hooks or dependencies could propagate it into a
# VPN unit. This is the same effective-unit preflight used by the installer.
sh "$SCRIPT_DIR/scripts/check-agent-unit.sh"
systemctl disable --now awg-docui-agent.service 2>/dev/null || true
rm -f /etc/systemd/system/awg-docui-agent.service
rm -f /etc/tmpfiles.d/awg-docui.conf
rm -f /usr/local/sbin/awg-docui-agent
rm -f /run/awg-docui/agent.sock
rm -f /etc/awg-docui/agent.env
rmdir /etc/awg-docui 2>/dev/null || true
systemctl daemon-reload

echo "Host agent removed."
echo "Preserved /etc/amnezia/amneziawg, its backups, every VPN systemd template, and VPN enablement state."
echo "Active interfaces and legacy amneziawg-ui management components were not changed."
