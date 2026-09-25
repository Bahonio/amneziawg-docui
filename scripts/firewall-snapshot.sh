#!/bin/sh
set -eu

if [ "$#" -ne 2 ]; then
    echo "Usage: firewall-snapshot.sh iptables-save|ip6tables-save OUTPUT" >&2
    exit 1
fi
case "$1" in
    iptables-save|ip6tables-save) ;;
    *) echo "Unsupported firewall snapshot command" >&2; exit 1 ;;
esac

umask 077
# Save separately: a pipeline can hide a failed dump on POSIX sh. Keep the raw
# output for diagnosis; never restore it or modify the running firewall.
if ! "$1" > "$2.raw"; then
    echo "FAIL: $1 could not read firewall rules" >&2
    exit 1
fi

# iptables-save includes timestamps and chain counters even without -c.
# Remove only volatile metadata, retaining policies, rule order and literal
# text inside rules (including --comment strings containing # or [N:N]).
awk '
    /^#/ { next }
    /^:[^[:space:]]+[[:space:]]+[^[:space:]]+[[:space:]]+\[[0-9]+:[0-9]+\]$/ {
        sub(/\[[0-9]+:[0-9]+\]$/, "[0:0]")
    }
    { print }
' "$2.raw" > "$2"
