FROM debian:13.8-slim@sha256:8aa5a7bf5a1955bddbe7d74fcdd0b91ab30e74362a8b73af672b0481b35b44a0

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
