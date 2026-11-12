FROM debian:13.7-slim@sha256:a72b4433de1cf7d6bae0c4cee74d0547f58601a643b3375f070c51600d63b6b2

ARG OTEL_COL_VERSION=0.136.0

WORKDIR /opt

RUN apt update && \
    apt install -y systemd wget && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

RUN wget "https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v${OTEL_COL_VERSION}/otelcol-contrib_${OTEL_COL_VERSION}_linux_amd64.tar.gz" && \
    tar -xzf "otelcol-contrib_${OTEL_COL_VERSION}_linux_amd64.tar.gz"
COPY otelcol-config.yml /opt/otelcol-config.yml

ENTRYPOINT ["/opt/otelcol-contrib"]
CMD ["--config", "/opt/otelcol-config.yml"]
