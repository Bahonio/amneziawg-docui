#!/bin/sh
set -eu

test -s LICENSE
test -s LICENSES/Apache-2.0.txt
test -s LICENSES/MPL-2.0.txt
test -s NOTICE
test -s THIRD_PARTY_NOTICES.txt
test -s web/static/fonts/Manrope-LICENSE.txt

grep -Fq 'GNU AFFERO GENERAL PUBLIC LICENSE' LICENSE
grep -Fq 'Apache License' LICENSES/Apache-2.0.txt
grep -Fq 'Mozilla Public License Version 2.0' LICENSES/MPL-2.0.txt
grep -Fq '005dfc1b120816c2d4a3f4f0292d2f7234d0413c' NOTICE
grep -Fq 'Copyright 2018 The Manrope Project Authors' THIRD_PARTY_NOTICES.txt
grep -Fq 'Container asset: Mozilla CA certificate bundle' THIRD_PARTY_NOTICES.txt
grep -Fq 'https://github.com/Bahonio/awg-docui' web/static/app.js

grep -Fq 'FROM scratch' Dockerfile
if grep -Eq '^RUN apk add|ENTRYPOINT .*tini|HEALTHCHECK .*wget' Dockerfile; then
    echo 'Final image must not contain Alpine userspace helpers' >&2
    exit 1
fi

module_raw=$(mktemp "${TMPDIR:-/tmp}/awg-docui-modules.XXXXXX")
module_list=$(mktemp "${TMPDIR:-/tmp}/awg-docui-modules-sorted.XXXXXX")
trap 'rm -f "$module_raw" "$module_list"' EXIT HUP INT TERM

# Keep go list out of a pipeline: POSIX sh reports only the last process'
# status, which previously let a failed dependency scan print PASS.
go list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}}{{end}}{{end}}' \
    . ./cmd/host-agent > "$module_raw"
sort -u "$module_raw" > "$module_list"
while IFS= read -r module; do
    [ -n "$module" ] || continue
    grep -Fq "Module: $module" THIRD_PARTY_NOTICES.txt || {
        echo "Missing third-party notice for $module" >&2
        exit 1
    }
done < "$module_list"

derived_count=$(grep -R -l \
    --exclude-dir=.git \
    --exclude=THIRD_PARTY_NOTICES.txt \
    --exclude=check-licenses.sh \
    'Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.' \
    . | wc -l | tr -d ' ')
[ "$derived_count" -eq 73 ] || {
    echo "Expected 73 inherited files with modification notices; found $derived_count" >&2
    exit 1
}

echo 'PASS: license grant, attribution, dependency notices and source link are complete'
