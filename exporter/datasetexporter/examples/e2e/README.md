# E2E Example

This is simple example that demonstrates how to use open telemetry collector with `DataSetExporter`.

## How To Use It

1. Modify [otel-config.yaml](otel-config.yaml)
   1. Update `dataset_url` with the URL to your dataset server
   2. Update `api_key` with your API key
2. Run `docker-compose up --abort-on-container-exit`

## What It Does

It is using `telemetrygen` to generate logs and traces. Dataset exporter is defined there twice:
1. It's defined for accepting logs. It has 