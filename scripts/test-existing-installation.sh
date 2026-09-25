#!/bin/sh
set -eu

INTERFACE=${INTERFACE:-wg0}
SCRIPT_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
CONFIG=/etc/amnezia/amneziawg/$INTERFACE.conf

if [ "$(id -u)" -ne 0 ]; then
    echo "Run this regression check as root" >&2
    exit 1
fi
case "$INTERFACE" in ''|*[!A-Za-z0-9_.-]*|.*|-*) echo "Invalid INTERFACE" >&2; exit 1;; esac
if [ ! -f "$CONFIG" ]; then echo "Missing $CONFIG" >&2; exit 1; fi
if ! ip link show dev "$INTERFACE" >/dev/null 2>&1; then echo "$INTERFACE is not running" >&2; exit 1; fi

umask 077
check_dir=$(mktemp -d /tmp/awg-docui-install-check.XXXXXX)
cleanup() {
    check_status=$?
    trap - EXIT HUP INT TERM
    if [ "$check_status" -eq 0 ]; then
        rm -f "$check_dir"/*
        rmdir "$check_dir"
    else
        echo "Diagnostic firewall snapshots retained in: $check_dir" >&2
    fi
    exit "$check_status"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

configured_template=""
for env_file in /etc/awg-docui/agent.env /etc/amneziawg-ui/agent.env; do
    [ -f "$env_file" ] || continue
    configured_template=$(sed -n -e 's/^AWG_DOCUI_VPN_UNIT=//p' -e 's/^AWGUI_VPN_UNIT=//p' "$env_file" | sed -n '$p')
    [ -n "$configured_template" ] && break
done

vpn_unit=""
for template in "$configured_template" \
    'awg-quick@%s.service' \
    'amneziawg@%s.service' \
    'awg-docui-vpn@%s.service' \
    'amneziawg-ui-vpn@%s.service'; do
    case "$template" in
        'awg-quick@%s.service') candidate="awg-quick@$INTERFACE.service" ;;
        'amneziawg@%s.service') candidate="amneziawg@$INTERFACE.service" ;;
        'awg-docui-vpn@%s.service') candidate="awg-docui-vpn@$INTERFACE.service" ;;
        'amneziawg-ui-vpn@%s.service') candidate="amneziawg-ui-vpn@$INTERFACE.service" ;;
        *) continue ;;
    esac
    if systemctl is-active --quiet "$candidate"; then
        vpn_unit=$candidate
        break
    fi
done

before_hash=$(sha256sum "$CONFIG" | awk '{print $1}')
before_identity=$(awg show "$INTERFACE" dump | awk 'NR==1 {print $2, $3}')
before_peers=$(awg show "$INTERFACE" dump | awk 'NR>1 {print $1, $4}' | sort)
before_ifindex=$(cat "/sys/class/net/$INTERFACE/ifindex")
before_unit_state=""
if [ -n "$vpn_unit" ]; then
    before_unit_state=$(systemctl show "$vpn_unit" \
        --property=InvocationID \
        --property=ActiveEnterTimestampMonotonic)
fi
firewall_tools=""
for tool in iptables-save ip6tables-save; do
    if command -v "$tool" >/dev/null 2>&1; then
        sh "$SCRIPT_DIR/scripts/firewall-snapshot.sh" "$tool" "$check_dir/$tool.before"
        firewall_tools="${firewall_tools}${firewall_tools:+ }$tool"
    else
        echo "SKIP: $tool is unavailable; its rules will not be checked."
    fi
done

"$SCRIPT_DIR/install-host-agent.sh"

after_hash=$(sha256sum "$CONFIG" | awk '{print $1}')
after_identity=$(awg show "$INTERFACE" dump | awk 'NR==1 {print $2, $3}')
after_peers=$(awg show "$INTERFACE" dump | awk 'NR>1 {print $1, $4}' | sort)
after_ifindex=$(cat "/sys/class/net/$INTERFACE/ifindex")
after_unit_state=""
if [ -n "$vpn_unit" ]; then
    systemctl is-active --quiet "$vpn_unit" || { echo "FAIL: $vpn_unit is no longer active" >&2; exit 1; }
    after_unit_state=$(systemctl show "$vpn_unit" \
        --property=InvocationID \
        --property=ActiveEnterTimestampMonotonic)
fi
for tool in $firewall_tools; do
    sh "$SCRIPT_DIR/scripts/firewall-snapshot.sh" "$tool" "$check_dir/$tool.after"
done

[ "$before_hash" = "$after_hash" ] || { echo "FAIL: config hash changed" >&2; exit 1; }
[ "$before_identity" = "$after_identity" ] || { echo "FAIL: key or listen port changed" >&2; exit 1; }
[ "$before_peers" = "$after_peers" ] || { echo "FAIL: peers changed" >&2; exit 1; }
[ "$before_ifindex" = "$after_ifindex" ] || { echo "FAIL: interface was recreated" >&2; exit 1; }
if [ -n "$vpn_unit" ] && [ "$before_unit_state" != "$after_unit_state" ]; then
    echo "FAIL: VPN systemd unit $vpn_unit was restarted" >&2
    exit 1
fi
firewall_changed=no
for tool in $firewall_tools; do
    if diff -u "$check_dir/$tool.before" "$check_dir/$tool.after" > "$check_dir/$tool.diff"; then
        echo "PASS: $tool rules and policies unchanged (timestamps and counters excluded)."
    else
        echo "FAIL: $tool rules differ, or comparison failed; see $check_dir/$tool.diff" >&2
        firewall_changed=yes
    fi
done
if [ "$firewall_changed" = yes ]; then
    echo "A rules difference does not identify its source; other host services may also update the firewall." >&2
    exit 1
fi
ip link show dev "$INTERFACE" >/dev/null
awg show "$INTERFACE" latest-handshakes

if [ -n "$vpn_unit" ]; then
    echo "PASS: VPN unit $vpn_unit kept the same invocation and activation timestamp."
else
    echo "INFO: no active known VPN systemd unit found; interface lifetime was checked by ifindex."
fi
echo "PASS: config hash, identity, port, peers and interface lifetime were preserved."
echo "Confirm that the parallel SSH-over-VPN session remained connected."
