#!/bin/sh
set -eu
SCRIPT_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)

for scenario in fresh adopt update bundle symlink log-symlink; do
    docker run --rm --platform linux/amd64 --network none --cap-drop ALL \
        --security-opt no-new-privileges:true \
        --mount "type=bind,src=$SCRIPT_DIR,target=/source,readonly" \
        --env AWG_DOCUI_BOOTSTRAP_SANDBOX=yes \
        --env AWG_DOCUI_BOOTSTRAP_SCENARIO="$scenario" \
        debian:bookworm-slim \
        sh /source/scripts/testdata/bootstrap-sandbox.sh
done
