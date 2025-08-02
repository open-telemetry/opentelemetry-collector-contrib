# syntax=docker/dockerfile:1.7
################################################################################
# 1) Build stage  ─ arm64 container, but cross-compiling to GOARCH=amd64
################################################################################
FROM --platform=linux/arm64 docker.io/library/golang:1.23.3-bullseye AS builder

ENV CGO_ENABLED=0 \
    # compile for AMD64 even though builder image is arm64
    GOOS=linux \
    GOARCH=amd64 \
    GO111MODULE=on

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN cd ./cmd/otelcontribcol && go build -trimpath -o /src/otelcol-contrib .

################################################################################
# 2) Runtime stage ─ real linux/amd64 image
################################################################################
FROM --platform=linux/amd64 gcr.io/distroless/static:latest
COPY --from=builder /src/otelcol-contrib /otelcol-contrib
EXPOSE 4317 4318 55680 8888
ENTRYPOINT ["/otelcol-contrib"]
CMD ["--config", "/etc/otel/config.yml"]
