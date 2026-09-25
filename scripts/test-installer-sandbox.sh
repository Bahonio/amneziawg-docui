#!/bin/sh
set -eu
SCRIPT_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)

# Only the checkout is mounted, read-only. All /etc, /run and /usr/local
# writes made by the real installer are confined to the disposable container.
docker run --rm --network none --cap-drop ALL \
    --security-opt no-new-privileges:true \
    --mount "type=bind,src=$SCRIPT_DIR,target=/source,readonly" \
    --env AWG_DOCUI_INSTALLER_SANDBOX=yes \
    golang:1.26-bookworm \
    sh /source/scripts/testdata/installer-sandbox.sh
