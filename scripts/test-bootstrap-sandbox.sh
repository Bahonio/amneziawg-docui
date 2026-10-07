#!/bin/sh
set -eu
SCRIPT_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)

run_scenario() {
    distribution=$1
    scenario=$2
    test_arch=$3
    test_keyring=${4:-default}
    case "$distribution" in
        debian:13) sandbox_image=debian:trixie-slim ;;
        *) sandbox_image=debian:bookworm-slim ;;
    esac
    docker run --rm --platform linux/amd64 --network none --cap-drop ALL \
        --security-opt no-new-privileges:true \
        --mount "type=bind,src=$SCRIPT_DIR,target=/source,readonly" \
        --env AWG_DOCUI_BOOTSTRAP_SANDBOX=yes \
        --env AWG_DOCUI_BOOTSTRAP_SCENARIO="$scenario" \
        --env AWG_DOCUI_BOOTSTRAP_DISTRIBUTION="$distribution" \
        --env AWG_DOCUI_BOOTSTRAP_ARCH="$test_arch" \
        --env AWG_DOCUI_BOOTSTRAP_KEYRING="$test_keyring" \
        "$sandbox_image" \
        sh /source/scripts/testdata/bootstrap-sandbox.sh
}

for scenario in fresh adopt update bundle access-vpn access-proxy access-all access-invalid access-reconfigure unhealthy symlink log-symlink; do
    run_scenario ubuntu:24.04 "$scenario" amd64
done
run_scenario ubuntu:22.04 fresh amd64
for distribution in debian:12 debian:13; do
    for test_arch in amd64 arm64; do
        for scenario in fresh adopt update; do
            run_scenario "$distribution" "$scenario" "$test_arch"
        done
    done
done
run_scenario debian:13 bundle arm64
run_scenario debian:13 fresh amd64 legacy
run_scenario debian:11 unsupported amd64
run_scenario debian:13 unsupported-arch armv7
run_scenario debian:13 bad-key amd64
run_scenario debian:12 missing-headers amd64
