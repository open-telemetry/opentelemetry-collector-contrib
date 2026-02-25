# Trace ID aware load-balancing exporter demo

Supported pipeline types: logs

## How to run

1. From the root of the project, run:
```shell
docker build -t otelcontribcol .
```

2. Then from this directory (exporter/loadbalancingexporter/example), run:
```shell
docker-compose up
```

## How does it work

- The `mingrammer/flog` container is used as a log generator.
- The `otel-agent` container is used as the otel-collector that receives log using `fluentforward`.
- The `otel-collector-{1,3}` containers are used to represent multiple otel-collector backends.
- Since none of the logs generated contain `traceID`, the load-balancing is random.