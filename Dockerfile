FROM alpine:3.18

WORKDIR /otel

# Install required packages
RUN apk add --no-cache ca-certificates tzdata

# Copy binary and config
COPY ./bin/otelcontribcol_linux_amd64 /otel/otelcol
COPY ./config.yaml /otel/config.yaml

# Make it executable
RUN chmod +x /otel/otelcol

EXPOSE 4317 4318

ENTRYPOINT ["/otel/otelcol", "--config", "/otel/config.yaml"]
