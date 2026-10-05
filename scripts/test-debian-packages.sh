#!/bin/sh
set -eu
SCRIPT_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
test_suite=${1:-}
test_arch=${2:-}
case "$test_suite" in
    ''|bookworm|trixie) ;;
    *) echo 'Usage: test-debian-packages.sh [bookworm|trixie] [amd64|arm64]' >&2; exit 2 ;;
esac
if [ -z "$test_arch" ]; then
    case "$(uname -m)" in
        x86_64|amd64) test_arch=amd64 ;;
        aarch64|arm64) test_arch=arm64 ;;
        *) echo 'Unsupported test architecture.' >&2; exit 2 ;;
    esac
fi
case "$test_arch" in amd64|arm64) ;; *) echo 'Expected amd64 or arm64.' >&2; exit 2 ;; esac

for suite in ${test_suite:-bookworm trixie}; do
    # These containers may download and build packages, but cannot load a host
    # kernel module. Only the source checkout is mounted, read-only.
    docker run --rm --platform "linux/$test_arch" \
        --security-opt no-new-privileges:true \
        --mount "type=bind,src=$SCRIPT_DIR,target=/source,readonly" \
        --env AWG_DOCUI_DEBIAN_PACKAGE_SANDBOX=yes \
        "debian:$suite-slim" sh /source/scripts/testdata/debian-packages.sh
done
