#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
CONFIG_DIR=/etc/amnezia/amneziawg
AGENT_DIR=/etc/awg-docui
AGENT_BIN=/usr/local/sbin/awg-docui-agent
AGENT_UNIT=/etc/systemd/system/awg-docui-agent.service
FALLBACK_UNIT=/etc/systemd/system/awg-docui-vpn@.service
TMPFILES_CONF=/etc/tmpfiles.d/awg-docui.conf
SOCKET_GROUP=awg-docui
LEGACY_AGENT_DIR=/etc/amneziawg-ui

check_only=no
binary_path=""
while [ "$#" -gt 0 ]; do
    case "$1" in
        --check) check_only=yes; shift ;;
        --binary)
            [ "$#" -ge 2 ] || { echo "--binary requires a path" >&2; exit 1; }
            binary_path=$2; shift 2 ;;
        --help|-h)
            echo "Usage: install-host-agent.sh [--check] [--binary PATH]"
            exit 0 ;;
        *) echo "Usage: install-host-agent.sh [--check] [--binary PATH]" >&2; exit 1 ;;
    esac
done

if [ "$(id -u)" -ne 0 ]; then
    echo "install-host-agent.sh must be run as root" >&2
    exit 1
fi
if ! command -v systemctl >/dev/null 2>&1 || [ ! -d /run/systemd/system ]; then
    echo "systemd is required on the host" >&2
    exit 1
fi
if [ -n "$binary_path" ] && { [ ! -f "$binary_path" ] || [ -L "$binary_path" ]; }; then
    echo "Host-agent binary must be a regular, non-symlink file: $binary_path" >&2
    exit 1
fi

awg_ok=no
quick_ok=no
module_installed=no
module_loaded=no
ipv4_forwarding=unknown
command -v awg >/dev/null 2>&1 && awg_ok=yes
command -v awg-quick >/dev/null 2>&1 && quick_ok=yes
if [ -d /sys/module/amneziawg ]; then
    module_installed=yes
    module_loaded=yes
elif command -v modinfo >/dev/null 2>&1 && modinfo amneziawg >/dev/null 2>&1; then
    module_installed=yes
fi
if [ -r /proc/sys/net/ipv4/ip_forward ]; then
    ipv4_forwarding=$(cat /proc/sys/net/ipv4/ip_forward)
fi

configs=""
if [ -d "$CONFIG_DIR" ]; then
    for path in "$CONFIG_DIR"/*.conf; do
        [ -f "$path" ] || continue
        configs="${configs}${configs:+, }$(basename "$path" .conf)"
    done
fi
interfaces=""
if [ "$awg_ok" = yes ]; then
    if ! interfaces=$(awg show interfaces); then
        echo "Cannot inspect existing AmneziaWG interfaces; installation aborted." >&2
        exit 1
    fi
    interfaces=$(printf '%s' "$interfaces" | tr ' ' ',')
fi
if ! unit_files=$(systemctl list-unit-files --type=service --no-legend); then
    echo "Cannot inspect existing systemd units; installation aborted." >&2
    exit 1
fi
units=$(printf '%s\n' "$unit_files" \
    | awk '/(^awg-quick@|^amneziawg@|^awg-docui-vpn@|^amneziawg-ui-vpn@)/ {print $1}' \
    | paste -sd, -)

if [ "$module_installed" = yes ] || [ "$awg_ok" = yes ] || [ "$quick_ok" = yes ] || [ -n "$configs" ] || [ -n "$interfaces" ] || [ -n "$units" ]; then
    echo "Existing AmneziaWG installation detected."
    echo "It will NOT be reinstalled or modified."
    if [ "$check_only" = yes ]; then
        echo "Checking management installation only; no changes will be made."
    else
        echo "Installing management components only."
    fi
    echo "Existing interfaces: ${interfaces:-none active}"
    echo "Existing configs: ${configs:-none found}"
    echo "Existing systemd templates: ${units:-none found}"
fi
echo "Kernel module installed: $module_installed"
echo "Kernel module loaded: $module_loaded"
echo "awg: $awg_ok"
echo "awg-quick: $quick_ok"
echo "IPv4 forwarding: $ipv4_forwarding"

# Never follow a management destination symlink into an existing VPN file.
for destination in "$AGENT_DIR" "$AGENT_BIN" "$AGENT_UNIT" "$AGENT_DIR/agent.env" "$TMPFILES_CONF" /run/awg-docui; do
    if [ -L "$destination" ]; then
        echo "Refusing to replace symlink management destination: $destination" >&2
        exit 1
    fi
done
sh "$SCRIPT_DIR/scripts/check-agent-unit.sh"

if [ "$check_only" = yes ]; then
    echo "PASS: read-only installation checks completed. No files, services or VPN interfaces were changed."
    exit 0
fi

if [ -z "$binary_path" ] && ! command -v go >/dev/null 2>&1; then
    echo "Go is required to build the host agent from this source checkout." >&2
    echo "Alternatively pass a verified release binary with --binary PATH." >&2
    exit 1
fi

# Finish binary preparation before changing any installed management component.
build_tmp=$(mktemp /tmp/awg-docui-agent.XXXXXX)
trap 'rm -f "$build_tmp"' EXIT
trap 'exit 1' HUP INT TERM
if [ -n "$binary_path" ]; then
    install -m 0755 "$binary_path" "$build_tmp"
else
    (cd "$SCRIPT_DIR" && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$build_tmp" ./cmd/host-agent)
fi

if ! getent group "$SOCKET_GROUP" >/dev/null 2>&1; then
    groupadd --system "$SOCKET_GROUP"
fi
socket_gid=$(getent group "$SOCKET_GROUP" | cut -d: -f3)

if [ ! -d "$CONFIG_DIR" ]; then
    install -d -m 0700 "$CONFIG_DIR"
fi
install -d -m 0755 "$AGENT_DIR"
install -m 0755 "$build_tmp" "$AGENT_BIN"
install -m 0644 "$SCRIPT_DIR/packaging/awg-docui-agent.service" "$AGENT_UNIT"
install -d -m 0755 /etc/tmpfiles.d
install -m 0644 "$SCRIPT_DIR/packaging/awg-docui-tmpfiles.conf" "$TMPFILES_CONF"
systemd-tmpfiles --create "$TMPFILES_CONF"

vpn_unit='awg-docui-vpn@%s.service'
previous_vpn_unit=""
for env_file in "$AGENT_DIR/agent.env" "$LEGACY_AGENT_DIR/agent.env"; do
    [ -f "$env_file" ] || continue
    candidate=$(sed -n -e 's/^AWG_DOCUI_VPN_UNIT=//p' -e 's/^AWGUI_VPN_UNIT=//p' "$env_file" | sed -n '$p')
    case "$candidate" in
        'awg-quick@%s.service'|'amneziawg@%s.service'|'awg-docui-vpn@%s.service'|'amneziawg-ui-vpn@%s.service')
            previous_vpn_unit=$candidate
            break
            ;;
    esac
done

official_quick_compatible=no
quick_path=$(command -v awg-quick 2>/dev/null || true)
if systemctl cat awg-quick@.service >/dev/null 2>&1 \
   && systemctl cat awg-quick@.service | grep -Eq 'awg-quick[[:space:]]+up[[:space:]]+%i'; then
    if systemctl cat awg-quick@.service | grep -Fq "$CONFIG_DIR/%i.conf" \
       || { [ -n "$quick_path" ] && grep -Fq "$CONFIG_DIR" "$quick_path" 2>/dev/null; }; then
        official_quick_compatible=yes
    fi
fi

install_fallback_template() {
    if [ -e "$FALLBACK_UNIT" ] || [ -L "$FALLBACK_UNIT" ]; then
        echo "Preserving existing awg-docui-vpn@.service without changes."
    else
        install -m 0644 "$SCRIPT_DIR/packaging/awg-docui-vpn@.service" "$FALLBACK_UNIT"
        echo "Installed isolated awg-docui-vpn@.service; no VPN instance will be started."
    fi
}

if [ -n "$previous_vpn_unit" ]; then
    vpn_unit=$previous_vpn_unit
    case "$vpn_unit" in
        'awg-docui-vpn@%s.service') install_fallback_template ;;
        'amneziawg-ui-vpn@%s.service')
            if [ ! -f /etc/systemd/system/amneziawg-ui-vpn@.service ]; then
                echo "Legacy VPN template selected but missing; installation aborted." >&2
                exit 1
            fi
            ;;
    esac
    echo "Preserving the previously configured VPN systemd template: $vpn_unit"
elif [ "$official_quick_compatible" = yes ]; then
    vpn_unit='awg-quick@%s.service'
    echo "Using the installed awg-quick@.service template."
elif [ -f /etc/systemd/system/amneziawg@.service ] \
   && grep -Eq 'awg-quick[[:space:]]+up[[:space:]]+/etc/amnezia/amneziawg/%i\.conf' /etc/systemd/system/amneziawg@.service; then
    vpn_unit='amneziawg@%s.service'
    echo "Using existing compatible amneziawg@.service without modifying it."
else
    install_fallback_template
fi

umask 077
env_tmp=$(mktemp "$AGENT_DIR/agent.env.XXXXXX")
{
    echo "AWG_DOCUI_CONFIG_DIR=$CONFIG_DIR"
    echo "AWG_DOCUI_AGENT_SOCKET=/run/awg-docui/agent.sock"
    echo "AWG_DOCUI_SOCKET_MODE=0660"
    echo "AWG_DOCUI_SOCKET_GROUP=$SOCKET_GROUP"
    echo "AWG_DOCUI_VPN_UNIT=$vpn_unit"
} > "$env_tmp"
chmod 0600 "$env_tmp"
mv -f "$env_tmp" "$AGENT_DIR/agent.env"

# Safety invariant: installation may reload systemd metadata, but it must never
# start, stop, restart, enable or disable a VPN template instance. Only this
# management agent is enabled and restarted.
systemctl daemon-reload
sh "$SCRIPT_DIR/scripts/check-agent-unit.sh"
systemctl enable awg-docui-agent.service
systemctl restart awg-docui-agent.service

echo "Host management agent installed."
echo "Unix socket: /run/awg-docui/agent.sock"
echo "Set AWG_DOCUI_AGENT_GID=$socket_gid in the Web UI .env file."
if [ -f /etc/systemd/system/amneziawg-ui-agent.service ]; then
    echo "Legacy amneziawg-ui-agent.service was detected and was not changed."
    echo "Verify AWG DocUI first, then remove the legacy management agent separately if desired."
fi
if [ "$module_installed" != yes ]; then
    echo "AmneziaWG kernel module is not available on the host." >&2
    echo "Install the official AmneziaWG host package (Debian/Ubuntu: apt install amneziawg), then load the module." >&2
elif [ "$module_loaded" != yes ]; then
    echo "AmneziaWG kernel module not loaded. Load the official module before starting an interface." >&2
fi
if [ "$awg_ok" != yes ] || [ "$quick_ok" != yes ]; then
    echo "awg-tools not installed. Install the official package that provides awg and awg-quick." >&2
fi
if [ "$ipv4_forwarding" != 1 ]; then
    echo "IPv4 forwarding is not enabled; full-tunnel clients will not reach routed networks." >&2
    echo "Enable net.ipv4.ip_forward persistently according to your Linux distribution policy." >&2
fi
