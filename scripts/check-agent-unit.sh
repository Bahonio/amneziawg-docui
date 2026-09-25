#!/bin/sh
set -eu

# Read the effective graph, including reverse dependencies. Merely checking
# the packaged unit misses local overrides and VPN units with PartOf=agent.
if ! unit_state=$(systemctl show awg-docui-agent.service \
    --property=LoadState --property=NeedDaemonReload --property=DropInPaths \
    --property=Requires --property=Wants --property=Requisite \
    --property=BindsTo --property=PartOf --property=ConsistsOf \
    --property=BoundBy --property=RequiredBy --property=RequisiteOf \
    --property=Upholds --property=UpheldBy \
    --property=Conflicts --property=ConflictedBy \
    --property=PropagatesStopTo --property=StopPropagatedFrom \
    --property=OnFailure --property=OnSuccess \
    --property=ExecStartPre --property=ExecStartPost \
    --property=ExecStop --property=ExecStopPost); then
    echo "FAIL: cannot inspect the agent systemd unit; refusing automatic restart" >&2
    exit 1
fi

if ! printf '%s\n' "$unit_state" | awk -F= '
    function reject(property) {
        print "FAIL: agent unit property " property " requires review before installation"
        failed = 1
    }
    {
        key = $1
        value = substr($0, length(key) + 2)
        if (key == "LoadState") {
            seen_load = 1
            if (value != "loaded" && value != "not-found") reject(key)
        } else if (key == "NeedDaemonReload") {
            if (value == "yes") reject(key)
        } else if (key == "Requires") {
            n = split(value, units, " ")
            for (i = 1; i <= n; i++) {
                if (units[i] != "sysinit.target" && units[i] !~ /\.(mount|slice)$/) reject(key)
            }
        } else if (key == "Wants") {
            n = split(value, units, " ")
            for (i = 1; i <= n; i++) {
                if (units[i] !~ /^(-|tmp|var|var-tmp)\.mount$/) reject(key)
            }
        } else if (key == "Conflicts" || key == "ConflictedBy") {
            if (value != "" && value != "shutdown.target") reject(key)
        } else if (value != "") {
            reject(key)
        }
    }
    END {
        if (!seen_load) reject("missing LoadState")
        exit failed
    }
' >&2; then
    echo "No agent restart was requested. Inspect local overrides and dependent units; do not remove VPN dependencies blindly." >&2
    exit 1
fi

echo "PASS: agent unit has no custom hooks, overrides or service dependencies that could affect VPN."
