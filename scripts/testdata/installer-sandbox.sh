#!/bin/sh
set -eu

# This fixture writes synthetic host files at production paths. Never run it
# directly on a host; the outer script creates the isolated container.
if [ "${AWG_DOCUI_INSTALLER_SANDBOX:-}" != yes ] || [ ! -e /.dockerenv ] || [ "$(id -u)" -ne 0 ]; then
    echo "Use scripts/test-installer-sandbox.sh" >&2
    exit 1
fi

mkdir -p /fixture/bin /run/systemd/system /etc/amnezia/amneziawg \
    /etc/awg-docui /etc/amneziawg-ui /etc/systemd/system /usr/local/sbin
export PATH="/fixture/bin:$PATH"

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
    show)
        if [ "${AWG_DOCUI_FAULT:-}" = systemd ]; then exit 1; fi
        if [ "${AWG_DOCUI_FAULT:-}" = after-reload ] && [ -f /fixture/reloaded ]; then
            printf 'LoadState=loaded\nConsistsOf=amneziawg@awg0.service\n'
        else
            cat /fixture/unit-state
        fi
        ;;
    list-unit-files)
        printf 'amneziawg@.service enabled\nawg-quick@.service disabled\n'
        ;;
    cat)
        if [ "${AWG_DOCUI_VENDOR:-}" = quick ]; then
            printf '[Service]\nExecStart=/usr/bin/awg-quick up %%i\n'
        else
            exit 1
        fi
        ;;
    daemon-reload)
        touch /fixture/reloaded
        ;;
    enable|restart)
        if [ "$#" -ne 2 ] || [ "$2" != awg-docui-agent.service ]; then
            echo "FORBIDDEN systemctl action" >> /fixture/forbidden
            exit 1
        fi
        printf '%s\n' "$*" >> /fixture/lifecycle.log
        ;;
    *) echo "FORBIDDEN systemctl action" >> /fixture/forbidden; exit 1 ;;
esac
SH
cat > /fixture/bin/awg <<'SH'
#!/bin/sh
if [ "$*" != 'show interfaces' ]; then touch /fixture/forbidden; exit 1; fi
if [ "${AWG_DOCUI_FAULT:-}" = awg ]; then exit 1; fi
echo awg0
SH
cat > /fixture/bin/awg-quick <<'SH'
#!/bin/sh
# /etc/amnezia/amneziawg is the official tool lookup directory in this fixture.
touch /fixture/forbidden
exit 1
SH
cat > /fixture/bin/go <<'SH'
#!/bin/sh
set -eu
if [ "${AWG_DOCUI_FAULT:-}" = build ]; then exit 1; fi
while [ "$#" -gt 0 ]; do
    if [ "$1" = -o ]; then
        printf '#!/bin/sh\nexit 0\n' > "$2"
        exit 0
    fi
    shift
done
exit 1
SH
cat > /fixture/bin/getent <<'SH'
#!/bin/sh
if [ "$*" != 'group awg-docui' ]; then exit 1; fi
echo 'awg-docui:x:984:'
SH
cat > /fixture/bin/modinfo <<'SH'
#!/bin/sh
[ "$*" = amneziawg ]
SH
for command in ip iptables ip6tables nft modprobe insmod apt apt-get dkms groupadd; do
    cat > "/fixture/bin/$command" <<'SH'
#!/bin/sh
touch /fixture/forbidden
exit 1
SH
done
chmod +x /fixture/bin/*

printf '[Interface]\nPrivateKey = SYNTHETIC-TEST-KEY\nListenPort = 42000\n' > /etc/amnezia/amneziawg/awg0.conf
chmod 0600 /etc/amnezia/amneziawg/awg0.conf
printf '[Service]\nExecStart=/usr/bin/awg-quick up /etc/amnezia/amneziawg/%%i.conf\n' > /etc/systemd/system/amneziawg@.service
# Preserve customized existing templates too, rather than just identical copies.
printf '# Existing customized VPN template; preserve this exact file.\n' > /etc/systemd/system/amneziawg-ui-vpn@.service
printf 'AWGUI_VPN_UNIT=amneziawg-ui-vpn@%%s.service\n' > /etc/amneziawg-ui/agent.env
# Include the implicit PrivateTmp dependency reported by a real systemd host.
printf 'LoadState=loaded\nNeedDaemonReload=no\nRequires=system.slice sysinit.target -.mount\nWants=tmp.mount\nDropInPaths=\nConflicts=shutdown.target\n' > /fixture/unit-state

vpn_snapshot() {
    for file in /etc/amnezia/amneziawg/awg0.conf /etc/systemd/system/amneziawg@.service /etc/systemd/system/amneziawg-ui-vpn@.service; do
        sha256sum "$file"
        stat -c '%n %a %Y %i' "$file"
    done
}
management_snapshot() {
    find /etc/awg-docui /etc/amneziawg-ui /usr/local/sbin -type f -exec sha256sum '{}' + | sort
    if [ -f /etc/systemd/system/awg-docui-agent.service ]; then
        sha256sum /etc/systemd/system/awg-docui-agent.service
    fi
}
vpn_before=$(vpn_snapshot)
assert_vpn_preserved() {
    [ "$vpn_before" = "$(vpn_snapshot)" ] || { echo "FAIL: existing VPN files changed" >&2; exit 1; }
    [ ! -e /fixture/forbidden ] || { echo "FAIL: forbidden host command" >&2; exit 1; }
}
clear_calls() {
    rm -f /fixture/reloaded /fixture/lifecycle.log /fixture/systemctl.log
}

before=$(management_snapshot)
sh /source/install-host-agent.sh --check > /fixture/output
[ "$before" = "$(management_snapshot)" ]
[ ! -f /fixture/lifecycle.log ] && [ ! -f /fixture/reloaded ]
assert_vpn_preserved
echo 'PASS: --check writes no management or VPN files and requests no reload/restart'

for scenario in fallback quick; do
    clear_calls
    if [ "$scenario" = quick ]; then rm -f /etc/awg-docui/agent.env /etc/amneziawg-ui/agent.env; fi
    AWG_DOCUI_VENDOR=$scenario sh /source/install-host-agent.sh > /fixture/output
    expected='enable awg-docui-agent.service
restart awg-docui-agent.service'
    [ "$(cat /fixture/lifecycle.log)" = "$expected" ]
    assert_vpn_preserved
    echo "PASS: $scenario template selection preserves VPN files; only the agent is enabled/restarted"
done

clear_calls
printf '#!/bin/sh\nexit 0\n' > /fixture/release-agent
chmod 0755 /fixture/release-agent
AWG_DOCUI_FAULT=build sh /source/install-host-agent.sh --binary /fixture/release-agent > /fixture/output
expected='enable awg-docui-agent.service
restart awg-docui-agent.service'
[ "$(cat /fixture/lifecycle.log)" = "$expected" ]
assert_vpn_preserved
echo 'PASS: verified release binary installation does not invoke the Go toolchain or change VPN state'

for fault in build awg systemd; do
    clear_calls
    before=$(management_snapshot)
    if AWG_DOCUI_FAULT=$fault sh /source/install-host-agent.sh > /fixture/output 2>&1; then
        echo "FAIL: $fault was accepted" >&2; exit 1
    fi
    [ "$before" = "$(management_snapshot)" ]
    [ ! -f /fixture/lifecycle.log ] && [ ! -f /fixture/reloaded ]
    assert_vpn_preserved
    echo "PASS: $fault failure aborts before installed files or services change"
done

for property in ConsistsOf BoundBy RequiredBy Wants Conflicts ExecStop DropInPaths; do
    clear_calls
    printf 'LoadState=loaded\n%s=amneziawg@awg0.service\n' "$property" > /fixture/unit-state
    before=$(management_snapshot)
    if sh /source/install-host-agent.sh > /fixture/output 2>&1; then
        echo "FAIL: unsafe $property was accepted" >&2; exit 1
    fi
    [ "$before" = "$(management_snapshot)" ]
    [ ! -f /fixture/lifecycle.log ] && [ ! -f /fixture/reloaded ]
    assert_vpn_preserved
    echo "PASS: unsafe $property aborts installation before changes"
done
printf 'LoadState=loaded\n' > /fixture/unit-state

clear_calls
if AWG_DOCUI_FAULT=after-reload sh /source/install-host-agent.sh > /fixture/output 2>&1; then
    echo 'FAIL: dependency introduced during install was accepted' >&2; exit 1
fi
[ ! -f /fixture/lifecycle.log ]
assert_vpn_preserved
echo 'PASS: dependency appearing after daemon-reload prevents agent enable/restart'

clear_calls
rm -f /etc/awg-docui/agent.env /etc/amneziawg-ui/agent.env
ln -s /etc/amnezia/amneziawg/awg0.conf /etc/awg-docui/agent.env
if sh /source/install-host-agent.sh > /fixture/output 2>&1; then
    echo 'FAIL: symlink management destination was accepted' >&2; exit 1
fi
[ ! -f /fixture/lifecycle.log ] && [ ! -f /fixture/reloaded ]
assert_vpn_preserved
echo 'PASS: management symlink cannot overwrite an existing VPN config'
