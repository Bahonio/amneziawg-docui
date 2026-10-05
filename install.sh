#!/bin/sh
set -eu

PROJECT_REPOSITORY=${AWG_DOCUI_REPOSITORY:-Bahonio/amneziawg-docui}
INSTALL_DIR=${AWG_DOCUI_INSTALL_DIR:-/opt/awg-docui}
OS_RELEASE_FILE=${AWG_DOCUI_OS_RELEASE_FILE:-/etc/os-release}

mode=auto
action=install
requested_version=""
version_was_requested=no
agent_binary=""
image_override=""
pull_image=yes

usage() {
    cat <<'EOF'
Usage: install.sh [OPTIONS]

Install AWG DocUI on Ubuntu/Debian or attach it to an existing AmneziaWG host.

Options:
  --fresh                 Require a host without an existing AmneziaWG setup
  --adopt                 Preserve and use an existing AmneziaWG setup
  --update                Update AWG DocUI; do not update AmneziaWG packages
  --version VERSION       Install a release such as v0.1.0 (default: latest)
  --agent-binary PATH     Use a local host-agent binary instead of downloading one
  --image IMAGE           Override the AWG DocUI container image
  --no-pull               Use an image that is already present on the host
  -h, --help              Show this help

In auto mode, any existing module, tool, config, interface, or VPN unit selects
adopt mode. Package installation and sysctl changes are only allowed in fresh
mode. Official AmneziaWG updates remain managed by APT.

Numbered stages are printed to the terminal. Full output is appended to
/var/log/awg-docui/install.log with mode 0600; generated passwords are never
written to that log.
EOF
}

parse_args() {
    while [ "$#" -gt 0 ]; do
        case "$1" in
            --fresh)
                [ "$mode" = auto ] || { echo "Choose only one installation mode." >&2; exit 2; }
                mode=fresh
                shift
                ;;
            --adopt)
                [ "$mode" = auto ] || { echo "Choose only one installation mode." >&2; exit 2; }
                mode=adopt
                shift
                ;;
            --update)
                action=update
                shift
                ;;
            --version)
                [ "$#" -ge 2 ] || { echo "--version requires a value" >&2; exit 2; }
                requested_version=$2
                version_was_requested=yes
                shift 2
                ;;
            --agent-binary)
                [ "$#" -ge 2 ] || { echo "--agent-binary requires a path" >&2; exit 2; }
                agent_binary=$2
                shift 2
                ;;
            --image)
                [ "$#" -ge 2 ] || { echo "--image requires a value" >&2; exit 2; }
                image_override=$2
                shift 2
                ;;
            --no-pull)
                pull_image=no
                shift
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                echo "Unknown option: $1" >&2
                usage >&2
                exit 2
                ;;
        esac
    done
}

parse_args "$@"

if [ "$(id -u)" -ne 0 ]; then
    echo "install.sh must be run as root" >&2
    exit 1
fi
if [ "$action" = update ] && [ "$mode" = fresh ]; then
    echo "--update cannot be combined with --fresh; updates always preserve the existing host setup." >&2
    exit 2
fi

start_install_log() {
    [ "${AWG_DOCUI_LOG_ACTIVE:-no}" != yes ] || return 0
    # A script read from stdin has no path to re-execute. Its downloaded bundle
    # is a real file and starts logging before making host changes.
    [ -f "$0" ] || return 0

    log_dir=/var/log/awg-docui
    log_file=$log_dir/install.log
    if [ -L "$log_dir" ] || [ -L "$log_file" ]; then
        echo "Refusing symlinked installer log path: $log_file" >&2
        exit 1
    fi
    if [ ! -d "$log_dir" ]; then
        install -d -o root -g root -m 0700 "$log_dir"
    fi
    chown root:root "$log_dir"
    chmod 0700 "$log_dir"
    touch "$log_file"
    chown root:root "$log_file"
    chmod 0600 "$log_file"
    command -v tee >/dev/null 2>&1 || {
        echo "tee is required to record the installation log." >&2
        exit 1
    }

    status_file=$(mktemp /tmp/awg-docui-install-status.XXXXXX)
    stage_file=$(mktemp /tmp/awg-docui-install-stage.XXXXXX)
    chmod 0600 "$status_file" "$stage_file"
    trap 'rm -f "$status_file" "$stage_file"' EXIT HUP INT TERM

    # File descriptor 3 stays connected to the caller's terminal. The child
    # uses it only for the generated password, keeping that secret out of the
    # persistent support log while leaving it visible to the operator.
    exec 3>&1
    set +e
    {
        printf '\n=== AWG DocUI installer: %s ===\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
        AWG_DOCUI_LOG_ACTIVE=yes AWG_DOCUI_SECRET_FD=3 \
            AWG_DOCUI_STAGE_FILE="$stage_file" sh "$0" "$@"
        printf '%s\n' "$?" > "$status_file"
    } 2>&1 | tee -a "$log_file"
    install_status=$(sed -n '1p' "$status_file")
    failed_stage=$(sed -n '1p' "$stage_file")
    rm -f "$status_file" "$stage_file"
    trap - EXIT HUP INT TERM

    case "$install_status" in
        0)
            printf 'Installation completed. Full log: %s\n' "$log_file" | tee -a "$log_file"
            ;;
        ''|*[!0-9]*)
            install_status=1
            printf 'Installation failed. Full log: %s\n' "$log_file" | tee -a "$log_file" >&2
            ;;
        *)
            if [ -n "$failed_stage" ]; then
                printf 'Installation failed with exit code %s during %s. Full log: %s\n' \
                    "$install_status" "$failed_stage" "$log_file" | tee -a "$log_file" >&2
            else
                printf 'Installation failed with exit code %s before numbered stages. Full log: %s\n' \
                    "$install_status" "$log_file" | tee -a "$log_file" >&2
            fi
            ;;
    esac
    exit "$install_status"
}

stage() {
    stage_text="[$1/7] $2"
    printf '\n%s\n' "$stage_text"
    if [ -n "${AWG_DOCUI_STAGE_FILE:-}" ]; then
        printf '%s\n' "$stage_text" > "$AWG_DOCUI_STAGE_FILE"
    fi
}

start_install_log "$@"

validate_release_version() {
    if [ "$1" = latest ] || printf '%s\n' "$1" | grep -Eq '^v?[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$'; then
        return 0
    fi
    echo "Invalid release version: $1" >&2
    echo "Use latest or a semantic version such as v0.1.0." >&2
    exit 2
}

release_tag() {
    case "$1" in
        latest) printf '%s\n' latest ;;
        v*) printf '%s\n' "$1" ;;
        *) printf 'v%s\n' "$1" ;;
    esac
}

release_asset_base() {
    if [ "$1" = latest ]; then
        printf 'https://github.com/%s/releases/latest/download\n' "$PROJECT_REPOSITORY"
    else
        printf 'https://github.com/%s/releases/download/%s\n' "$PROJECT_REPOSITORY" "$(release_tag "$1")"
    fi
}

verify_download() {
    checksum_file=$1
    artifact=$2
    artifact_name=$(basename "$artifact")
    expected=$(awk -v name="$artifact_name" '$2 == name || $2 == "*" name { print $1; exit }' "$checksum_file")
    [ -n "$expected" ] || { echo "No checksum published for $artifact_name" >&2; exit 1; }
    actual=$(sha256sum "$artifact" | awk '{print $1}')
    [ "$actual" = "$expected" ] || { echo "Checksum mismatch for $artifact_name" >&2; exit 1; }
}

SCRIPT_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
support_complete=yes
for support_path in \
    install-host-agent.sh \
    uninstall-host-agent.sh \
    docker-compose.yml \
    .env.example \
    LICENSE \
    NOTICE \
    THIRD_PARTY_NOTICES.txt \
    LICENSES/Apache-2.0.txt \
    LICENSES/MPL-2.0.txt \
    packaging/awg-docui-agent.service \
    packaging/awg-docui-tmpfiles.conf \
    packaging/awg-docui-vpn@.service \
    scripts/check-agent-unit.sh \
    scripts/setup-debian-repositories.sh; do
    [ -f "$SCRIPT_DIR/$support_path" ] || support_complete=no
done

# The standalone release installer carries no mutable helper files. Download a
# checksummed release bundle and run its copy. Updates do the same so that an
# older installed bootstrap cannot leave Compose or host packaging behind.
if [ "${AWG_DOCUI_BUNDLE_READY:-no}" != yes ] \
   && { [ "$support_complete" != yes ] || [ "$action" = update ]; } \
   && [ "${AWG_DOCUI_SKIP_BUNDLE_DOWNLOAD:-no}" != yes ]; then
    for command_name in curl tar sha256sum awk; do
        command -v "$command_name" >/dev/null 2>&1 \
            || { echo "$command_name is required to download the release installer." >&2; exit 1; }
    done
    bundle_version=${requested_version:-latest}
    validate_release_version "$bundle_version"
    asset_base=$(release_asset_base "$bundle_version")
    bundle_tmp=$(mktemp -d /tmp/awg-docui-installer.XXXXXX)
    trap 'rm -rf "$bundle_tmp"' EXIT HUP INT TERM
    curl --proto '=https' --tlsv1.2 -fL "$asset_base/SHA256SUMS" -o "$bundle_tmp/SHA256SUMS"
    curl --proto '=https' --tlsv1.2 -fL "$asset_base/awg-docui-install-bundle.tar.gz" \
        -o "$bundle_tmp/awg-docui-install-bundle.tar.gz"
    verify_download "$bundle_tmp/SHA256SUMS" "$bundle_tmp/awg-docui-install-bundle.tar.gz"
    mkdir "$bundle_tmp/bundle"
    tar -xzf "$bundle_tmp/awg-docui-install-bundle.tar.gz" -C "$bundle_tmp/bundle"
    [ -f "$bundle_tmp/bundle/install.sh" ] \
        || { echo "Release installer bundle is incomplete." >&2; exit 1; }
    AWG_DOCUI_BUNDLE_READY=yes sh "$bundle_tmp/bundle/install.sh" "$@"
    exit $?
fi

[ "$support_complete" = yes ] || {
    echo "Installer support files are missing. Use a published release installer or a complete source checkout." >&2
    exit 1
}

stage 1 'Inspecting the host and selecting a safe installation mode'

if ! command -v systemctl >/dev/null 2>&1 || [ ! -d /run/systemd/system ]; then
    echo "A systemd-based Linux host is required." >&2
    exit 1
fi

# Refuse deployment symlinks before package, agent, or container changes. This
# prevents a privileged update from following an unexpected destination.
for deployment_destination in \
    "$INSTALL_DIR" \
    "$INSTALL_DIR/.env" \
    "$INSTALL_DIR/.env.example" \
    "$INSTALL_DIR/LICENSE" \
    "$INSTALL_DIR/NOTICE" \
    "$INSTALL_DIR/THIRD_PARTY_NOTICES.txt" \
    "$INSTALL_DIR/LICENSES" \
    "$INSTALL_DIR/VERSION" \
    "$INSTALL_DIR/docker-compose.yml" \
    "$INSTALL_DIR/install.sh" \
    "$INSTALL_DIR/install-host-agent.sh" \
    "$INSTALL_DIR/uninstall-host-agent.sh" \
    "$INSTALL_DIR/packaging" \
    "$INSTALL_DIR/scripts"; do
    if [ -L "$deployment_destination" ]; then
        echo "Refusing deployment symlink: $deployment_destination" >&2
        exit 1
    fi
done

case "$mode" in auto|fresh|adopt) ;; *) echo "Invalid installation mode: $mode" >&2; exit 2 ;; esac

existing_awg=no
[ -d /sys/module/amneziawg ] && existing_awg=yes
if command -v modinfo >/dev/null 2>&1 && modinfo amneziawg >/dev/null 2>&1; then
    existing_awg=yes
fi
command -v awg >/dev/null 2>&1 && existing_awg=yes
command -v awg-quick >/dev/null 2>&1 && existing_awg=yes
for config_path in /etc/amnezia/amneziawg/*.conf; do
    [ -f "$config_path" ] && existing_awg=yes
done
if systemctl list-unit-files --type=service --no-legend 2>/dev/null \
    | awk '$1 ~ /^(awg-quick@|amneziawg@|awg-docui-vpn@|amneziawg-ui-vpn@)/ { found=1 } END { exit !found }'; then
    existing_awg=yes
fi

if [ "$action" = update ]; then
    [ -d "$INSTALL_DIR" ] || { echo "AWG DocUI is not installed in $INSTALL_DIR." >&2; exit 1; }
    mode=adopt
elif [ "$mode" = auto ]; then
    if [ "$existing_awg" = yes ]; then mode=adopt; else mode=fresh; fi
elif [ "$mode" = fresh ] && [ "$existing_awg" = yes ]; then
    echo "An existing AmneziaWG installation was detected; refusing fresh mode." >&2
    echo "Run with --adopt so existing interfaces and configuration are preserved." >&2
    exit 1
fi

echo "AWG DocUI installation mode: $mode"

install_fresh_dependencies() {
    [ -r "$OS_RELEASE_FILE" ] || { echo "Cannot read $OS_RELEASE_FILE" >&2; exit 1; }
    # shellcheck disable=SC1090
    . "$OS_RELEASE_FILE"
    case "${ID:-}:${VERSION_ID:-}" in
        ubuntu:22.04) docker_distro=ubuntu; docker_suite=jammy ;;
        ubuntu:24.04) docker_distro=ubuntu; docker_suite=noble ;;
        debian:12) docker_distro=debian; docker_suite=bookworm ;;
        debian:13) docker_distro=debian; docker_suite=trixie ;;
        *)
            echo "Fresh installation supports Ubuntu 22.04/24.04 and Debian 12/13 only; detected ${ID:-unknown} ${VERSION_ID:-unknown}." >&2
            echo "Install the official AmneziaWG module, tools and Docker manually, then use --adopt." >&2
            exit 1
            ;;
    esac
    command -v apt-get >/dev/null 2>&1 || { echo "apt-get is required for fresh installation." >&2; exit 1; }

    export DEBIAN_FRONTEND=noninteractive
    apt-get update
    apt-get install -y ca-certificates curl gnupg iproute2 iptables kmod openssl linux-headers-"$(uname -r)"

    if [ "$docker_distro" = ubuntu ]; then
        apt-get install -y python3-launchpadlib software-properties-common
        # Two flags also add missing deb-src entries for enabled Ubuntu repos.
        add-apt-repository -y --enable-source --enable-source
        add-apt-repository -y --enable-source ppa:amnezia/ppa
    else
        sh "$SCRIPT_DIR/scripts/setup-debian-repositories.sh" "$docker_suite"
    fi
    apt-get update
    apt-get install -y amneziawg

    command -v modprobe >/dev/null 2>&1 || { echo "modprobe is unavailable after package installation." >&2; exit 1; }
    modprobe amneziawg
    if ! command -v modinfo >/dev/null 2>&1 || ! modinfo amneziawg >/dev/null 2>&1; then
        echo "The AmneziaWG kernel module was not installed for the running kernel." >&2
        exit 1
    fi
    command -v awg >/dev/null 2>&1 || { echo "The amneziawg package did not install awg." >&2; exit 1; }
    command -v awg-quick >/dev/null 2>&1 || { echo "The amneziawg package did not install awg-quick." >&2; exit 1; }

    install -d -m 0755 /etc/sysctl.d
    sysctl_tmp=$(mktemp /etc/sysctl.d/99-awg-docui-forwarding.conf.XXXXXX)
    printf '%s\n' 'net.ipv4.ip_forward=1' > "$sysctl_tmp"
    chmod 0644 "$sysctl_tmp"
    mv -f "$sysctl_tmp" /etc/sysctl.d/99-awg-docui-forwarding.conf
    sysctl -w net.ipv4.ip_forward=1 >/dev/null

    if ! command -v docker >/dev/null 2>&1; then
        install -m 0755 -d /etc/apt/keyrings
        docker_key_tmp=$(mktemp /tmp/docker-key.XXXXXX)
        curl --proto '=https' --tlsv1.2 -fsSL "https://download.docker.com/linux/$docker_distro/gpg" \
            -o "$docker_key_tmp"
        install -m 0644 "$docker_key_tmp" /etc/apt/keyrings/docker.asc
        rm -f "$docker_key_tmp"
        docker_arch=$(dpkg --print-architecture)
        cat > /etc/apt/sources.list.d/docker.sources <<EOF
Types: deb
URIs: https://download.docker.com/linux/$docker_distro
Suites: $docker_suite
Components: stable
Architectures: $docker_arch
Signed-By: /etc/apt/keyrings/docker.asc
EOF
        apt-get update
        apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
    elif ! docker compose version >/dev/null 2>&1; then
        echo "Docker is installed without Compose v2." >&2
        echo "Install a Compose v2 plugin compatible with the existing Docker installation, then rerun this installer." >&2
        exit 1
    fi
    systemctl enable --now docker.service
    docker compose version >/dev/null 2>&1 \
        || { echo "Docker Compose v2 is unavailable after installation." >&2; exit 1; }
}

verify_adopt_dependencies() {
    missing=""
    if ! { [ -d /sys/module/amneziawg ] || { command -v modinfo >/dev/null 2>&1 && modinfo amneziawg >/dev/null 2>&1; }; }; then
        missing="$missing amneziawg-kernel-module"
    fi
    command -v awg >/dev/null 2>&1 || missing="$missing awg"
    command -v awg-quick >/dev/null 2>&1 || missing="$missing awg-quick"
    if ! command -v docker >/dev/null 2>&1 || ! docker compose version >/dev/null 2>&1; then
        missing="$missing docker-compose-v2"
    fi
    command -v curl >/dev/null 2>&1 || missing="$missing curl"
    command -v openssl >/dev/null 2>&1 || missing="$missing openssl"
    if [ -n "$missing" ]; then
        echo "Existing-host mode never installs or changes host packages." >&2
        echo "Install the missing requirements manually and rerun:$missing" >&2
        exit 1
    fi
}

machine_arch=$(uname -m)
case "$machine_arch" in
    x86_64|amd64) release_arch=amd64 ;;
    aarch64|arm64) release_arch=arm64 ;;
    *) echo "Unsupported host architecture: $machine_arch" >&2; exit 1 ;;
esac

stage 2 'Checking or installing host dependencies'

if [ "$mode" = fresh ]; then
    install_fresh_dependencies
else
    verify_adopt_dependencies
fi

stage 3 'Preparing verified AWG DocUI release artifacts'

if [ -z "$requested_version" ] && [ -s "$SCRIPT_DIR/VERSION" ]; then
    requested_version=$(sed -n '1p' "$SCRIPT_DIR/VERSION")
fi
release_version=${requested_version:-latest}
validate_release_version "$release_version"
if [ "$version_was_requested" = yes ] && [ "$release_version" != latest ]; then
    image_tag=${release_version#v}
else
    image_tag=latest
fi
image_ref=${image_override:-ghcr.io/bahonio/awg-docui:$image_tag}
case "$image_ref" in *[!A-Za-z0-9._:/@-]*) echo "Invalid container image: $image_ref" >&2; exit 2 ;; esac

agent_tmp=""
cleanup_agent() {
    [ -z "$agent_tmp" ] || rm -rf "$agent_tmp"
}
trap cleanup_agent EXIT HUP INT TERM

if [ -n "$agent_binary" ]; then
    [ -f "$agent_binary" ] && [ ! -L "$agent_binary" ] \
        || { echo "Agent binary must be a regular, non-symlink file: $agent_binary" >&2; exit 1; }
    selected_agent=$agent_binary
elif [ -d "$SCRIPT_DIR/cmd/host-agent" ] && command -v go >/dev/null 2>&1; then
    selected_agent=""
else
    agent_tmp=$(mktemp -d /tmp/awg-docui-agent.XXXXXX)
    asset_base=$(release_asset_base "$release_version")
    agent_name=awg-docui-agent-linux-$release_arch
    curl --proto '=https' --tlsv1.2 -fL "$asset_base/SHA256SUMS" -o "$agent_tmp/SHA256SUMS"
    curl --proto '=https' --tlsv1.2 -fL "$asset_base/$agent_name" -o "$agent_tmp/$agent_name"
    verify_download "$agent_tmp/SHA256SUMS" "$agent_tmp/$agent_name"
    chmod 0755 "$agent_tmp/$agent_name"
    selected_agent=$agent_tmp/$agent_name
fi

# Pull before restarting our management agent. Pulling an image does not touch
# the running panel or any VPN interface.
if [ "$pull_image" = yes ]; then
    docker pull "$image_ref"
elif ! docker image inspect "$image_ref" >/dev/null 2>&1; then
    echo "The local image does not exist: $image_ref" >&2
    exit 1
fi

stage 4 'Installing the host management agent'

if [ -n "$selected_agent" ]; then
    sh "$SCRIPT_DIR/install-host-agent.sh" --binary "$selected_agent"
else
    sh "$SCRIPT_DIR/install-host-agent.sh"
fi

socket_gid=$(getent group awg-docui | cut -d: -f3)
[ -n "$socket_gid" ] || { echo "Cannot determine the awg-docui socket group GID." >&2; exit 1; }

stage 5 'Writing AWG DocUI management configuration'

install -d -m 0755 "$INSTALL_DIR" "$INSTALL_DIR/packaging" "$INSTALL_DIR/scripts" "$INSTALL_DIR/LICENSES"
install -m 0755 "$SCRIPT_DIR/install.sh" "$INSTALL_DIR/install.sh"
install -m 0755 "$SCRIPT_DIR/install-host-agent.sh" "$INSTALL_DIR/install-host-agent.sh"
install -m 0755 "$SCRIPT_DIR/uninstall-host-agent.sh" "$INSTALL_DIR/uninstall-host-agent.sh"
install -m 0755 "$SCRIPT_DIR/scripts/check-agent-unit.sh" "$INSTALL_DIR/scripts/check-agent-unit.sh"
install -m 0755 "$SCRIPT_DIR/scripts/setup-debian-repositories.sh" "$INSTALL_DIR/scripts/setup-debian-repositories.sh"
install -m 0644 "$SCRIPT_DIR/packaging/awg-docui-agent.service" "$INSTALL_DIR/packaging/awg-docui-agent.service"
install -m 0644 "$SCRIPT_DIR/packaging/awg-docui-tmpfiles.conf" "$INSTALL_DIR/packaging/awg-docui-tmpfiles.conf"
install -m 0644 "$SCRIPT_DIR/packaging/awg-docui-vpn@.service" "$INSTALL_DIR/packaging/awg-docui-vpn@.service"
install -m 0644 "$SCRIPT_DIR/docker-compose.yml" "$INSTALL_DIR/docker-compose.yml"
install -m 0600 "$SCRIPT_DIR/.env.example" "$INSTALL_DIR/.env.example"
install -m 0644 "$SCRIPT_DIR/LICENSE" "$INSTALL_DIR/LICENSE"
install -m 0644 "$SCRIPT_DIR/NOTICE" "$INSTALL_DIR/NOTICE"
install -m 0644 "$SCRIPT_DIR/THIRD_PARTY_NOTICES.txt" "$INSTALL_DIR/THIRD_PARTY_NOTICES.txt"
install -m 0644 "$SCRIPT_DIR/LICENSES/Apache-2.0.txt" "$INSTALL_DIR/LICENSES/Apache-2.0.txt"
install -m 0644 "$SCRIPT_DIR/LICENSES/MPL-2.0.txt" "$INSTALL_DIR/LICENSES/MPL-2.0.txt"
if [ -s "$SCRIPT_DIR/VERSION" ]; then
    install -m 0644 "$SCRIPT_DIR/VERSION" "$INSTALL_DIR/VERSION"
fi

set_env_value() {
    env_file=$1
    env_key=$2
    env_value=$3
    env_tmp=$(mktemp "$env_file.tmp.XXXXXX")
    awk -v key="$env_key" -v value="$env_value" '
        BEGIN { found=0 }
        index($0, key "=") == 1 {
            if (!found) print key "=" value
            found=1
            next
        }
        { print }
        END { if (!found) print key "=" value }
    ' "$env_file" > "$env_tmp"
    chmod 0600 "$env_tmp"
    mv -f "$env_tmp" "$env_file"
}

env_path=$INSTALL_DIR/.env
generated_password=""
if [ ! -f "$env_path" ]; then
    install -m 0600 "$SCRIPT_DIR/.env.example" "$env_path"
fi
set_env_value "$env_path" AWG_DOCUI_AGENT_GID "$socket_gid"
set_env_value "$env_path" AWG_DOCUI_IMAGE "$image_ref"

password_hash=$(sed -n 's/^WEB_UI_PASSWORD=//p' "$env_path" | sed -n '$p')
if [ -z "$password_hash" ]; then
    generated_password=$(openssl rand -hex 18)
    password_hash=$(printf '%s' "$generated_password" | openssl dgst -sha256 -binary | base64)
    set_env_value "$env_path" WEB_UI_PASSWORD "$password_hash"
fi

stage 6 'Starting the Web UI container'

(cd "$INSTALL_DIR" && docker compose up -d --remove-orphans)

stage 7 'Running management health checks'

panel_health=no
health_attempt=0
while [ "$health_attempt" -lt 30 ]; do
    if (cd "$INSTALL_DIR" && docker compose exec -T awg-docui /usr/local/bin/awg-docui healthcheck); then
        panel_health=yes
        break
    fi
    health_attempt=$((health_attempt + 1))
    sleep 1
done
[ "$panel_health" = yes ] || { echo "AWG DocUI could not reach the host agent from inside the container." >&2; exit 1; }

echo
echo "AWG DocUI is installed in $INSTALL_DIR"
echo "Container image: $image_ref"
echo "Panel URL on the server: http://127.0.0.1:54845"
echo "Open it locally with: ssh -L 54845:127.0.0.1:54845 root@YOUR_SERVER"
echo "Web UI user: $(sed -n 's/^WEB_UI_USER=//p' "$env_path" | sed -n '$p')"
if [ -n "$generated_password" ]; then
    if [ "${AWG_DOCUI_SECRET_FD:-}" = 3 ]; then
        printf 'Web UI password: %s\n' "$generated_password" >&3
    else
        echo "Web UI password: $generated_password"
    fi
    echo "Save this password now; only its SHA-256 digest is stored."
fi
echo
echo "AmneziaWG module and tools are updated separately by the host APT configuration."
echo "Update only the Web UI: cd $INSTALL_DIR && docker compose pull && docker compose up -d"
echo "Update the complete AWG DocUI management plane: sudo $INSTALL_DIR/install.sh --update"
