# Custom telemetrygen image with shell capabilities for e2e testing
# This enables container ID detection from Kubernetes API for proper association testing

FROM alpine:latest as base

FROM ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen:latest as telemetrygen-source

FROM base

RUN apk add --no-cache curl

COPY --from=telemetrygen-source /telemetrygen /telemetrygen
ARG USER_UID=10001
ARG USER_GID=10001
USER ${USER_UID}:${USER_GID}

ENTRYPOINT ["/telemetrygen"]
