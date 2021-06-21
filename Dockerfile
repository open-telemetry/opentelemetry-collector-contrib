FROM ubuntu
COPY opentelemetry-collector-contrib /Users/adgollap/Documents/GitHub/opentelemetry-collector-contrib
RUN make otel-from-tree
RUN make otelcontribcol