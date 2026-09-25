#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
INSTALLER=$SCRIPT_DIR/install-host-agent.sh

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

# Extract every systemctl lifecycle operation even when it appears inside an
# assignment or conditional. Read-only inspection and daemon-reload are not
# lifecycle operations. Any new lifecycle call must pass this explicit allowlist.
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
' "$INSTALLER")

expected_calls='systemctl enable awg-docui-agent.service
systemctl restart awg-docui-agent.service'

if [ "$lifecycle_calls" != "$expected_calls" ]; then
    echo "Expected installer lifecycle calls:" >&2
    printf '%s\n' "$expected_calls" >&2
    echo "Actual installer lifecycle calls:" >&2
    printf '%s\n' "${lifecycle_calls:-<none>}" >&2
    fail "installer may only enable and restart the management agent"
fi

# A direct runtime command in the installer would bypass the systemd allowlist.
if grep -Eq '(^|[^A-Za-z0-9_-])awg-quick[[:space:]]+(up|down)([[:space:]]|$)' "$INSTALLER"; then
    fail "installer contains a direct awg-quick up/down command"
fi
if grep -Eq '(^|[^A-Za-z0-9_-])awg[[:space:]]+(set|syncconf)([[:space:]]|$)' "$INSTALLER"; then
    fail "installer contains a direct live VPN mutation"
fi

grep -Fxq 'systemctl daemon-reload' "$INSTALLER" \
    || fail "installer must reload systemd metadata after installing the agent unit"

echo "PASS: installer only enables/restarts awg-docui-agent.service."
echo "PASS: installer has no VPN lifecycle or direct VPN mutation commands."
