#!/bin/sh
set -eu

if [ "${AWG_DOCUI_BOOTSTRAP_SANDBOX:-}" != yes ] \
   || [ ! -e /.dockerenv ] \
   || [ "$(id -u)" -ne 0 ]; then
    echo "Use scripts/test-bootstrap-sandbox.sh" >&2
    exit 1
fi

scenario=${AWG_DOCUI_BOOTSTRAP_SCENARIO:?}
mkdir -p /fixture/bin /run/systemd/system /etc/systemd/system \
    /etc/amnezia/amneziawg /opt/awg-docui
export PATH="/fixture/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

cat > /fixture/os-release <<'EOF'
ID=ubuntu
VERSION_ID="24.04"
VERSION_CODENAME=noble
UBUNTU_CODENAME=noble
EOF
printf '#!/bin/sh\nexit 0\n' > /fixture/agent
chmod 0755 /fixture/agent

cat > /fixture/bin/uname <<'SH'
#!/bin/sh
case "$1" in
    -m) echo x86_64 ;;
    -r) echo 6.8.0-sandbox ;;
    *) exit 1 ;;
esac
SH
cat > /fixture/bin/openssl <<'SH'
#!/bin/sh
case "$1" in
    rand) echo 0123456789abcdef0123456789abcdef0123 ;;
    dgst) cat >/dev/null; printf synthetic-digest ;;
    *) exit 1 ;;
esac
SH
cat > /fixture/bin/curl <<'SH'
#!/bin/sh
set -eu
output=""
previous=""
url=""
for argument in "$@"; do
    if [ "$previous" = -o ]; then output=$argument; fi
    case "$argument" in https://*) url=$argument ;; esac
    previous=$argument
done
if [ -n "$output" ]; then
    case "$url" in
        */SHA256SUMS) cp /fixture/release/SHA256SUMS "$output" ;;
        */awg-docui-install-bundle.tar.gz) cp /fixture/release/awg-docui-install-bundle.tar.gz "$output" ;;
        *) printf synthetic-download > "$output" ;;
    esac
fi
exit 0
SH
cat > /fixture/bin/dpkg <<'SH'
#!/bin/sh
[ "$*" = '--print-architecture' ] && { echo amd64; exit 0; }
exit 1
SH
cat > /fixture/bin/sysctl <<'SH'
#!/bin/sh
printf '%s\n' "$*" >> /fixture/sysctl.log
SH
cat > /fixture/bin/add-apt-repository <<'SH'
#!/bin/sh
printf '%s\n' "$*" >> /fixture/repositories.log
SH
cat > /fixture/bin/getent <<'SH'
#!/bin/sh
if [ "$*" = 'group awg-docui' ] && [ -f /fixture/group-created ]; then
    echo 'awg-docui:x:984:'
    exit 0
fi
exit 1
SH
cat > /fixture/bin/groupadd <<'SH'
#!/bin/sh
[ "$*" = '--system awg-docui' ] || exit 1
touch /fixture/group-created
SH

create_awg_commands() {
    cat > /fixture/bin/awg <<'SH'
#!/bin/sh
[ "$*" = 'show interfaces' ] || exit 1
if [ -f /fixture/existing ]; then echo awg0; fi
SH
    cat > /fixture/bin/awg-quick <<'SH'
#!/bin/sh
exit 1
SH
    cat > /fixture/bin/modinfo <<'SH'
#!/bin/sh
[ "$*" = amneziawg ]
SH
    cat > /fixture/bin/modprobe <<'SH'
#!/bin/sh
[ "$*" = amneziawg ] || exit 1
printf '%s\n' "$*" >> /fixture/modprobe.log
SH
    chmod 0755 /fixture/bin/awg /fixture/bin/awg-quick /fixture/bin/modinfo /fixture/bin/modprobe
}

create_docker_command() {
    cat > /fixture/bin/docker <<'SH'
#!/bin/sh
set -eu
printf '%s\n' "$*" >> /fixture/docker.log
case "$1" in
    pull) exit 0 ;;
    compose)
        shift
        case "$1" in
            version|up) exit 0 ;;
            ps) echo awg-docui; exit 0 ;;
            exec)
                [ "$*" = 'exec -T awg-docui /usr/local/bin/awg-docui healthcheck' ] || exit 1
                [ "${AWG_DOCUI_BOOTSTRAP_SCENARIO:-}" != unhealthy ]
                exit $? ;;
        esac
        ;;
esac
exit 1
SH
    chmod 0755 /fixture/bin/docker
}

cat > /fixture/bin/apt-get <<'SH'
#!/bin/sh
set -eu
printf '%s\n' "$*" >> /fixture/apt.log
case " $* " in
    *' amneziawg '*)
        cat > /fixture/bin/awg <<'EOF'
#!/bin/sh
[ "$*" = 'show interfaces' ]
EOF
        cat > /fixture/bin/awg-quick <<'EOF'
#!/bin/sh
exit 1
EOF
        cat > /fixture/bin/modinfo <<'EOF'
#!/bin/sh
[ "$*" = amneziawg ]
EOF
        cat > /fixture/bin/modprobe <<'EOF'
#!/bin/sh
[ "$*" = amneziawg ] || exit 1
printf '%s\n' "$*" >> /fixture/modprobe.log
EOF
        chmod 0755 /fixture/bin/awg /fixture/bin/awg-quick /fixture/bin/modinfo /fixture/bin/modprobe
        ;;
    *' docker-ce '*)
        cat > /fixture/bin/docker <<'EOF'
#!/bin/sh
set -eu
printf '%s\n' "$*" >> /fixture/docker.log
case "$1" in
    pull) exit 0 ;;
    compose)
        shift
        case "$1" in
            version|up) exit 0 ;;
            ps) echo awg-docui; exit 0 ;;
            exec)
                [ "$*" = 'exec -T awg-docui /usr/local/bin/awg-docui healthcheck' ] || exit 1
                [ "${AWG_DOCUI_BOOTSTRAP_SCENARIO:-}" != unhealthy ]
                exit $? ;;
        esac
        ;;
esac
exit 1
EOF
        chmod 0755 /fixture/bin/docker
        ;;
esac
SH

cat > /fixture/bin/systemd-tmpfiles <<'SH'
#!/bin/sh
[ "$*" = '--create /etc/tmpfiles.d/awg-docui.conf' ] || exit 1
grep -Fxq 'd /run/awg-docui 0750 root awg-docui -' /etc/tmpfiles.d/awg-docui.conf || exit 1
mkdir -p /run/awg-docui
SH
chmod 0755 /fixture/bin/systemd-tmpfiles

cat > /fixture/bin/systemctl <<'SH'
#!/bin/sh
set -eu
printf '%s\n' "$*" >> /fixture/systemctl.log
case "$1" in
    list-unit-files)
        if [ -f /fixture/existing ]; then printf 'amneziawg@.service enabled\n'; fi
        ;;
    show)
        printf 'LoadState=loaded\nNeedDaemonReload=no\nRequires=system.slice sysinit.target -.mount\nWants=tmp.mount\nDropInPaths=\nConflicts=shutdown.target\n'
        ;;
    cat)
        exit 1
        ;;
    daemon-reload)
        ;;
    enable)
        if [ "${2:-}" = --now ] && [ "${3:-}" = docker.service ]; then exit 0; fi
        [ "$#" -eq 2 ] && [ "$2" = awg-docui-agent.service ] || {
            touch /fixture/forbidden-systemctl
            exit 1
        }
        ;;
    restart)
        [ "$#" -eq 2 ] && [ "$2" = awg-docui-agent.service ] || {
            touch /fixture/forbidden-systemctl
            exit 1
        }
        ;;
    *) exit 1 ;;
esac
SH
chmod 0755 /fixture/bin/*

prepare_existing_host() {
    touch /fixture/existing /fixture/group-created
    create_awg_commands
    create_docker_command
    cat > /etc/amnezia/amneziawg/awg0.conf <<'EOF'
[Interface]
PrivateKey = SYNTHETIC-PRIVATE-KEY
ListenPort = 42000
EOF
    chmod 0600 /etc/amnezia/amneziawg/awg0.conf
    cat > /etc/systemd/system/amneziawg@.service <<'EOF'
[Service]
ExecStart=/usr/bin/awg-quick up /etc/amnezia/amneziawg/%i.conf
EOF
}

assert_no_host_package_changes() {
    [ ! -e /fixture/apt.log ] || { echo "FAIL: APT was used in $scenario mode" >&2; exit 1; }
    [ ! -e /fixture/repositories.log ] || { echo "FAIL: repositories changed in $scenario mode" >&2; exit 1; }
    [ ! -e /fixture/sysctl.log ] || { echo "FAIL: sysctl changed in $scenario mode" >&2; exit 1; }
    [ ! -e /fixture/modprobe.log ] || { echo "FAIL: module loaded in $scenario mode" >&2; exit 1; }
}

assert_license_install() {
    cmp /source/LICENSE /opt/awg-docui/LICENSE
    cmp /source/NOTICE /opt/awg-docui/NOTICE
    cmp /source/THIRD_PARTY_NOTICES.txt /opt/awg-docui/THIRD_PARTY_NOTICES.txt
    cmp /source/LICENSES/Apache-2.0.txt /opt/awg-docui/LICENSES/Apache-2.0.txt
    cmp /source/LICENSES/MPL-2.0.txt /opt/awg-docui/LICENSES/MPL-2.0.txt
}

assert_install_log() {
    log_file=/var/log/awg-docui/install.log
    [ -f "$log_file" ]
    [ "$(stat -c '%a' "$log_file")" = 600 ]
    for marker in '[1/7]' '[2/7]' '[3/7]' '[4/7]' '[5/7]' '[6/7]' '[7/7]'; do
        grep -Fq "$marker" "$log_file"
    done
    grep -Fq 'Installation completed. Full log:' "$log_file"
    if grep -Fq 'Web UI password:' "$log_file" \
       || grep -Fq '0123456789abcdef0123456789abcdef0123' "$log_file"; then
        echo 'FAIL: generated Web password leaked into installer log' >&2
        exit 1
    fi
}

case "$scenario" in
    fresh)
        AWG_DOCUI_SKIP_BUNDLE_DOWNLOAD=yes \
        AWG_DOCUI_OS_RELEASE_FILE=/fixture/os-release \
        sh /source/install.sh --fresh --agent-binary /fixture/agent > /fixture/output
        grep -Fq 'install -y amneziawg' /fixture/apt.log
        grep -Fq 'install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin' /fixture/apt.log
        grep -Fxq 'amneziawg' /fixture/modprobe.log
        grep -Fxq 'net.ipv4.ip_forward=1' /etc/sysctl.d/99-awg-docui-forwarding.conf
        grep -Fxq 'AWG_DOCUI_IMAGE=ghcr.io/bahonio/awg-docui:latest' /opt/awg-docui/.env
        assert_license_install
        assert_install_log
        grep -Fq 'Web UI password: 0123456789abcdef0123456789abcdef0123' /fixture/output
        echo 'PASS: fresh bootstrap installs official host packages, Docker, forwarding, agent and UI'
        ;;
    adopt)
        prepare_existing_host
        before_config=$(sha256sum /etc/amnezia/amneziawg/awg0.conf)
        before_unit=$(sha256sum /etc/systemd/system/amneziawg@.service)
        AWG_DOCUI_SKIP_BUNDLE_DOWNLOAD=yes \
        sh /source/install.sh --adopt --agent-binary /fixture/agent > /fixture/output
        [ "$before_config" = "$(sha256sum /etc/amnezia/amneziawg/awg0.conf)" ]
        [ "$before_unit" = "$(sha256sum /etc/systemd/system/amneziawg@.service)" ]
        assert_no_host_package_changes
        assert_license_install
        assert_install_log
        [ ! -e /fixture/forbidden-systemctl ]
        grep -Fq 'AWG DocUI installation mode: adopt' /fixture/output
        echo 'PASS: adopt bootstrap preserves packages, sysctl, module, VPN config and VPN unit'
        ;;
    update)
        prepare_existing_host
        cat > /opt/awg-docui/.env <<'EOF'
AWG_DOCUI_AGENT_GID=984
AWG_DOCUI_IMAGE=ghcr.io/bahonio/awg-docui:old
WEB_UI_USER=operator
WEB_UI_PASSWORD=preserved-hash
WEB_UI_PORT=54845
WEB_UI_BIND_ADDRESS=10.66.66.1
EOF
        AWG_DOCUI_SKIP_BUNDLE_DOWNLOAD=yes \
        sh /source/install.sh --update --version v0.2.0 \
            --agent-binary /fixture/agent > /fixture/output
        assert_no_host_package_changes
        assert_license_install
        assert_install_log
        grep -Fxq 'AWG_DOCUI_IMAGE=ghcr.io/bahonio/awg-docui:0.2.0' /opt/awg-docui/.env
        grep -Fxq 'WEB_UI_USER=operator' /opt/awg-docui/.env
        grep -Fxq 'WEB_UI_PASSWORD=preserved-hash' /opt/awg-docui/.env
        grep -Fxq 'WEB_UI_BIND_ADDRESS=10.66.66.1' /opt/awg-docui/.env
        ! grep -Fq 'Web UI password:' /fixture/output
        echo 'PASS: full update preserves credentials and settings while selecting the requested release'
        ;;
    bundle)
        prepare_existing_host
        mkdir -p /fixture/release/bundle/packaging /fixture/release/bundle/scripts /fixture/release/bundle/LICENSES
        cp /source/install.sh /source/install-host-agent.sh /source/uninstall-host-agent.sh \
            /source/docker-compose.yml /source/.env.example /source/LICENSE /source/NOTICE \
            /source/THIRD_PARTY_NOTICES.txt /fixture/release/bundle/
        cp /source/LICENSES/Apache-2.0.txt /fixture/release/bundle/LICENSES/
        cp /source/LICENSES/MPL-2.0.txt /fixture/release/bundle/LICENSES/
        cp /source/packaging/awg-docui-agent.service \
            /source/packaging/awg-docui-vpn@.service /source/packaging/awg-docui-tmpfiles.conf /fixture/release/bundle/packaging/
        cp /source/scripts/check-agent-unit.sh /fixture/release/bundle/scripts/
        printf 'v0.3.0\n' > /fixture/release/bundle/VERSION
        tar -C /fixture/release/bundle -czf /fixture/release/awg-docui-install-bundle.tar.gz .
        (cd /fixture/release && sha256sum awg-docui-install-bundle.tar.gz > SHA256SUMS)
        cp /source/install.sh /fixture/standalone-install.sh
        sh /fixture/standalone-install.sh --adopt \
            --agent-binary /fixture/agent > /fixture/output
        grep -Fxq 'v0.3.0' /opt/awg-docui/VERSION
        grep -Fxq 'AWG_DOCUI_IMAGE=ghcr.io/bahonio/awg-docui:latest' /opt/awg-docui/.env
        assert_license_install
        assert_no_host_package_changes
        assert_install_log
        echo 'PASS: standalone installer verifies and executes the published support bundle'
        ;;
    unhealthy)
        prepare_existing_host
        cat > /fixture/bin/sleep <<'SH'
#!/bin/sh
exit 0
SH
        chmod 0755 /fixture/bin/sleep
        if sh /source/install.sh --adopt --agent-binary /fixture/agent > /fixture/output 2>&1; then
            echo 'FAIL: installer accepted a running panel unable to reach the agent' >&2
            exit 1
        fi
        grep -Fq 'could not reach the host agent from inside the container' /fixture/output
        grep -Fq 'compose exec -T awg-docui /usr/local/bin/awg-docui healthcheck' /fixture/docker.log
        assert_no_host_package_changes
        echo 'PASS: installer rejects a running container with failed agent connectivity'
        ;;
    symlink)
        prepare_existing_host
        config_before=$(sha256sum /etc/amnezia/amneziawg/awg0.conf)
        ln -s /etc/amnezia/amneziawg/awg0.conf /opt/awg-docui/.env
        if AWG_DOCUI_SKIP_BUNDLE_DOWNLOAD=yes \
            sh /source/install.sh --adopt --agent-binary /fixture/agent \
            > /fixture/output 2>&1; then
            echo 'FAIL: deployment .env symlink was accepted' >&2
            exit 1
        fi
        [ "$config_before" = "$(sha256sum /etc/amnezia/amneziawg/awg0.conf)" ]
        [ ! -e /fixture/docker.log ]
        if [ -e /fixture/systemctl.log ] \
           && grep -Eq '(^| )(enable|restart|start|stop)( |$)' /fixture/systemctl.log; then
            echo 'FAIL: lifecycle command ran before symlink rejection' >&2
            exit 1
        fi
        grep -Fq 'Refusing deployment symlink' /fixture/output
        grep -Fq 'Installation failed with exit code' /fixture/output
        grep -Fq 'Refusing deployment symlink' /var/log/awg-docui/install.log
        grep -Fq 'Installation failed with exit code' /var/log/awg-docui/install.log
        echo 'PASS: deployment symlink is rejected before agent, container or VPN changes'
        ;;
    log-symlink)
        mkdir -p /fixture/log-target
        ln -s /fixture/log-target /var/log/awg-docui
        if AWG_DOCUI_SKIP_BUNDLE_DOWNLOAD=yes \
            sh /source/install.sh --adopt --agent-binary /fixture/agent \
            > /fixture/output 2>&1; then
            echo 'FAIL: symlinked installer log directory was accepted' >&2
            exit 1
        fi
        [ ! -e /fixture/log-target/install.log ]
        grep -Fq 'Refusing symlinked installer log path' /fixture/output
        [ ! -e /fixture/docker.log ]
        echo 'PASS: symlinked installer log path is rejected before host changes'
        ;;
    *) echo "Unknown scenario: $scenario" >&2; exit 1 ;;
esac
