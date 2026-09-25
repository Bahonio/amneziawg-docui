#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
UNINSTALLER=$SCRIPT_DIR/uninstall-host-agent.sh

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

lifecycle_calls=$(awk '
{
    line = $0
    if (match(line, /systemctl[[:space:]]+/)) {
        command = substr(line, RSTART)
        count = split(command, fields, /[[:space:]]+/)
        for (i = 2; i <= count; i++) {
            if (fields[i] ~ /^(start|stop|restart|try-restart|reload|reload-or-restart|enable|disable)$/) {
                print command
                break
            }
        }
    }
}
' "$UNINSTALLER")

expected='systemctl disable --now awg-docui-agent.service 2>/dev/null || true'
[ "$lifecycle_calls" = "$expected" ] \
    || fail "uninstaller may only stop and disable awg-docui-agent.service"

grep -Fq 'scripts/check-agent-unit.sh' "$UNINSTALLER" \
    || fail "uninstaller must inspect the effective agent unit before stopping it"
if grep -Eq '(^|[^A-Za-z0-9_-])awg-quick[[:space:]]+(up|down)([[:space:]]|$)' "$UNINSTALLER"; then
    fail "uninstaller contains a direct VPN lifecycle command"
fi

echo "PASS: uninstaller preflights dependencies and only disables/stops awg-docui-agent.service."
