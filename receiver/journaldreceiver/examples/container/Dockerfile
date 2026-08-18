FROM debian:13-slim@sha256:3a39a0592364683e6bab97937b72cad5a8fa6dcbbee90edb3bb48c7f8e94f258

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
