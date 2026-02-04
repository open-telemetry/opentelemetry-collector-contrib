FROM debian:trixie-slim

ARG OTEL_COL_VERSION=0.136.0

WORKDIR /opt

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        systemd \
        wget \
        ca-certificates \
    && wget "https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v${OTEL_COL_VERSION}/otelcol-contrib_${OTEL_COL_VERSION}_linux_amd64.tar.gz" \
    && tar -xzf "otelcol-contrib_${OTEL_COL_VERSION}_linux_amd64.tar.gz" \
    && rm -f "otelcol-contrib_${OTEL_COL_VERSION}_linux_amd64.tar.gz" \
    && apt-get purge -y --auto-remove wget \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

COPY otelcol-config.yml /opt/otelcol-config.yml

ENTRYPOINT ["/opt/otelcol-contrib"]
CMD ["--config", "/opt/otelcol-config.yml"]

EXPOSE 4317 4318 8888
