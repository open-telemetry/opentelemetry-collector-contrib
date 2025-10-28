# Custom telemetrygen image with shell capabilities for e2e testing
# This enables container ID detection from Kubernetes API for proper association testing

FROM alpine:3.18 as base

# Get the telemetrygen binary from the official image
FROM ghcr.io/open-telemetry/opentelemetry-collector-contrib/telemetrygen:latest as telemetrygen-source

# Build the final image with shell + telemetrygen
FROM base

# Install curl for Kubernetes API calls
RUN apk add --no-cache curl

# Copy telemetrygen binary from the official image
COPY --from=telemetrygen-source /telemetrygen /telemetrygen

# Set up a non-root user (matching the original telemetrygen image)
ARG USER_UID=10001
ARG USER_GID=10001
USER ${USER_UID}:${USER_GID}

ENTRYPOINT ["/telemetrygen"]
