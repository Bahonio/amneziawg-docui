#!/bin/sh
set -eu

INTERFACE=${1:-wg0}
case "$INTERFACE" in ''|*[!A-Za-z0-9_.-]*|.*|-*) echo "Invalid interface" >&2; exit 1;; esac

check_interface() {
    ip link show dev "$INTERFACE" >/dev/null
    awg show "$INTERFACE" >/dev/null
}

check_interface
runtime_name=amneziawg'-go'
if pgrep -x "$runtime_name" >/dev/null; then
    echo "FAIL: unexpected non-kernel VPN runtime process" >&2
    exit 1
fi

docker stop awg-docui
check_interface
docker rm awg-docui
check_interface
systemctl stop awg-docui-agent.service
check_interface
systemctl start awg-docui-agent.service

echo "PASS: interface survived UI stop/removal and agent stop."
echo "Connect client 1 and verify internet + SSH. Add client 2 in UI and verify client 1 stays connected."
echo "Reboot gate: disable UI/container autostart, reboot, leave Docker stopped, then run:"
echo "  ip link show dev $INTERFACE"
echo "  awg show $INTERFACE"
echo "Verify client handshake and SSH, then start UI and confirm it discovers the running interface."
