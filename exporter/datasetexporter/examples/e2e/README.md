# E2E Example

This is a simple example that demonstrates how to use the OpenTelemetry collector with `DataSetExporter`.

## How To Use It

1. Modify [otel-config.yaml](otel-config.yaml)
   1. Update `dataset_url` with the URL to your dataset server
   2. Update `api_key` with your API key
2. Run `docker-compose up --build --abort-on-container-exit`

## What It Does

It starts three containers [docker-compose.yaml](docker-compose.yaml):
1. `otel-collector-dataset` - collector that accepts logs and traces
2. `telemetrygen-logs-dataset` - telemetry generator that generates logs for 10 minutes
3. `telemetrygen-traces-dataset` - telemetry generator that generates traces for 10 minutes

### otel-collector-dataset

This collector is based with components specified in [otelcol-builder.yaml](otelcol-builder.yaml).
It contains several processors and extensions that you may find useful.

When it's running following configuration is used - [otel-config.yaml](otel-config.yaml).

Dataset exporter is defined twice:
1. `dataset/log` to export logs
2. `dataset/traces` to export traces. This configuration is keeping buffers around longer to increase chance that full trace will so that aggregated numbers will be correct.