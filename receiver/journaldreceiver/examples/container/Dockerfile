FROM debian:13.7-slim@sha256:a29215f6a35e51e22adffa17f89e9d2ef06214e64a2bad10d765c46aea49f11f

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
