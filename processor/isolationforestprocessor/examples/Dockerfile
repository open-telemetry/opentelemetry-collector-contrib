# Multi-stage build for x86_64 Kubernetes deployment
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata make

WORKDIR /src

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build using the repository's build system
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN make otelcontribcol

# Or try alternative build commands if the above fails:
# RUN make otelcol-contrib
# RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o otelcontribcol ./cmd/otelcol-contrib

# Final stage - minimal runtime image
FROM alpine:3.18

# Add ca-certificates for TLS connections
RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 10001 otel && \
    adduser -D -u 10001 -G otel otel

# Copy the binary (adjust path based on where make puts it)
COPY --from=builder /src/bin/otelcontribcol_linux_amd64 /usr/local/bin/otelcontribcol

# Create config directory
RUN mkdir -p /etc/otelcol

# Switch to non-root user
USER otel

# Expose common OTEL ports
EXPOSE 4317 4318 8888 8889 13133

ENTRYPOINT ["/usr/local/bin/otelcontribcol"]
CMD ["--config=/etc/otelcol/config.yaml"]
