# Custom telemetrygen image with shell capabilities for e2e testing
# This enables container ID detection from Kubernetes API for proper association testing

FROM alpine:3.24.2@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6 as base

FROM ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen:latest@sha256:ec57561d1eb429e26e4bc4d9eecd8d8dbd49c01f8b80a01c27185d8f502ade0c as telemetrygen-source

FROM base

RUN apk add --no-cache curl

COPY --from=telemetrygen-source /telemetrygen /telemetrygen
ARG USER_UID=10001
ARG USER_GID=10001
USER ${USER_UID}:${USER_GID}

ENTRYPOINT ["/telemetrygen"]
