# syntax=docker/dockerfile:1
# Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
# SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

# AWG DocUI is management-only: the image contains no VPN runtime or host tools.
FROM golang:1.26-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN install -d /out/rootfs/usr/local/bin \
    && if [ -n "$TARGETOS" ]; then export GOOS="$TARGETOS"; fi \
    && if [ -n "$TARGETARCH" ]; then export GOARCH="$TARGETARCH"; fi \
    && CGO_ENABLED=0 go build -trimpath \
       -ldflags="-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.buildDate=$BUILD_DATE" \
       -o /out/rootfs/usr/local/bin/awg-docui . \
    && install -d -o 65532 -g 65532 -m 0700 /out/rootfs/app/data \
    && install -d -m 0755 /out/rootfs/app/licenses/LICENSES /out/rootfs/etc/ssl/certs \
    && install -m 0644 LICENSE NOTICE THIRD_PARTY_NOTICES.txt /out/rootfs/app/licenses/ \
    && install -m 0644 LICENSES/Apache-2.0.txt LICENSES/MPL-2.0.txt /out/rootfs/app/licenses/LICENSES/ \
    && install -m 0644 web/static/fonts/Manrope-LICENSE.txt /out/rootfs/app/licenses/ \
    && install -m 0644 /etc/ssl/certs/ca-certificates.crt /out/rootfs/etc/ssl/certs/

FROM scratch
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
LABEL org.opencontainers.image.title="AWG DocUI" \
      org.opencontainers.image.description="Management-only Web UI for host-kernel AmneziaWG" \
      org.opencontainers.image.source="https://github.com/Bahonio/awg-docui" \
      org.opencontainers.image.licenses="AGPL-3.0-or-later" \
      org.opencontainers.image.version="$VERSION" \
      org.opencontainers.image.revision="$COMMIT" \
      org.opencontainers.image.created="$BUILD_DATE"
COPY --from=builder /out/rootfs/ /
WORKDIR /app
USER 65532:65532
ENV WEB_UI_PORT=54845 HOST_AGENT_SOCKET=/run/awg-docui/agent.sock
EXPOSE 54845
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/awg-docui", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/awg-docui"]
