FROM golang:1.25-alpine AS builder

WORKDIR /build

RUN apk add --no-cache make git bash

COPY . .

# Create .tools directory for builder
RUN mkdir -p .tools

# Build tools first, ensuring static linking for Alpine
RUN cd internal/tools && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../../.tools/builder -trimpath go.opentelemetry.io/collector/cmd/builder

# Build the collector (this will use the statically linked builder)
RUN make otelcontribcol

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /otelcol

COPY --from=builder /build/bin/otelcontribcol_linux_amd64 /otelcol-contrib

EXPOSE 4317 4318 55679 13133 8888

ENTRYPOINT ["/otelcol-contrib"]
CMD ["--config=/etc/otelcol/config.yaml"]
